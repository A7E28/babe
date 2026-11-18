package ui

import (
	"fmt"
	"io"
	"os"

	"github.com/A7E28/babe/internal/platform"
)

func ClearScreen() {
	platform.ClearConsole()
}

func Section(title string) {
	fmt.Printf("\n%s %s\n", Gray("───"), White(title))
}

func Divider() {
	fmt.Println(Gray("──────────────────────────────────────────────────"))
}

func Success(format string, args ...any) {
	tag := wrap(ansiBoldGreen, "[OK]")
	fmt.Printf("%s "+format+"\n", append([]any{tag}, args...)...)
}

func ErrorTo(w io.Writer, format string, args ...any) {
	if w == nil {
		w = os.Stdout
	}
	tag := wrap(ansiBoldRed, "[ERROR]")
	fmt.Fprintf(w, "%s "+format+"\n", append([]any{tag}, args...)...)
}

func Error(format string, args ...any) {
	ErrorTo(os.Stdout, format, args...)
}

func Info(format string, args ...any) {
	tag := wrap(ansiBoldCyan, "[INFO]")
	fmt.Printf("%s "+format+"\n", append([]any{tag}, args...)...)
}

func Warn(format string, args ...any) {
	tag := wrap(ansiBoldWarn, "[!]")
	fmt.Printf("%s "+format+"\n", append([]any{tag}, args...)...)
}

func Prompt(text string) {
	fmt.Print(text)
	_ = os.Stdout.Sync()
}
