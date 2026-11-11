//go:build windows

package platform

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	comdlg32                 = windows.NewLazySystemDLL("comdlg32.dll")
	procGetOpenFileName      = comdlg32.NewProc("GetOpenFileNameW")
	procCommDlgExtendedError = comdlg32.NewProc("CommDlgExtendedError")
)

type openFileNameW struct {
	lStructSize       uint32
	hwndOwner         windows.HWND
	hInstance         windows.Handle
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        unsafe.Pointer
	dwReserved        uint32
	flagsEx           uint32
}

func utf16Filter(pairs ...string) []uint16 {
	var filter []uint16
	for _, p := range pairs {
		filter = append(filter, utf16.Encode([]rune(p))...)
		filter = append(filter, 0)
	}
	filter = append(filter, 0)
	return filter
}

func (w *windowsPlatformManager) OpenFilePicker(title string) (string, error) {
	titlePtr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return "", err
	}

	filter := utf16Filter("All Files (*.*)", "*.*")
	fileBuffer := make([]uint16, 2048)

	var ofn openFileNameW
	ofn.lStructSize = uint32(unsafe.Sizeof(ofn))
	ofn.lpstrFilter = &filter[0]
	ofn.nFilterIndex = 1
	ofn.lpstrFile = &fileBuffer[0]
	ofn.nMaxFile = uint32(len(fileBuffer))
	ofn.lpstrTitle = titlePtr
	ofn.flags = 0x00000800 | 0x00001000 | 0x00000008 | 0x00080000

	ret, _, _ := procGetOpenFileName.Call(uintptr(unsafe.Pointer(&ofn)))
	if ret != 0 {
		return syscall.UTF16ToString(fileBuffer), nil
	}

	errCode, _, _ := procCommDlgExtendedError.Call()
	if errCode == 0 {
		return "", nil
	}

	return openFilePickerPSFallback(title)
}

func openFilePickerPSFallback(title string) (string, error) {
	escapedTitle := strings.ReplaceAll(title, "'", "''")
	script := fmt.Sprintf(
		`Add-Type -AssemblyName System.Windows.Forms; $f = New-Object System.Windows.Forms.OpenFileDialog; $f.Title = '%s'; if ($f.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $f.FileName }`,
		escapedTitle,
	)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("file dialog error: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
