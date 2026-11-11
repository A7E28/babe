//go:build linux

package tools

import (
	"fmt"
	"os"
)

func replaceExecutable(targetPath, newPath string) error {
	if err := os.Chmod(newPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	if err := os.Rename(newPath, targetPath); err != nil {
		return fmt.Errorf("failed to replace executable (check sudo privileges): %w", err)
	}

	return nil
}
