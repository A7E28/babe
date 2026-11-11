package main

import (
	"os"

	"github.com/A7E28/babe/internal/platform"
)

// Injected at build time via -ldflags "-X main.version=vX.Y.Z"
var version = "dev"

func cleanupOldBinary() {
	if exePath, err := os.Executable(); err == nil {
		_ = os.Remove(exePath + ".old")
	}
}

func main() {
	_ = platform.InitConsole()
	cleanupOldBinary()

	if len(os.Args) > 1 {
		handleCLI(os.Args[1:])
		return
	}

	runInteractiveMenu()
}
