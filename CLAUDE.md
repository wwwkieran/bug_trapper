# Bug Trapper

A Go application that captures webcam photos of organisms, identifies them via OpenAI's vision API, generates ASCII art illustrations, and formats receipts for a 32-char thermal printer.

## Build & Run

```bash
# Prerequisites (choose one)
brew install imagesnap         # simple webcam capture (default)
brew install opencv            # for GoCV preview window support
brew install libusb            # for thermal printer USB support

# Build (default - uses imagesnap)
go build -o bug_trapper .

# Build with GoCV preview support
go build -tags gocv -o bug_trapper .

# Run
HUIT_API_KEY=your_key ./bug_trapper
./bug_trapper --preview        # with live camera preview window
./bug_trapper --db-path my.db  # custom database path
./bug_trapper --output out.txt # custom output file
./bug_trapper --print "hello"  # print string to thermal printer and exit
./bug_trapper --no-loop        # one-shot mode (default is kiosk loop)
./bug_trapper --no-hardware    # skip GPIO init even on Pi build
./bug_trapper --hw-test all    # hardware self-test: button|ring|matrix|all
```

By default the program runs as a kiosk loop: wait for button-or-ENTER, capture, identify, print, repeat. Press Ctrl+C to exit.

## Test

```bash
go test ./...                          # all unit tests
go test -v ./store/                    # store package only
go test -v ./receipt/                  # receipt package only
go test -v ./ascii/                    # ascii package only
go test -v ./identifier/              # identifier package only
go test -tags=integration ./camera/   # camera integration test (needs webcam)
```

## Project Structure

```
bug_trapper/
  main.go           # CLI entry point, wires components together
  store/             # SQLite sighting tracking
  receipt/           # 32-char receipt formatting
  ascii/             # Image-to-ASCII converter
  identifier/        # OpenAI GPT-4o vision + DALL-E integration
  camera/            # Webcam capture (imagesnap default, GoCV with -tags gocv)
  printer/           # 58mm thermal printer driver (USB via gousb, ESC/POS)
  hardware/          # Pi GPIO peripherals (button, WS2812 ring, MAX7219 matrix); -tags pi
```

## Architecture

- All external dependencies (camera, OpenAI API, database) are behind interfaces for testability
- Default camera uses `imagesnap` CLI (macOS); GoCV implementation available with `-tags gocv` build tag
- OpenAI integration: GPT-4o for vision identification, DALL-E 3 for illustration generation
- SQLite stores sightings with case-insensitive organism name matching
- Receipt is saved to disk and sent to the thermal printer (skipped if printer not connected)

## Environment Variables

- `HUIT_API_KEY` - Required. Harvard HUIT API key for OpenAI access via `go.apis.huit.harvard.edu`.

## Thermal Printer

Target: Micro Thermal Printer 58mm (TTL/RS232), 32 characters per line at default 12x24 font.

## Raspberry Pi (Pi Zero 2 W)

Build with `-tags pi` to enable GPIO peripherals: a button (capture trigger), a 12-LED WS2812B ring (scene light), and a MAX7219 8x8 matrix (working indicator).

### Wiring

| Device         | Function | Pi pin | BCM     | Notes |
|----------------|----------|--------|---------|-------|
| Push button    | Signal   | 36     | GPIO16  | active-low, internal pull-up |
| Push button    | GND      | 20     | —       | |
| WS2812B ring   | Data     | 32     | GPIO12  | PWM0 |
| WS2812B ring   | 5V       | 2      | —       | |
| WS2812B ring   | GND      | 6      | —       | |
| MAX7219 matrix | DIN      | 38     | GPIO20  | SPI1 MOSI |
| MAX7219 matrix | CLK      | 40     | GPIO21  | SPI1 SCLK |
| MAX7219 matrix | CS       | 12     | GPIO18  | SPI1 CE0 |
| MAX7219 matrix | 5V       | 4      | —       | |
| MAX7219 matrix | GND      | 14     | —       | |

### Pi setup (one-time)

```bash
sudo raspi-config nonint do_spi 0    # enable SPI

# Edit /boot/firmware/config.txt and add:
#   dtoverlay=spi1-1cs   # exposes /dev/spidev1.0
#   dtparam=audio=off    # frees PWM block for ws281x; without this the ring won't work
sudo nano /boot/firmware/config.txt
sudo reboot

sudo apt install -y build-essential scons libusb-1.0-0-dev rpicam-apps
sudo usermod -aG gpio,spi $USER && newgrp gpio

# Build and install the rpi_ws281x C library (provides ws2811.h for cgo).
# rpi-ws281x-go is just a wrapper; the C library must be installed first.
git clone https://github.com/jgarff/rpi_ws281x.git ~/rpi_ws281x
cd ~/rpi_ws281x
command -v scons >/dev/null || sudo apt install -y scons   # in case the apt install above missed it
scons
ls libws2811.a                                             # must exist before continuing
sudo cp libws2811.a /usr/local/lib/
sudo cp *.h /usr/local/include/
sudo ldconfig
cd -
```

Verify after reboot: `ls /dev/spidev*` shows both `/dev/spidev0.0` and `/dev/spidev1.0`; `lsmod | grep snd_bcm2835` returns nothing. Also verify: `ls /usr/local/include/ws2811.h /usr/local/lib/libws2811.a` returns both files.

### Build & run on the Pi

```bash
go build -tags pi -o bug_trapper .
sudo HUIT_API_KEY=... ./bug_trapper          # sudo required for ws281x DMA
sudo ./bug_trapper --hw-test all             # button + ring + matrix self-test
```

### Cross-compile from Mac (optional)

Requires zig as the C compiler (rpi-ws281x-go uses cgo).

Cross-compile also needs `ws2811.h` + `libws2811.a` reachable by zig. Easiest path: run the build directly on the Pi (above) — cross-compiling ws281x cgo from macOS requires staging the Pi's `/usr/local/include` and `/usr/local/lib` locally and pointing `CGO_CFLAGS` / `CGO_LDFLAGS` at them. Skip cross-compile unless you've set that up.

```bash
# 64-bit Pi OS (default for Zero 2 W):
CGO_ENABLED=1 CC="zig cc -target aarch64-linux-gnu" \
  GOOS=linux GOARCH=arm64 go build -tags pi -o bug_trapper .

# 32-bit Pi OS:
CGO_ENABLED=1 CC="zig cc -target arm-linux-gnueabihf" \
  GOOS=linux GOARCH=arm GOARM=7 go build -tags pi -o bug_trapper .
```

### Notes

- Ring brightness defaults to 64 (~25%) to stay within the Zero 2 W's 5V budget. For full brightness, power the strip from a separate 5V supply with shared ground.
- If a peripheral fails to initialize (not connected, wiring fault), startup logs `skipped (X not connected)` and the rest of the loop keeps working.
