package main

import (
	"os"
	"path/filepath"

	"github.com/A7E28/babe/internal/platform"
)

// Injected at build time via -ldflags "-X main.version=vX.Y.Z"
var version = "dev"

func cleanupOldBinary() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}
	_ = os.Remove(exePath + ".old")
	if matches, err := filepath.Glob(exePath + ".*.old"); err == nil {
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}
}

func main() {
	_ = platform.InitConsole()
	cleanupOldBinary()

	if len(os.Args) > 1 {
		args := os.Args[1:]
		noticeHandle := startBackgroundUpdateCheck(args)
		err := executeCLI(args)
		finishBackgroundUpdateCheck(noticeHandle, err == nil, os.Stderr)
		if err != nil {
			os.Exit(1)
		}
		return
	}

	runInteractiveMenu()
}
