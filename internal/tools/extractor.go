package tools

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var ErrZipSlip = errors.New("zip file contains illegal relative path (ZipSlip vulnerability)")

// unpacks a zip archive into destDir with path sanitization and permission preservation.
func ExtractZip(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip archive: %w", err)
	}
	defer reader.Close()

	destClean := filepath.Clean(destDir)

	for _, f := range reader.File {
		targetPath := filepath.Join(destClean, f.Name)
		targetClean := filepath.Clean(targetPath)

		// Guard against ZipSlip
		if !strings.HasPrefix(targetClean, destClean+string(filepath.Separator)) && targetClean != destClean {
			return fmt.Errorf("%w: %s", ErrZipSlip, f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetClean, 0755); err != nil {
				return fmt.Errorf("create directory %s: %w", targetClean, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetClean), 0755); err != nil {
			return fmt.Errorf("create parent directory for %s: %w", targetClean, err)
		}

		if err := extractFile(f, targetClean); err != nil {
			return fmt.Errorf("extract file %s: %w", f.Name, err)
		}
	}

	return nil
}

func extractFile(f *zip.File, targetPath string) error {
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	mode := f.Mode()
	baseName := filepath.Base(targetPath)
	if baseName == "adb" || baseName == "fastboot" || baseName == "mke2fs" || baseName == "sqlite3" {
		mode = 0755
	}

	dst, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dst.Close()

	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(dst, src, buf); err != nil {
		return err
	}

	return nil
}
