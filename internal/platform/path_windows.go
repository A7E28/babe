//go:build windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	procSendMessageTimeout = user32.NewProc("SendMessageTimeoutW")
)

const (
	HWND_BROADCAST   = 0xFFFF
	WM_SETTINGCHANGE = 0x001A
	SMTO_ABORTIFHUNG = 0x0002
)

type windowsPlatformManager struct{}

func defaultManager() PlatformManager {
	return &windowsPlatformManager{}
}

func resolveLocalAppData() (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	return localAppData, nil
}

func (w *windowsPlatformManager) GetDefaultInstallDir() (string, error) {
	base, err := resolveLocalAppData()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "babe", "platform-tools"), nil
}

func (w *windowsPlatformManager) GetBabeBinDir() (string, error) {
	base, err := resolveLocalAppData()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "babe", "bin"), nil
}

func (w *windowsPlatformManager) getRegistryPath() (string, uint32, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", registry.EXPAND_SZ, nil
		}
		return "", 0, fmt.Errorf("open Environment key: %w", err)
	}
	defer k.Close()

	val, valType, err := k.GetStringValue("Path")
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", registry.EXPAND_SZ, nil
		}
		return "", 0, fmt.Errorf("read Path value: %w", err)
	}

	return val, valType, nil
}

func (w *windowsPlatformManager) IsInPath(dir string) (bool, error) {
	currentPath, _, err := w.getRegistryPath()
	if err != nil {
		return false, err
	}

	segments := CleanPathString(currentPath, ';')
	return ContainsPath(segments, dir, true), nil
}

func (w *windowsPlatformManager) AddToPath(dir string) error {
	inPath, err := w.IsInPath(dir)
	if err != nil {
		return err
	}
	if inPath {
		return ErrPathAlreadyExists
	}

	currentPath, valType, err := w.getRegistryPath()
	if err != nil {
		return err
	}

	var newPath string
	if currentPath == "" {
		newPath = dir
	} else {
		newPath = currentPath + ";" + dir
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Environment registry key for writing: %w", err)
	}
	defer k.Close()

	// Crucial: preserve REG_EXPAND_SZ so %USERPROFILE% etc. stay expandable!
	if valType == registry.SZ {
		err = k.SetStringValue("Path", newPath)
	} else {
		err = k.SetExpandStringValue("Path", newPath)
	}

	if err != nil {
		return fmt.Errorf("write updated Path to registry: %w", err)
	}

	procPath := os.Getenv("PATH")
	if !ContainsPath(CleanPathString(procPath, ';'), dir, true) {
		_ = os.Setenv("PATH", procPath+";"+dir)
	}

	broadcastEnvironmentChange()
	return nil
}

func (w *windowsPlatformManager) RemoveFromPath(dir string) error {
	currentPath, valType, err := w.getRegistryPath()
	if err != nil {
		return err
	}

	segments := CleanPathString(currentPath, ';')
	filtered := FilterOutPath(segments, dir, true)
	newPath := FormatPathString(filtered, ';')

	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Environment registry key for writing: %w", err)
	}
	defer k.Close()

	if valType == registry.SZ {
		err = k.SetStringValue("Path", newPath)
	} else {
		err = k.SetExpandStringValue("Path", newPath)
	}

	if err != nil {
		return fmt.Errorf("write updated Path to registry: %w", err)
	}

	broadcastEnvironmentChange()
	return nil
}

func broadcastEnvironmentChange() {
	envStr, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}

	var result uintptr
	_, _, _ = procSendMessageTimeout.Call(
		uintptr(HWND_BROADCAST),
		uintptr(WM_SETTINGCHANGE),
		0,
		uintptr(unsafe.Pointer(envStr)),
		uintptr(SMTO_ABORTIFHUNG),
		5000,
		uintptr(unsafe.Pointer(&result)),
	)
}
