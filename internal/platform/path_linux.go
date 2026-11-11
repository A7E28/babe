//go:build linux

package platform

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type linuxPlatformManager struct{}

func defaultManager() PlatformManager {
	return &linuxPlatformManager{}
}

func (l *linuxPlatformManager) GetDefaultInstallDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "babe", "platform-tools"), nil
}

func (l *linuxPlatformManager) IsInPath(dir string) (bool, error) {
	currentPath := os.Getenv("PATH")
	segments := CleanPathString(currentPath, ':')
	if ContainsPath(segments, dir, false) {
		return true, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return false, nil
	}
	localBin := filepath.Join(home, ".local", "bin")
	if !ContainsPath(segments, localBin, false) {
		return false, nil
	}

	link, err := os.Readlink(filepath.Join(localBin, "adb"))
	if err == nil && strings.HasPrefix(link, dir) {
		return true, nil
	}
	return false, nil
}

func (l *linuxPlatformManager) AddToPath(dir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}

	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0755); err != nil {
		return fmt.Errorf("create ~/.local/bin: %w", err)
	}

	binaries := []string{"adb", "fastboot", "mke2fs", "sqlite3"}
	for _, bin := range binaries {
		src := filepath.Join(dir, bin)
		if _, err := os.Stat(src); err == nil {
			target := filepath.Join(localBin, bin)
			_ = os.Remove(target)
			if err := os.Symlink(src, target); err != nil {
				return fmt.Errorf("symlink %s into ~/.local/bin: %w", bin, err)
			}
		}
	}

	return nil
}

func (l *linuxPlatformManager) RemoveFromPath(dir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}

	localBin := filepath.Join(home, ".local", "bin")
	binaries := []string{"adb", "fastboot", "mke2fs", "sqlite3"}
	for _, bin := range binaries {
		target := filepath.Join(localBin, bin)
		if link, err := os.Readlink(target); err == nil {
			if strings.HasPrefix(link, dir) {
				_ = os.Remove(target)
			}
		}
	}

	return nil
}

func (l *linuxPlatformManager) OpenFilePicker(title string) (string, error) {
	// Try zenity if available
	if zenityPath, err := exec.LookPath("zenity"); err == nil {
		cmd := exec.Command(zenityPath, "--file-selection", "--title="+title)
		out, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}

	// Try kdialog if available
	if kdialogPath, err := exec.LookPath("kdialog"); err == nil {
		cmd := exec.Command(kdialogPath, "--getopenfilename", ".", "", "--title", title)
		out, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}

	// Fallback to terminal input prompt
	fmt.Printf("%s\nEnter file path: ", title)
	reader := bufio.NewReader(os.Stdin)
	text, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	clean := strings.TrimSpace(text)
	if clean == "" {
		return "", nil
	}
	return clean, nil
}
