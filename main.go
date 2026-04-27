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
	"time"

	"bug_trapper/ascii"
	"bug_trapper/camera"
	"bug_trapper/identifier"
	"bug_trapper/printer"
	"bug_trapper/receipt"
	"bug_trapper/store"
)

func main() {
	dbPath := flag.String("db-path", "sightings.db", "Path to SQLite database")
	output := flag.String("output", "receipt.txt", "Output receipt file path")
	preview := flag.Bool("preview", false, "Show live camera preview window (requires: go build -tags gocv)")
	printText := flag.String("print", "", "Print this string to the thermal printer and exit")
	flag.Parse()

	if *printText != "" {
		if err := printString(*printText); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if os.Getenv("HUIT_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "Error: HUIT_API_KEY environment variable is not set")
		os.Exit(1)
	}

	if err := run(*dbPath, *output, *preview); err != nil {
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

func run(dbPath, output string, wantPreview bool) error {
	ctx := context.Background()

	// --- Step 1: Open camera ---
	step(1, 7, "Opening camera...")
	cam := camera.NewDefaultCamera()
	if err := cam.Open(); err != nil {
		return fmt.Errorf("opening camera: %w", err)
	}
	defer cam.Close()
	done()

	// --- Step 2: Preview (if supported) and capture ---
	var photo image.Image

	if wantPreview {
		if !camera.SupportsPreview() {
			fmt.Println("    Preview not available in this build.")
			fmt.Println("    Rebuild with: go build -tags gocv -o bug_trapper .")
			fmt.Println("    (requires: brew install opencv)")
			fmt.Println()
			fmt.Println("    Falling back to direct capture...")
		} else if previewCam, ok := cam.(camera.PreviewCamera); ok {
			step(2, 7, "Showing camera preview...")
			fmt.Println()
			fmt.Println("    Camera preview window is open.")
			fmt.Println("    Compose your shot, then press ENTER here to capture.")
			fmt.Println()

			if err := previewCam.ShowPreview(); err != nil {
				return fmt.Errorf("opening preview: %w", err)
			}

			// Run preview loop in background, capture on Enter
			capturedCh := make(chan struct{})
			go func() {
				reader := bufio.NewReader(os.Stdin)
				fmt.Print("    Press ENTER to take photo > ")
				reader.ReadString('\n')
				close(capturedCh)
			}()

			// Keep updating preview until user presses Enter
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

			step(2, 7, "Capturing photo...")
			var err error
			photo, err = cam.Capture()
			if err != nil {
				return fmt.Errorf("capturing photo: %w", err)
			}
			done()
		}
	}

	// If no preview or preview not used, do simple capture
	if photo == nil {
		step(2, 7, "Ready to capture!")
		fmt.Println()
		fmt.Print("    Press ENTER to take a photo > ")
		reader := bufio.NewReader(os.Stdin)
		reader.ReadString('\n')

		step(2, 7, "Capturing photo...")
		var err error
		photo, err = cam.Capture()
		if err != nil {
			return fmt.Errorf("capturing photo: %w", err)
		}
		done()
	}

	// --- Step 3: Identify organism ---
	step(3, 7, "Identifying organism via OpenAI... (this may take a moment)")
	fmt.Println()
	ident := &identifier.OpenAIIdentifier{
		APIKey:  os.Getenv("HUIT_API_KEY"),
		BaseURL: "https://go.apis.huit.harvard.edu/ais-openai-direct/v1",
	}
	result, err := ident.Identify(ctx, photo)
	if err != nil {
		return fmt.Errorf("identifying organism: %w", err)
	}
	fmt.Printf("    Found: %s\n", result.Name)

	// --- Step 4: Download and convert illustration ---
	step(4, 7, "Downloading illustration...")
	fmt.Println()
	illustration, err := downloadImage(result.IllustrationURL)
	if err != nil {
		return fmt.Errorf("downloading illustration: %w", err)
	}
	step(4, 7, "Converting to ASCII art...")
	conv := &ascii.Converter{}
	art, err := conv.Convert(illustration, receipt.ReceiptWidth)
	if err != nil {
		return fmt.Errorf("converting to ASCII: %w", err)
	}
	done()

	// --- Step 5: Record sighting ---
	step(5, 7, "Recording sighting in database...")
	db, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()
	count, err := db.RecordSighting(ctx, result.Name)
	if err != nil {
		return fmt.Errorf("recording sighting: %w", err)
	}
	done()
	if count > 1 {
		fmt.Printf("    Seen %d times before!\n", count)
	}

	// --- Step 6: Format receipt ---
	step(6, 8, "Formatting receipt...")
	receiptData := receipt.ReceiptData{
		OrganismName:  result.Name,
		Description:   result.Description,
		ASCIIArt:      art,
		SightingCount: count,
		Timestamp:     time.Now(),
	}
	formatted := receipt.Format(receiptData)
	done()

	// --- Step 7: Save receipt ---
	step(7, 8, "Saving receipt...")
	if err := os.WriteFile(output, []byte(formatted), 0644); err != nil {
		return fmt.Errorf("writing receipt: %w", err)
	}
	done()

	fmt.Println()
	fmt.Printf("Receipt saved to %s\n", output)
	fmt.Println()
	fmt.Println(formatted)

	// --- Step 8: Print to thermal printer ---
	step(8, 8, "Sending to thermal printer...")
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

func printReceipt(header, name, description string, img image.Image, footer string) error {
	p, err := printer.NewUSBPrinter()
	if err != nil {
		return err
	}
	defer p.Close()
	return p.PrintReceipt(header, name, description, img, footer)
}

func step(n, total int, msg string) {
	fmt.Printf("[%d/%d] %s", n, total, msg)
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

