package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"bug_trapper/ascii"
	"bug_trapper/camera"
	"bug_trapper/hardware"
	"bug_trapper/identifier"
	"bug_trapper/printer"
	"bug_trapper/receipt"
	"bug_trapper/store"
)

func main() {
	dbPath := flag.String("db-path", "sightings.db", "Path to SQLite database")
	output := flag.String("output", "receipt.txt", "Output receipt file path")
	preview := flag.Bool("preview", false, "Show live camera preview window (requires: go build -tags gocv); implies --no-loop")
	printText := flag.String("print", "", "Print this string to the thermal printer and exit")
	noLoop := flag.Bool("no-loop", false, "Run a single iteration and exit (default: loop forever)")
	noHardware := flag.Bool("no-hardware", false, "Skip hardware (button/ring/matrix) initialization even on Pi build")
	hwTest := flag.String("hw-test", "", "Run a hardware self-test and exit: button|ring|matrix|all")
	flag.Parse()

	if *printText != "" {
		if err := printString(*printText); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *hwTest != "" {
		if err := runHWTest(*hwTest, *noHardware); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if os.Getenv("HUIT_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "Error: HUIT_API_KEY environment variable is not set")
		os.Exit(1)
	}

	oneShot := *noLoop || *preview
	if err := run(*dbPath, *output, *preview, oneShot, *noHardware); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		os.Exit(1)
	}
}

func printString(text string) error {
	p, err := printer.NewUSBPrinter()
	if err != nil {
		return err
	}
	defer p.Close()
	return p.Print(text)
}

func run(dbPath, output string, wantPreview, oneShot, noHW bool) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	step(1, "Opening camera...")
	cam := camera.NewDefaultCamera()
	if err := cam.Open(); err != nil {
		return fmt.Errorf("opening camera: %w", err)
	}
	defer cam.Close()
	done()

	hw, err := openHardware(noHW)
	if err != nil {
		return fmt.Errorf("opening hardware: %w", err)
	}
	defer hw.Close()

	db, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	ident := &identifier.OpenAIIdentifier{
		APIKey:  os.Getenv("HUIT_API_KEY"),
		BaseURL: "https://go.apis.huit.harvard.edu/ais-openai-direct/v1",
	}

	for {
		if err := waitTrigger(ctx, hw.Button); err != nil {
			return nil
		}
		if err := captureAndPrint(ctx, cam, hw, ident, db, output, wantPreview); err != nil {
			fmt.Fprintf(os.Stderr, "iteration error: %v\n", err)
			hw.Matrix.FlashX(2 * time.Second)
		}
		if oneShot {
			return nil
		}
	}
}

func openHardware(noHW bool) (*hardware.Devices, error) {
	if noHW {
		return hardware.NewNoOp(), nil
	}
	return hardware.New()
}

func captureAndPrint(
	ctx context.Context,
	cam camera.Camera,
	hw *hardware.Devices,
	ident *identifier.OpenAIIdentifier,
	db *store.SQLiteStore,
	output string,
	wantPreview bool,
) error {
	photo, err := capturePhoto(cam, hw, wantPreview)
	if err != nil {
		return fmt.Errorf("capturing photo: %w", err)
	}

	chaseCtx, chaseCancel := context.WithCancel(ctx)
	defer func() { chaseCancel(); hw.Matrix.Stop() }()
	hw.Matrix.StartChase(chaseCtx)

	step(3, "Identifying organism via OpenAI... (this may take a moment)")
	fmt.Println()
	result, err := ident.Identify(ctx, photo)
	if err != nil {
		return fmt.Errorf("identifying organism: %w", err)
	}
	fmt.Printf("    Found: %s\n", result.Name)

	step(4, "Downloading illustration...")
	fmt.Println()
	illustration, err := downloadImage(result.IllustrationURL)
	if err != nil {
		return fmt.Errorf("downloading illustration: %w", err)
	}
	step(4, "Converting to ASCII art...")
	conv := &ascii.Converter{}
	art, err := conv.Convert(illustration, receipt.ReceiptWidth)
	if err != nil {
		return fmt.Errorf("converting to ASCII: %w", err)
	}
	done()

	step(5, "Recording sighting in database...")
	count, err := db.RecordSighting(ctx, result.Name)
	if err != nil {
		return fmt.Errorf("recording sighting: %w", err)
	}
	done()
	if count > 1 {
		fmt.Printf("    Seen %d times before!\n", count)
	}

	step(6, "Formatting receipt...")
	receiptData := receipt.ReceiptData{
		OrganismName:  result.Name,
		Description:   result.Description,
		ASCIIArt:      art,
		SightingCount: count,
		Timestamp:     time.Now(),
	}
	formatted := receipt.Format(receiptData)
	done()

	step(7, "Saving receipt...")
	if err := os.WriteFile(output, []byte(formatted), 0644); err != nil {
		return fmt.Errorf("writing receipt: %w", err)
	}
	done()

	fmt.Println()
	fmt.Printf("Receipt saved to %s\n", output)
	fmt.Println()
	fmt.Println(formatted)

	step(8, "Sending to thermal printer...")
	header := receipt.FormatHeader()
	description := receipt.FormatDescription(result.Description)
	footer := receipt.FormatBottom(receiptData)
	if err := printReceipt(header, result.Name, description, illustration, footer); err != nil {
		if errors.Is(err, printer.ErrDeviceNotFound) {
			fmt.Println(" skipped (printer not connected).")
		} else {
			fmt.Printf(" failed: %v\n", err)
		}
	} else {
		done()
	}

	return nil
}

func capturePhoto(cam camera.Camera, hw *hardware.Devices, wantPreview bool) (image.Image, error) {
	if wantPreview {
		if !camera.SupportsPreview() {
			fmt.Println("    Preview not available in this build.")
			fmt.Println("    Rebuild with: go build -tags gocv -o bug_trapper .")
			fmt.Println("    (requires: brew install opencv)")
			fmt.Println()
			fmt.Println("    Falling back to direct capture...")
		} else if previewCam, ok := cam.(camera.PreviewCamera); ok {
			return capturePhotoWithPreview(previewCam, hw)
		}
	}

	step(2, "Capturing photo...")
	_ = hw.Ring.On(hardware.White)
	photo, err := cam.Capture()
	_ = hw.Ring.Off()
	if err != nil {
		return nil, err
	}
	done()
	return photo, nil
}

func capturePhotoWithPreview(previewCam camera.PreviewCamera, hw *hardware.Devices) (image.Image, error) {
	step(2, "Showing camera preview...")
	fmt.Println()
	fmt.Println("    Camera preview window is open.")
	fmt.Println("    Compose your shot, then press ENTER here to capture.")
	fmt.Println()

	if err := previewCam.ShowPreview(); err != nil {
		return nil, fmt.Errorf("opening preview: %w", err)
	}

	capturedCh := make(chan struct{})
	go func() {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("    Press ENTER to take photo > ")
		reader.ReadString('\n')
		close(capturedCh)
	}()

previewLoop:
	for {
		select {
		case <-capturedCh:
			break previewLoop
		default:
			if !previewCam.UpdatePreview() {
				break previewLoop
			}
		}
	}

	previewCam.StopPreview()

	step(2, "Capturing photo...")
	_ = hw.Ring.On(hardware.White)
	photo, err := previewCam.Capture()
	_ = hw.Ring.Off()
	if err != nil {
		return nil, err
	}
	done()
	return photo, nil
}

var (
	stdinLines = make(chan struct{}, 1)
	stdinOnce  sync.Once
)

func ensureStdinPump() {
	stdinOnce.Do(func() {
		go func() {
			r := bufio.NewReader(os.Stdin)
			for {
				if _, err := r.ReadString('\n'); err != nil {
					return
				}
				select {
				case stdinLines <- struct{}{}:
				default:
				}
			}
		}()
	})
}

func waitTrigger(ctx context.Context, btn hardware.Button) error {
	ensureStdinPump()
	fmt.Println()
	fmt.Println("    Press button or ENTER to capture (Ctrl+C to quit)...")

	btnCtx, btnCancel := context.WithCancel(ctx)
	defer btnCancel()
	btnTrig := make(chan struct{}, 1)
	go func() {
		if err := btn.Wait(btnCtx); err == nil {
			select {
			case btnTrig <- struct{}{}:
			default:
			}
		}
	}()

	select {
	case <-stdinLines:
		return nil
	case <-btnTrig:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func runHWTest(target string, noHW bool) error {
	hw, err := openHardware(noHW)
	if err != nil {
		return err
	}
	defer hw.Close()

	switch target {
	case "ring":
		fmt.Println("Ring: white for 1s")
		if err := hw.Ring.On(hardware.White); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
		return hw.Ring.Off()
	case "matrix":
		fmt.Println("Matrix: 3s edge chase, then 1s X flash")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		hw.Matrix.StartChase(ctx)
		<-ctx.Done()
		cancel()
		hw.Matrix.Stop()
		hw.Matrix.FlashX(1 * time.Second)
		return nil
	case "button":
		fmt.Println("Button: press within 10s")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := hw.Button.Wait(ctx); err != nil {
			return fmt.Errorf("button wait: %w", err)
		}
		fmt.Println("    detected")
		return nil
	case "all":
		for _, t := range []string{"ring", "matrix", "button"} {
			fmt.Printf("--- %s ---\n", t)
			if err := runHWTest(t, noHW); err != nil {
				fmt.Fprintf(os.Stderr, "    %s failed: %v\n", t, err)
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown --hw-test target %q (want button|ring|matrix|all)", target)
	}
}

func printReceipt(header, name, description string, img image.Image, footer string) error {
	p, err := printer.NewUSBPrinter()
	if err != nil {
		return err
	}
	defer p.Close()
	return p.PrintReceipt(header, name, description, img, footer)
}

func step(n int, msg string) {
	fmt.Printf("[%d/8] %s", n, msg)
}

func done() {
	fmt.Println(" done!")
}

func downloadImage(url string) (image.Image, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}
	return img, nil
}
