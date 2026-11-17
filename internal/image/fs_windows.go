//go:build windows

package image

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"golang.org/x/sys/windows"
)

const fsctlSetSparse = 0x000900C4

// markFileSparse marks a newly created NTFS file sparse so Truncate + forward Seek avoids synchronous zero-fill.
func markFileSparse(f *os.File) bool {
	var bytesReturned uint32
	err := windows.DeviceIoControl(
		windows.Handle(f.Fd()),
		fsctlSetSparse,
		nil,
		0,
		nil,
		0,
		&bytesReturned,
		nil,
	)
	runtime.KeepAlive(f)
	return err == nil
}

func commitTempFile(tmpPath, finalPath string, noClobber bool) error {
	_ = os.Chmod(tmpPath, 0644)
	if !noClobber {
		var lastErr error
		for i := 0; i < 5; i++ {
			if lastErr = os.Rename(tmpPath, finalPath); lastErr == nil {
				return nil
			}
			time.Sleep(20 * time.Millisecond)
		}
		return lastErr
	}
	from, err := windows.UTF16PtrFromString(tmpPath)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(finalPath)
	if err != nil {
		return err
	}
	// MoveFileEx without MOVEFILE_REPLACE_EXISTING fails atomically if destination exists.
	var lastErr error
	for i := 0; i < 5; i++ {
		lastErr = windows.MoveFileEx(from, to, 0)
		if lastErr == nil {
			return nil
		}
		if lastErr == windows.ERROR_FILE_EXISTS || lastErr == windows.ERROR_ALREADY_EXISTS {
			return fmt.Errorf("output file %q already exists (--no-clobber)", finalPath)
		}
		time.Sleep(20 * time.Millisecond)
	}
	return lastErr
}
