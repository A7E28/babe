package main

import (
	"bufio"
	"fmt"

	"github.com/A7E28/babe/internal/hash"
	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/ui"
)

func checksumMenu(pm platform.PlatformManager, reader *bufio.Reader) {
	ui.Section("Calculate File Checksum")

	filePath := promptFileWithFallback(pm, reader, "Select File for Checksum")
	if filePath == "" {
		return
	}

	if err := printFileHashes(filePath); err != nil {
		ui.Error("Hash calculation failed: %v", err)
	}
}

func printFileHashes(filePath string) error {
	ui.Prompt(fmt.Sprintf("\n%s\n", ui.Gray("Computing checksums...")))

	digests, err := hash.CalculateAllFileHashes(filePath)
	if err != nil {
		return err
	}

	ui.RenderChecksums(filePath, digests.MD5, digests.SHA1, digests.SHA256)
	return nil
}
