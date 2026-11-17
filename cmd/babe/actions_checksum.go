package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

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

	if err := printFileHashes(os.Stdout, filePath); err != nil {
		ui.Error("Hash calculation failed: %v", err)
	}
}

func printFileHashes(w io.Writer, filePath string) error {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintf(w, "\n%s\n", ui.Gray("Computing checksums..."))
	if w == os.Stdout {
		_ = os.Stdout.Sync()
	}

	digests, err := hash.CalculateAllFileHashes(filePath)
	if err != nil {
		return err
	}

	ui.RenderChecksums(filePath, digests.MD5, digests.SHA1, digests.SHA256)
	return nil
}
