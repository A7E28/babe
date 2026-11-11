//go:build windows

package platform

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                       = windows.NewLazySystemDLL("kernel32.dll")
	procFillConsoleOutputCharacter = kernel32.NewProc("FillConsoleOutputCharacterW")
	procFillConsoleOutputAttribute = kernel32.NewProc("FillConsoleOutputAttribute")
)

func initConsolePlatform() error {
	handle := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return err
	}
	mode |= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	return windows.SetConsoleMode(handle, mode)
}

func clearConsolePlatform() {
	fmt.Print("\x1b[2J\x1b[3J\x1b[H")

	// 2. Fallback sweep for legacy Win32 console buffer
	handle := windows.Handle(os.Stdout.Fd())
	var csbi windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(handle, &csbi); err == nil {
		size := uint32(csbi.Size.X) * uint32(csbi.Size.Y)
		var written uint32
		_, _, _ = procFillConsoleOutputCharacter.Call(uintptr(handle), uintptr(' '), uintptr(size), 0, uintptr(unsafe.Pointer(&written)))
		_, _, _ = procFillConsoleOutputAttribute.Call(uintptr(handle), uintptr(csbi.Attributes), uintptr(size), 0, uintptr(unsafe.Pointer(&written)))
		_ = windows.SetConsoleCursorPosition(handle, windows.Coord{X: 0, Y: 0})
	}
}
