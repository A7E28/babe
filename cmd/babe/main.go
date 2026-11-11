package main

import (
	"os"

	"github.com/A7E28/babe/internal/platform"
)

// Injected at build time via -ldflags "-X main.version=vX.Y.Z"
var version = "dev"

func main() {
	_ = platform.InitConsole()

	if len(os.Args) > 1 {
		handleCLI(os.Args[1:])
		return
	}

	runInteractiveMenu()
}
