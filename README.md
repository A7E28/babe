# babe

A fast, standalone, cross-platform (Windows & Linux) Android Systems, Firmware, and Diagnostics Toolkit with zero runtime prerequisites.

## Installation

### Quick Install (Automatic PATH Setup)
**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/A7E28/babe/main/install.ps1 | iex
```

**Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/A7E28/babe/main/install.sh | sh
```

### Pre-built Binaries
Download the latest executable for your operating system and architecture from the [Releases](https://github.com/A7E28/babe/releases) page, then run `./babe setup` to register it globally in your user `PATH`.

### Building from Source
Requires Go 1.22 or later.

```bash
git clone https://github.com/A7E28/babe.git
cd babe

# Build for current host OS & architecture
go run build.go

# Build all Windows & Linux targets (amd64 & arm64)
go run build.go all

# Run unit tests
go test -v ./...
```

## Usage

### Interactive Terminal Menu
Run `babe` without arguments to launch the interactive TUI:

```text
──────────────────────────────────────────────────
  babe v0.2.0
──────────────────────────────────────────────────
  [1]  System & USB Doctor (Diagnostics & udev)
  [2]  Platform-Tools & babe Updates (Status / Install / Self-Update)
  [3]  Device Telemetry & Reboot (ADB)
  [4]  Fastboot & Slot A/B Manager
  [5]  Logcat Stream & Crash Harvester
  [6]  APK & Split-Bundle Suite (Inspect / Install / Packages)
  [7]  Firmware Boot Image Inspector (boot.img / init_boot.img)
  [8]  Calculate File Checksum (MD5 / SHA1 / SHA256)
  [9]  Exit

Enter choice [1-9]:
```

### Domain-Scoped CLI Commands
Every interactive feature is available via structured, scriptable CLI subcommands (flags work before or after positional arguments):

```bash
# Global PATH setup, system diagnostics & udev
babe setup [-f|--force] [-s|--status]    # Install babe globally into user PATH
babe doctor
sudo babe doctor --fix-udev              # Install/reload Linux Android udev rules

# Manage Android SDK Platform-Tools & Self-Update
babe tools status
babe tools update                        # Or: babe update tools / babe update --tools
babe update self [-f|--force] [-c]       # Or: babe update --self

# Offline APK, Split-Bundle & Firmware Image Analysis (No device needed)
babe apk inspect app.apk [-a|--all]      # Inspect binary AndroidManifest.xml, SDKs, ABIs, certs
babe apk inspect bundle.apks [-a|--all]  # Inspect Split APK bundle
babe image inspect init_boot.img         # Inspect boot.img / init_boot.img (v0-v4) header
babe hash firmware.zip                   # Single-pass MD5, SHA1 & SHA256 checksums

# On-Device APK & Package Management
babe apk install app.apk [-s <serial>]   # Install .apk or .apks/.xapk/.apkm bundle
babe apk list -3 -f chrome [-s <serial>] # List installed packages
babe apk pull com.android.chrome -o ./   # Extract installed APK from device

# Device Telemetry & Reboot (ADB)
babe device list
babe device info [-s <serial>]
babe device reboot bootloader [-s <serial>]

# Fastboot & A/B Slot Operations
babe fastboot devices
babe fastboot info [-s <serial>]
babe fastboot slot b [-s <serial>]
babe fastboot flash init_boot magisk_patched.img [-s <serial>]

# Logcat Streaming & Crash Forensics
babe logcat -v W -p com.example.app -o session.log
babe logcat crash -o crash_report.txt
```
