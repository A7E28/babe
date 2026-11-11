//go:build windows

package tools

import (
	"fmt"
	"os"
)

func replaceExecutable(targetPath, newPath string) error {
	oldPath := targetPath + ".old"
	_ = os.Remove(oldPath)

	if err := os.Rename(targetPath, oldPath); err != nil {
		return fmt.Errorf("failed to backup current executable (check file permissions): %w", err)
	}

	if err := os.Rename(newPath, targetPath); err != nil {
		_ = os.Rename(oldPath, targetPath)
		return fmt.Errorf("failed to replace executable: %w", err)
	}

	return nil
}
