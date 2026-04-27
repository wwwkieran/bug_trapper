# Bug Trapper

Capture photos of organisms with your webcam, identify them with AI, and print a receipt with ASCII art on a thermal printer.

Bug Trapper uses GPT-4o to identify organisms and DALL-E 3 to generate a cute illustration. The illustration is converted to ASCII art and formatted as a 32-character-wide receipt for a 58mm thermal printer. A SQLite database tracks every sighting -- if you find the same organism twice, the receipt tells you how many times you've seen it.

## Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- A `HUIT_API_KEY` environment variable with your Harvard HUIT API key

## macOS

### Install dependencies

```bash
# Webcam capture (pick one):
brew install imagesnap          # simple, no-frills capture
brew install opencv             # needed for live camera preview window

# SQLite (usually already present on macOS, but just in case):
brew install sqlite3
```

### Build

```bash
# Standard build (uses imagesnap for capture)
go build -o bug_trapper .

# Build with live camera preview window (requires opencv)
go build -tags gocv -o bug_trapper .
```

### Run

```bash
# Basic usage
export HUIT_API_KEY=your_key
./bug_trapper

# With live camera preview (only works with the gocv build)
./bug_trapper --preview

# Custom database and output paths
./bug_trapper --db-path my_sightings.db --output my_receipt.txt
```

When using `--preview`, a window opens showing the live camera feed so you can compose your shot. Press ENTER in the terminal to capture. Without `--preview`, the app captures immediately after you press ENTER.

## Raspberry Pi

### Install dependencies

```bash
sudo apt update
sudo apt install -y golang gcc libsqlite3-dev fswebcam
```

`fswebcam` is used for webcam capture on Linux. If you're on a newer Raspberry Pi OS with `libcamera`, you can also use `libcamera-still` (it's pre-installed).

### Build

```bash
go build -o bug_trapper .
```

> **Note:** The `--preview` flag is not supported on Raspberry Pi. The preview window requires GoCV/OpenCV with a display server, which is impractical on most Pi setups. The app captures directly when you press ENTER.

### Connect the thermal printer

The target printer is a **Micro Thermal Printer 58mm (TTL/RS232)** running at 9600 or 19200 baud. Connect it to the Pi's UART pins:

| Printer Pin | Pi Pin        |
|-------------|---------------|
| VCC         | 5V (Pin 2/4)  |
| GND         | GND (Pin 6)   |
| RX          | TX (GPIO 14)  |

Enable the serial port:

```bash
sudo raspi-config
# Interface Options -> Serial Port -> No login shell -> Yes enable hardware
sudo reboot
```

### Run

```bash
export HUIT_API_KEY=your_key
./bug_trapper
```

To send the receipt to the thermal printer:

```bash
# Generate the receipt
./bug_trapper --output receipt.txt

# Send to printer via serial
cat receipt.txt > /dev/serial0
```

### Run on boot (optional)

To start Bug Trapper automatically:

```bash
# Create a systemd service
sudo tee /etc/systemd/system/bug-trapper.service > /dev/null <<EOF
[Unit]
Description=Bug Trapper
After=network.target

[Service]
ExecStart=/home/pi/bug_trapper
WorkingDirectory=/home/pi
Environment=HUIT_API_KEY=your_key
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable bug-trapper
sudo systemctl start bug-trapper
```

## CLI Flags

| Flag         | Default        | Description                                      |
|--------------|----------------|--------------------------------------------------|
| `--preview`  | `false`        | Show live camera preview (macOS + gocv build only)|
| `--db-path`  | `sightings.db` | Path to SQLite database file                     |
| `--output`   | `receipt.txt`  | Output receipt file path                         |

## How It Works

1. Open the camera
2. Wait for you to press ENTER (or compose in preview window)
3. Capture a photo
4. Send the photo to GPT-4o to identify the organism and get a kid-friendly description
5. Generate a cute illustration with DALL-E 3
6. Convert the illustration to ASCII art (32 chars wide)
7. Record the sighting in SQLite
8. Format and save the receipt
9. Auto-open the receipt file

### Sample Receipt

```
================================
          BUG TRAPPER
================================

Name: Monarch Butterfly
--------------------------------
A beautiful orange and black
butterfly that travels thousands
of miles every year to find
warm weather!
--------------------------------
     .:-=+*##*+=-:.
   :=*%@@@@@@@@%*=:
  -#@@@@@@@@@@@@@@#-
 .#@@@@%*=-::-=*%@@@#.
 +@@@@=.        .=@@@@+
 *@@@#   .:--:.   #@@@*
 *@@@%  -#@@@@#-  %@@@*
 +@@@@=. .=**=. .=@@@@+
 .#@@@@%*=:...:=*%@@@@#.
  -#@@@@@@@@@@@@@@@@#-
   :=*%@@@@@@@@%*=:
     .:-=+**+=-:.
--------------------------------
This is the 3rd time you've
found this organism!

      2026-04-09 14:30:00
================================
```

## Testing

```bash
go test ./...
```
