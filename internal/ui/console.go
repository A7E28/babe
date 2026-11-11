package ui

import (
	"fmt"
	"os"

	"github.com/A7E28/babe/internal/platform"
)

// completely resets both visible viewport and scrollback buffer.
func ClearScreen() {
	platform.ClearConsole()
}

// prints a clean, styled section divider.
func Section(title string) {
	fmt.Printf("\n%s %s\n", Gray("───"), White(title))
}

// prints a muted horizontal separator.
func Divider() {
	fmt.Println(Gray("──────────────────────────────────────────────────"))
}

// prints a standardized [OK] message.
func Success(format string, args ...any) {
	tag := wrap(ansiBoldGreen, "[OK]")
	fmt.Printf("%s "+format+"\n", append([]any{tag}, args...)...)
}

// prints a standardized [ERROR] message.
func Error(format string, args ...any) {
	tag := wrap(ansiBoldRed, "[ERROR]")
	fmt.Printf("%s "+format+"\n", append([]any{tag}, args...)...)
}

// prints a standardized [INFO] message.
func Info(format string, args ...any) {
	tag := wrap(ansiBoldCyan, "[INFO]")
	fmt.Printf("%s "+format+"\n", append([]any{tag}, args...)...)
}

// prints a standardized [!] warning message.
func Warn(format string, args ...any) {
	tag := wrap(ansiBoldWarn, "[!]")
	fmt.Printf("%s "+format+"\n", append([]any{tag}, args...)...)
}

// prints a prompt string without a trailing newline and flushes stdout.
func Prompt(text string) {
	fmt.Print(text)
	_ = os.Stdout.Sync()
}
