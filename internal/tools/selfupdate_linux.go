//go:build linux

package tools

import (
	"errors"
	"fmt"
	"os"
)

func replaceExecutable(targetPath, newPath string) error {
	if err := os.Chmod(newPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	if err := os.Rename(newPath, targetPath); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("permission denied replacing %s (re-run with sudo or install to ~/.local/bin via 'babe setup'): %w", targetPath, err)
		}
		return fmt.Errorf("failed to replace executable: %w", err)
	}

	return nil
}
