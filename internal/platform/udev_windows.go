//go:build windows

package platform

import "errors"

var ErrUdevUnsupported = errors.New("udev rules are only applicable on Linux systems")

func InstallUdevRules() error {
	return ErrUdevUnsupported
}
