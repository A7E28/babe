package tools

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/A7E28/babe/internal/hash"
	"github.com/A7E28/babe/internal/platform"
)

// describes the current global installation state of babe.
type BabeInstallStatus struct {
	CurrentExecutable string
	CanonicalBinDir   string
	CanonicalBinPath  string
	RunningFromTarget bool
	BinaryInstalled   bool
	UpToDate          bool
	InPath            bool
}

// summarizes the actions taken during SetupBabe.
type SetupResult struct {
	TargetBinDir     string
	TargetBinPath    string
	AlreadyInstalled bool
	BinaryUpdated    bool
	PathConfigured   bool
	PathAlreadySet   bool
}

// inspects whether babe is installed in the canonical user bin directory and registered in PATH.
func GetBabeInstallStatus(pm platform.PlatformManager) (*BabeInstallStatus, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve current executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}

	binDir, err := pm.GetBabeBinDir()
	if err != nil {
		return nil, fmt.Errorf("resolve canonical bin directory: %w", err)
	}

	binName := "babe"
	if runtime.GOOS == "windows" {
		binName = "babe.exe"
	}
	targetPath := filepath.Join(binDir, binName)

	inPath, _ := pm.IsInPath(binDir)
	status := &BabeInstallStatus{
		CurrentExecutable: exePath,
		CanonicalBinDir:   binDir,
		CanonicalBinPath:  targetPath,
		InPath:            inPath,
	}

	srcInfo, srcErr := os.Stat(exePath)
	dstInfo, dstErr := os.Stat(targetPath)
	if dstErr == nil && !dstInfo.IsDir() {
		status.BinaryInstalled = true
		if srcErr == nil && os.SameFile(srcInfo, dstInfo) {
			status.RunningFromTarget = true
			status.UpToDate = true
		} else {
			srcHash, err1 := hash.CalculateFileHash(exePath, hash.SHA256)
			dstHash, err2 := hash.CalculateFileHash(targetPath, hash.SHA256)
			if err1 == nil && err2 == nil && srcHash != "" && strings.EqualFold(srcHash, dstHash) {
				status.UpToDate = true
			}
		}
	}

	return status, nil
}

// installs the running babe binary into the canonical user bin directory and ensures PATH registration.
func SetupBabe(pm platform.PlatformManager, force bool) (*SetupResult, error) {
	status, err := GetBabeInstallStatus(pm)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(status.CanonicalBinDir, 0755); err != nil {
		return nil, fmt.Errorf("create bin directory %s: %w", status.CanonicalBinDir, err)
	}

	res := &SetupResult{
		TargetBinDir:  status.CanonicalBinDir,
		TargetBinPath: status.CanonicalBinPath,
	}

	if status.RunningFromTarget || (status.BinaryInstalled && status.UpToDate && !force) {
		res.AlreadyInstalled = true
	} else {
		if err := copyAndReplaceBinary(status.CurrentExecutable, status.CanonicalBinDir, status.CanonicalBinPath); err != nil {
			return nil, err
		}
		res.BinaryUpdated = true
	}

	if status.InPath {
		res.PathAlreadySet = true
		res.PathConfigured = true
	} else {
		if err := pm.AddToPath(status.CanonicalBinDir); err != nil && !errors.Is(err, platform.ErrPathAlreadyExists) {
			return res, fmt.Errorf("register %s in PATH: %w", status.CanonicalBinDir, err)
		}
		res.PathConfigured = true
	}

	return res, nil
}

func copyAndReplaceBinary(srcPath, targetDir, targetPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source binary %s: %w", srcPath, err)
	}
	defer srcFile.Close()

	tempFile, err := os.CreateTemp(targetDir, ".babe-setup-*.tmp")
	if err != nil {
		return fmt.Errorf("create staging binary in %s: %w", targetDir, err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(tempFile, srcFile, buf); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("copy binary to staging file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close staging binary: %w", err)
	}

	return replaceExecutable(targetPath, tempPath)
}
