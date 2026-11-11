package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/A7E28/babe/internal/hash"
	"github.com/A7E28/babe/internal/platform"
)

const (
	WindowsPlatformToolsURL = "https://dl.google.com/android/repository/platform-tools-latest-windows.zip"
	LinuxPlatformToolsURL   = "https://dl.google.com/android/repository/platform-tools-latest-linux.zip"
	DarwinPlatformToolsURL  = "https://dl.google.com/android/repository/platform-tools-latest-darwin.zip"
)

// returns the official Google SDK platform-tools URL for the current operating system.
func GetPlatformToolsURL() (string, error) {
	switch runtime.GOOS {
	case "windows":
		return WindowsPlatformToolsURL, nil
	case "linux":
		return LinuxPlatformToolsURL, nil
	case "darwin":
		return DarwinPlatformToolsURL, nil
	default:
		return "", fmt.Errorf("unsupported operating system for platform-tools: %s", runtime.GOOS)
	}
}

// downloads Google Platform-Tools, verifies its integrity, and adds it to PATH.
func DownloadAndConfigurePlatformTools(ctx context.Context, customURL, expectedChecksum string, onProgress ProgressFunc) (string, error) {
	downloadURL := customURL
	if downloadURL == "" {
		var err error
		downloadURL, err = GetPlatformToolsURL()
		if err != nil {
			return "", err
		}
	}

	pm := platform.DefaultManager()
	targetDir, err := pm.GetDefaultInstallDir()
	if err != nil {
		return "", fmt.Errorf("resolve platform-tools directory: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "babe-platform-tools-*")
	if err != nil {
		return "", fmt.Errorf("create temp download directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	zipFile := filepath.Join(tempDir, "platform-tools.zip")

	if err := DownloadFile(ctx, downloadURL, zipFile, onProgress); err != nil {
		return "", fmt.Errorf("download platform-tools: %w", err)
	}

	// Plausibility check (> 5MB)
	fi, err := os.Stat(zipFile)
	if err != nil || fi.Size() < 5*1024*1024 {
		return "", fmt.Errorf("downloaded zip archive appears truncated or invalid")
	}

	if expectedChecksum != "" {
		if err := verifyArchiveChecksum(zipFile, expectedChecksum); err != nil {
			return "", err
		}
	}

	parentDir := filepath.Dir(targetDir)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return "", fmt.Errorf("create parent directory %s: %w", parentDir, err)
	}

	// stop any running ADB server before replacing binaries (prevents Windows file locks)
	stopRunningADB(targetDir)

	// clean existing target if present before extracting new version
	if err := os.RemoveAll(targetDir); err != nil {
		return "", fmt.Errorf("remove existing installation at %s: %w", targetDir, err)
	}

	if err := ExtractZip(zipFile, parentDir); err != nil {
		return "", fmt.Errorf("extract platform-tools: %w", err)
	}

	// update current process PATH so interactive sessions immediately resolve adb/fastboot
	_ = os.Setenv("PATH", targetDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := pm.AddToPath(targetDir); err != nil && err != platform.ErrPathAlreadyExists {
		return targetDir, fmt.Errorf("saved platform-tools to %s, but failed to update PATH: %w", targetDir, err)
	}

	return targetDir, nil
}

func verifyArchiveChecksum(zipFile, expected string) error {
	expected = strings.TrimSpace(expected)
	var algo hash.Algorithm
	switch len(expected) {
	case 40:
		algo = hash.SHA1
	case 64:
		algo = hash.SHA256
	default:
		return nil
	}

	actual, err := hash.CalculateFileHash(zipFile, algo)
	if err != nil {
		return fmt.Errorf("calculate archive checksum: %w", err)
	}
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("archive checksum mismatch: got %s, expected %s", actual, expected)
	}
	return nil
}

func stopRunningADB(targetDir string) {
	killCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	binName := "adb"
	if runtime.GOOS == "windows" {
		binName = "adb.exe"
	}
	localADB := filepath.Join(targetDir, binName)
	if _, err := os.Stat(localADB); err == nil {
		_ = exec.CommandContext(killCtx, localADB, "kill-server").Run()
		return
	}
	_ = exec.CommandContext(killCtx, "adb", "kill-server").Run()
}

func InstallPlatformTools(ctx context.Context, onProgress ProgressFunc) (string, error) {
	return DownloadAndConfigurePlatformTools(ctx, "", "", onProgress)
}
