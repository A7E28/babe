//go:build windows

package platform

import "errors"

// indicates that Linux udev rule installation is not applicable on Windows.
var ErrUdevUnsupported = errors.New("udev rules are only applicable on Linux systems")

// returns ErrUdevUnsupported on Windows targets.
func InstallUdevRules() error {
	return ErrUdevUnsupported
}
