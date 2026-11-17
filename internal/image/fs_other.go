//go:build !windows

package image

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func markFileSparse(_ *os.File) bool { return true }

func commitTempFile(tmpPath, finalPath string, noClobber bool) error {
	_ = os.Chmod(tmpPath, 0644)
	if !noClobber {
		return os.Rename(tmpPath, finalPath)
	}
	// os.Link atomically fails with EEXIST if finalPath already exists.
	if err := os.Link(tmpPath, finalPath); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("output file %q already exists (--no-clobber)", finalPath)
		}
		if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.ENOSYS) {
			if _, stErr := os.Lstat(finalPath); stErr == nil {
				return fmt.Errorf("output file %q already exists (--no-clobber)", finalPath)
			}
			return os.Rename(tmpPath, finalPath)
		}
		return err
	}
	_ = os.Remove(tmpPath)
	return nil
}
