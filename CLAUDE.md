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
```

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
