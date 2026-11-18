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

func Bold(s string) string {
	return wrap(ansiBold, s)
}

func Dim(s string) string {
	return wrap(ansiDim, s)
}

func Gray(s string) string {
	return wrap(ansiGray, s)
}

func Cyan(s string) string {
	return wrap(ansiCyan, s)
}

func BoldCyan(s string) string {
	return wrap(ansiBoldCyan, s)
}

func Green(s string) string {
	return wrap(ansiGreen, s)
}

func Yellow(s string) string {
	return wrap(ansiYellow, s)
}

func Red(s string) string {
	return wrap(ansiRed, s)
}

func White(s string) string {
	return wrap(ansiBoldWhite, s)
}
