package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/A7E28/babe/internal/image"
	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/ui"
)

func imageSuiteMenu(pm platform.PlatformManager, reader *bufio.Reader) {
	ui.Section("Firmware & OTA Image Suite (Offline)")
	ui.RenderMenuItem(1, "Inspect Boot Image Header (boot.img / init_boot.img)")
	ui.RenderMenuItem(2, "List OTA Payload Partitions (ota.zip / payload.bin)")
	ui.RenderMenuItem(3, "Extract Partition(s) from OTA Payload (Multi-Core)")
	ui.RenderMenuItem(4, "Cancel")
	ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Enter choice [1-4]:")))

	choice, ok := readLine(reader)
	if !ok {
		return
	}
	switch choice {
	case "1":
		bootImageMenu(pm, reader)
	case "2":
		otaPath := promptFileWithFallback(pm, reader, "Select OTA .zip or payload.bin")
		if otaPath == "" {
			return
		}
		info, err := image.InspectPayload(otaPath)
		if err != nil {
			ui.Error("Payload inspection failed: %v", err)
			return
		}
		ui.RenderPayloadInfo(info)
	case "3":
		interactiveExtractPayload(pm, reader)
	case "4", "q", "cancel":
		return
	default:
		ui.Warn("Invalid selection. Choose an option from 1 to 4.")
	}
}

func interactiveExtractPayload(pm platform.PlatformManager, reader *bufio.Reader) {
	otaPath := promptFileWithFallback(pm, reader, "Select OTA .zip or payload.bin")
	if otaPath == "" {
		return
	}

	sess, err := image.OpenPayload(otaPath)
	if err != nil {
		ui.Error("Payload inspection failed: %v", err)
		return
	}
	defer sess.Close()

	info := sess.Info()
	ui.RenderPayloadInfo(info)

	ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Partition(s) to extract (comma-separated, or 'all'):")))
	rawParts, ok := readLine(reader)
	if !ok || rawParts == "" {
		ui.Warn("No partition specified.")
		return
	}

	hasLiteralAll := false
	for _, p := range info.Partitions {
		if strings.EqualFold(p.Name, "all") {
			hasLiteralAll = true
			break
		}
	}
	extractAll := strings.EqualFold(rawParts, "all") && !hasLiteralAll
	var parsed []string
	if !extractAll {
		parsed = expandPartitionFlags([]string{rawParts})
		if len(parsed) == 0 {
			ui.Warn("No partition specified.")
			return
		}
	}

	ui.Prompt(fmt.Sprintf("%s ", ui.White("Output directory [default: current directory]:")))
	outDir, ok := readLine(reader)
	if !ok {
		return
	}
	if outDir == "" {
		outDir = "."
	}

	opts := image.PayloadExtractOptions{
		OutputDir:   outDir,
		Verify:      true,
		InspectBoot: true,
		All:         extractAll,
		Partitions:  parsed,
	}

	if hasExistingTargetImg(outDir, info, opts) {
		ui.Prompt(fmt.Sprintf("%s ", ui.White("Existing .img file(s) found in output directory. Overwrite? [y/N]:")))
		ans, ansOK := readLine(reader)
		if !ansOK || (!strings.EqualFold(ans, "y") && !strings.EqualFold(ans, "yes")) {
			ui.Info("Extraction cancelled.")
			return
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	res, err := sess.Extract(ctx, opts)
	if err != nil {
		ui.Error("Payload extraction failed: %v", err)
		return
	}
	ui.RenderPayloadExtractResult(res)
}

func hasExistingTargetImg(outDir string, info *image.PayloadInfo, opts image.PayloadExtractOptions) bool {
	var names []string
	if opts.All {
		for _, p := range info.Partitions {
			if p.Extractable {
				names = append(names, p.Name)
			}
		}
	} else {
		byLower := make(map[string]string, len(info.Partitions))
		for _, p := range info.Partitions {
			byLower[strings.ToLower(p.Name)] = p.Name
		}
		for _, raw := range opts.Partitions {
			if canonical, ok := byLower[strings.ToLower(raw)]; ok {
				names = append(names, canonical)
			} else if filepath.Base(raw) == raw {
				names = append(names, raw)
			}
		}
	}
	for _, name := range names {
		if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
			continue
		}
		if _, err := os.Lstat(filepath.Join(outDir, name+".img")); err == nil {
			return true
		}
	}
	return false
}
