//go:build windows

package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func replaceExecutable(targetPath, newPath string) error {
	oldPath, err := uniqueBackupPath(targetPath)
	if err != nil {
		return err
	}

	hasOld := false
	if _, err := os.Stat(targetPath); err == nil {
		if err := renameWithRetry(targetPath, oldPath); err != nil {
			return fmt.Errorf("failed to backup current executable (check file permissions or close running babe instances): %w", err)
		}
		hasOld = true
	}

	if err := renameWithRetry(newPath, targetPath); err != nil {
		if hasOld {
			_ = renameWithRetry(oldPath, targetPath)
		}
		return fmt.Errorf("failed to replace executable: %w", err)
	}

	if hasOld {
		// Best-effort cleanup; if another babe process still maps the old image,
		// cleanupOldBinary removes stale *.old files on the next invocation.
		_ = os.Remove(oldPath)
	}
	return nil
}

func uniqueBackupPath(targetPath string) (string, error) {
	pid := os.Getpid()
	for i := 0; i < 16; i++ {
		candidate := fmt.Sprintf("%s.%d.old", targetPath, pid)
		if i > 0 {
			candidate = fmt.Sprintf("%s.%d.%d.old", targetPath, pid, i)
		}
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(targetPath), filepath.Base(targetPath)+".*.old")
	if err != nil {
		return "", fmt.Errorf("allocate backup executable path: %w", err)
	}
	name := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(name)
	return name, nil
}

func renameWithRetry(src, dst string) error {
	const maxAttempts = 6
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := os.Rename(src, dst); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return lastErr
}
