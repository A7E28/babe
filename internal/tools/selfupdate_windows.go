//go:build windows

package tools

import (
	"fmt"
	"os"
)

func replaceExecutable(targetPath, newPath string) error {
	oldPath := targetPath + ".old"
	_ = os.Remove(oldPath)

	hasOld := false
	if _, err := os.Stat(targetPath); err == nil {
		if err := os.Rename(targetPath, oldPath); err != nil {
			return fmt.Errorf("failed to backup current executable (check file permissions): %w", err)
		}
		hasOld = true
	}

	if err := os.Rename(newPath, targetPath); err != nil {
		if hasOld {
			_ = os.Rename(oldPath, targetPath)
		}
		return fmt.Errorf("failed to replace executable: %w", err)
	}

	if hasOld {
		_ = os.Remove(oldPath)
	}
	return nil
}
