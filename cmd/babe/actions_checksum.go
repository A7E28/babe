package main

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/A7E28/babe/internal/hash"
	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/ui"
)

func checksumMenu(pm platform.PlatformManager, reader *bufio.Reader) {
	ui.Section("Calculate File Checksum")

	filePath, err := pm.OpenFilePicker("Select File for Checksum")
	if err != nil {
		ui.Warn("File dialog unavailable: %v", err)
		ui.Prompt("Enter file path: ")
		input, _ := reader.ReadString('\n')
		filePath = strings.TrimSpace(input)
	}

	if filePath == "" {
		ui.Warn("No file selected.")
		return
	}

	printFileHashes(filePath)
}

func printFileHashes(filePath string) {
	ui.Prompt(fmt.Sprintf("\n%s\n", ui.Gray("Computing checksums...")))

	digests, err := hash.CalculateAllFileHashes(filePath)
	if err != nil {
		ui.Error("Hash calculation failed: %v", err)
		return
	}

	ui.RenderChecksums(filePath, digests.MD5, digests.SHA1, digests.SHA256)
}
