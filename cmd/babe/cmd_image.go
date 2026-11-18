package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/image"
	"github.com/A7E28/babe/internal/ui"
)

func newImageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "image <boot.img|ota.zip|payload.bin>",
		Aliases: []string{"img"},
		Short:   "Offline Android boot.img inspector & OTA payload/ROM extractor",
		Long:    "Offline parser for Android boot.img/init_boot.img headers (v0-v4) and multi-core OTA payload.bin / legacy ROM zip inspector & extractor.",
		GroupID: "pkg",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if isJSON(cmd) {
					return fmt.Errorf("image file path required")
				}
				return cmd.Help()
			}
			if args[0] == "help" && !isJSON(cmd) {
				if _, err := os.Stat("help"); err != nil {
					return cmd.Help()
				}
			}
			return runInspectAnyImage(cmd, args[0])
		},
	}

	inspectCmd := &cobra.Command{
		Use:     "inspect <boot.img|ota.zip|payload.bin>",
		Aliases: []string{"info"},
		Short:   "Inspect boot.img header or OTA payload/ROM zip partitions",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspectAnyImage(cmd, args[0])
		},
	}

	cmd.AddCommand(inspectCmd, newPayloadListCmd(), newPayloadExtractCmd(), newPayloadCmd())
	return cmd
}

func runInspectAnyImage(cmd *cobra.Command, filePath string) error {
	if isPayloadOrZipFile(filePath) {
		return runPayloadList(cmd, filePath)
	}
	return runInspectBootImage(cmd, filePath)
}

func isPayloadOrZipFile(filePath string) bool {
	lower := strings.ToLower(filePath)
	if strings.HasSuffix(lower, ".tar.md5") || strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar.gz") {
		return true
	}
	f, err := os.Open(filePath)
	if err != nil {
		return strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".bin")
	}
	defer f.Close()
	var magic [12]byte
	n, _ := f.ReadAt(magic[:], 0)
	if n >= len(image.BootMagic) && string(magic[:len(image.BootMagic)]) == image.BootMagic {
		return false
	}
	if strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".bin") {
		return true
	}
	if n < 4 {
		return false
	}
	return string(magic[:4]) == image.PayloadMagic || (magic[0] == 'P' && magic[1] == 'K') || (n == 12 && string(magic[:]) == "OPPOENCRYPT!")
}

func runInspectBootImage(cmd *cobra.Command, filePath string) error {
	info, err := image.InspectBootImage(filePath)
	if err != nil {
		return err
	}
	if isJSON(cmd) {
		return writeJSON(cmd.OutOrStdout(), info)
	}
	ui.RenderBootImageInfo(info)
	return nil
}
