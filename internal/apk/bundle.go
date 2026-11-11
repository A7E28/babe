package apk

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/A7E28/babe/internal/device"
)

// InstallAuto installs either a single .apk or a Split APK bundle (.apks, .xapk, .apkm) onto a device.
func InstallAuto(ctx context.Context, client *device.Client, serial, archivePath string, opts InstallOptions) error {
	lower := strings.ToLower(archivePath)
	if strings.HasSuffix(lower, ".apks") || strings.HasSuffix(lower, ".xapk") || strings.HasSuffix(lower, ".apkm") {
		return InstallBundle(ctx, client, serial, archivePath, opts)
	}

	// Also inspect if a generic file is a bundle containing split APKs
	if info, err := InspectArchive(archivePath); err == nil && info.IsBundle {
		return InstallBundle(ctx, client, serial, archivePath, opts)
	}

	return Install(ctx, client, serial, archivePath, opts)
}

// InstallBundle extracts split APKs from a bundle archive and installs them via `adb install-multiple`.
func InstallBundle(ctx context.Context, client *device.Client, serial, bundlePath string, opts InstallOptions) error {
	zr, err := zip.OpenReader(bundlePath)
	if err != nil {
		return fmt.Errorf("open split APK bundle: %w", err)
	}
	defer zr.Close()

	tempDir, err := os.MkdirTemp("", "babe-split-apks-*")
	if err != nil {
		return fmt.Errorf("create temp dir for split APKs: %w", err)
	}
	defer os.RemoveAll(tempDir)

	var extractedAPKs []string
	for i, f := range zr.File {
		if f.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(f.Name), ".apk") {
			continue
		}

		safeName := fmt.Sprintf("split_%d_%s", i, filepath.Base(f.Name))
		destPath := filepath.Join(tempDir, safeName)

		if err := extractZipEntry(f, destPath); err != nil {
			return fmt.Errorf("extract split %s: %w", f.Name, err)
		}
		extractedAPKs = append(extractedAPKs, destPath)
	}

	if len(extractedAPKs) == 0 {
		return fmt.Errorf("no .apk files found inside bundle %s", bundlePath)
	}

	args := []string{"install-multiple"}
	if opts.Reinstall {
		args = append(args, "-r")
	}
	if opts.Downgrade {
		args = append(args, "-d")
	}
	if opts.GrantPerm {
		args = append(args, "-g")
	}
	args = append(args, extractedAPKs...)

	out, err := client.ExecuteOnDevice(ctx, serial, args...)
	if err != nil {
		return fmt.Errorf("install-multiple failed: %w", err)
	}

	if !strings.Contains(out, "Success") {
		return fmt.Errorf("bundle installation failed: %s", out)
	}

	return nil
}

// PackSplitBundle packages multiple split APK files into a single .apks zip archive using Store compression.
func PackSplitBundle(splitPaths []string, bundlePath string) error {
	outFile, err := os.OpenFile(bundlePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer outFile.Close()

	zw := zip.NewWriter(outFile)
	buf := make([]byte, 64*1024)

	for _, splitPath := range splitPaths {
		if err := addFileToZip(zw, splitPath, buf); err != nil {
			_ = zw.Close()
			return err
		}
	}

	return zw.Close()
}

func addFileToZip(zw *zip.Writer, filePath string, buf []byte) error {
	src, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer src.Close()

	fi, err := src.Stat()
	if err != nil {
		return err
	}

	hdr, err := zip.FileInfoHeader(fi)
	if err != nil {
		return err
	}
	hdr.Name = filepath.Base(filePath)
	hdr.Method = zip.Store

	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}

	_, err = io.CopyBuffer(w, src, buf)
	return err
}

func extractZipEntry(f *zip.File, destPath string) error {
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer dst.Close()

	buf := make([]byte, 64*1024)
	_, err = io.CopyBuffer(dst, src, buf)
	return err
}
