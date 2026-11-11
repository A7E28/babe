package ui

import (
	"fmt"

	"github.com/A7E28/babe/internal/platform"
)

const (
	ansiReset     = "\x1b[0m"
	ansiBold      = "\x1b[1m"
	ansiDim       = "\x1b[2m"
	ansiRed       = "\x1b[31m"
	ansiGreen     = "\x1b[32m"
	ansiYellow    = "\x1b[33m"
	ansiBlue      = "\x1b[34m"
	ansiMagenta   = "\x1b[35m"
	ansiCyan      = "\x1b[36m"
	ansiWhite     = "\x1b[37m"
	ansiGray      = "\x1b[90m"
	ansiBoldCyan  = "\x1b[1;36m"
	ansiBoldGreen = "\x1b[1;32m"
	ansiBoldRed   = "\x1b[1;31m"
	ansiBoldWarn  = "\x1b[1;33m"
	ansiBoldWhite = "\x1b[1;37m"
)

func wrap(code, s string) string {
	if !platform.ColorsSupported() || s == "" {
		return s
	}
	return fmt.Sprintf("%s%s%s", code, s, ansiReset)
}

// returns text styled in bold.
func Bold(s string) string {
	return wrap(ansiBold, s)
}

// returns text styled with dimmed/faded intensity.
func Dim(s string) string {
	return wrap(ansiDim, s)
}

// returns text in muted dark gray.
func Gray(s string) string {
	return wrap(ansiGray, s)
}

// returns text in standard cyan.
func Cyan(s string) string {
	return wrap(ansiCyan, s)
}

// returns text in bold cyan.
func BoldCyan(s string) string {
	return wrap(ansiBoldCyan, s)
}

// returns text in standard green.
func Green(s string) string {
	return wrap(ansiGreen, s)
}

// returns text in warning amber/yellow.
func Yellow(s string) string {
	return wrap(ansiYellow, s)
}

// returns text in error red.
func Red(s string) string {
	return wrap(ansiRed, s)
}

// returns text in bright white.
func White(s string) string {
	return wrap(ansiBoldWhite, s)
}
