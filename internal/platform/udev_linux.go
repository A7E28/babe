//go:build linux

package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// indicates that Linux udev rule installation is not applicable on the current OS.
var ErrUdevUnsupported = errors.New("udev rules are only applicable on Linux systems")

const androidUdevRules = `# Android USB device rules for non-root ADB access
# Managed by babe
SUBSYSTEM=="usb", ATTR{idVendor}=="18d1", MODE="0666", GROUP="plugdev" # Google
SUBSYSTEM=="usb", ATTR{idVendor}=="04e8", MODE="0666", GROUP="plugdev" # Samsung
SUBSYSTEM=="usb", ATTR{idVendor}=="2717", MODE="0666", GROUP="plugdev" # Xiaomi
SUBSYSTEM=="usb", ATTR{idVendor}=="05c6", MODE="0666", GROUP="plugdev" # Qualcomm
SUBSYSTEM=="usb", ATTR{idVendor}=="22b8", MODE="0666", GROUP="plugdev" # Motorola
SUBSYSTEM=="usb", ATTR{idVendor}=="0bb4", MODE="0666", GROUP="plugdev" # HTC
SUBSYSTEM=="usb", ATTR{idVendor}=="12d1", MODE="0666", GROUP="plugdev" # Huawei
SUBSYSTEM=="usb", ATTR{idVendor}=="1004", MODE="0666", GROUP="plugdev" # LG
SUBSYSTEM=="usb", ATTR{idVendor}=="0fce", MODE="0666", GROUP="plugdev" # Sony
SUBSYSTEM=="usb", ATTR{idVendor}=="2a70", MODE="0666", GROUP="plugdev" # OnePlus
SUBSYSTEM=="usb", ATTR{idVendor}=="22d9", MODE="0666", GROUP="plugdev" # OPPO
SUBSYSTEM=="usb", ATTR{idVendor}=="2d95", MODE="0666", GROUP="plugdev" # Realme
SUBSYSTEM=="usb", ATTR{idVendor}=="2b4c", MODE="0666", GROUP="plugdev" # Vivo
SUBSYSTEM=="usb", ATTR{idVendor}=="2e04", MODE="0666", GROUP="plugdev" # Nothing
SUBSYSTEM=="usb", ATTR{idVendor}=="0b05", MODE="0666", GROUP="plugdev" # ASUS
`

// writes Android device udev rules to /etc/udev/rules.d/51-android.rules
// and triggers udev reload. Requires root / sudo.
func InstallUdevRules() error {
	rulesPath := "/etc/udev/rules.d/51-android.rules"

	if err := os.WriteFile(rulesPath, []byte(androidUdevRules), 0644); err != nil {
		return fmt.Errorf("write %s (requires sudo privileges): %w", rulesPath, err)
	}

	if err := exec.Command("udevadm", "control", "--reload-rules").Run(); err != nil {
		return fmt.Errorf("reload udev rules: %w", err)
	}

	if err := exec.Command("udevadm", "trigger").Run(); err != nil {
		return fmt.Errorf("trigger udev rules: %w", err)
	}

	return nil
}
