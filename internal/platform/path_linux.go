//go:build linux

package platform

import (
	"errors"
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

func (l *linuxPlatformManager) GetBabeBinDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "bin"), nil
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

	if filepath.Clean(dir) != filepath.Clean(localBin) {
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
	}

	ensureShellRC(home, localBin)
	return nil
}

func ensureShellRC(home, localBin string) {
	currentPath := os.Getenv("PATH")
	segments := CleanPathString(currentPath, ':')
	if !ContainsPath(segments, localBin, false) {
		cleanCurrent := strings.TrimLeft(strings.TrimSpace(currentPath), ":")
		if cleanCurrent == "" {
			_ = os.Setenv("PATH", localBin)
		} else {
			_ = os.Setenv("PATH", localBin+":"+cleanCurrent)
		}
	}

	rcFiles := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
	}
	if strings.Contains(os.Getenv("SHELL"), "zsh") {
		rcFiles = []string{filepath.Join(home, ".zshrc"), filepath.Join(home, ".bashrc")}
	}

	exportLine := "\n# Added by babe\nexport PATH=\"$HOME/.local/bin:$PATH\"\n"
	updatedAny := false
	for _, rc := range rcFiles {
		data, err := os.ReadFile(rc)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), ".local/bin") {
			updatedAny = true
			continue
		}
		f, err := os.OpenFile(rc, os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			_, _ = f.WriteString(exportLine)
			_ = f.Close()
			updatedAny = true
		}
	}

	if !updatedAny {
		profile := filepath.Join(home, ".profile")
		data, _ := os.ReadFile(profile)
		if !strings.Contains(string(data), ".local/bin") {
			if f, err := os.OpenFile(profile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
				_, _ = f.WriteString(exportLine)
				_ = f.Close()
			}
		}
	}
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
	if zenityPath, err := exec.LookPath("zenity"); err == nil {
		cmd := exec.Command(zenityPath, "--file-selection", "--title="+title)
		if out, err := cmd.Output(); err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}

	if kdialogPath, err := exec.LookPath("kdialog"); err == nil {
		cmd := exec.Command(kdialogPath, "--getopenfilename", ".", "", "--title", title)
		if out, err := cmd.Output(); err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}

	return "", errors.New("no graphical file dialog (zenity/kdialog) available")
}
