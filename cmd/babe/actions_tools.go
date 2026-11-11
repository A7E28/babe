package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/logcat"
	"github.com/A7E28/babe/internal/ui"
)

func logcatMenu(client *device.Client, reader *bufio.Reader) {
	ui.Section("Logcat & Crash Forensics")
	ui.RenderMenuItem(1, "Live Colored Logcat Stream")
	ui.RenderMenuItem(2, "Harvest Crash & Native Tombstone Buffer (Instant Dump)")
	ui.RenderMenuItem(3, "Cancel")
	ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Enter choice [1-3]:")))

	choiceStr, ok := readLine(reader)
	if !ok {
		return
	}
	switch choiceStr {
	case "1":
		startLogcatStream(client, reader)
	case "2":
		harvestCrashLogs(client, reader)
	}
}

func harvestCrashLogs(client *device.Client, reader *bufio.Reader) {
	serial, err := selectDevice(client, reader)
	if err != nil {
		ui.Warn("%v", err)
		return
	}

	ui.Prompt(fmt.Sprintf("%s %s: ", ui.White("Save crash report to file"), ui.Gray("[optional]")))
	outFile, ok := readLine(reader)
	if !ok {
		return
	}

	ui.Divider()
	count, err := logcat.DumpCrashLogs(context.Background(), client.BinaryPath(), serial, outFile, func(entry logcat.Entry) {
		fmt.Println(logcat.Colorize(entry))
	})
	ui.Divider()

	if err != nil {
		ui.Error("Crash harvest failed: %v", err)
		return
	}
	if count == 0 {
		ui.Info("No crash entries found in device crash buffer.")
		return
	}
	ui.Success("Harvested %d crash log line(s).", count)
	if outFile != "" {
		ui.Success("Crash report saved to: %s", outFile)
	}
}

func startLogcatStream(client *device.Client, reader *bufio.Reader) {
	serial, err := selectDevice(client, reader)
	if err != nil {
		ui.Warn("%v", err)
		return
	}

	fmt.Println(ui.Gray("Select minimum log level:"))
	ui.RenderMenuItem(1, "Verbose (V)")
	ui.RenderMenuItem(2, "Debug (D)")
	ui.RenderMenuItem(3, "Info (I)")
	ui.RenderMenuItem(4, "Warning (W)")
	ui.RenderMenuItem(5, "Error (E)")
	ui.RenderMenuItem(6, "Fatal (F)")
	ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Select log level [1-6, default 1]:")))
	lvlInput, ok := readLine(reader)
	if !ok {
		return
	}
	lvl := logcat.LevelVerbose
	switch lvlInput {
	case "2":
		lvl = logcat.LevelDebug
	case "3":
		lvl = logcat.LevelInfo
	case "4":
		lvl = logcat.LevelWarn
	case "5":
		lvl = logcat.LevelError
	case "6":
		lvl = logcat.LevelFatal
	}

	ui.Prompt(fmt.Sprintf("%s %s: ", ui.White("Tag filter"), ui.Gray("[optional]")))
	tagFilter, ok := readLine(reader)
	if !ok {
		return
	}

	ui.Prompt(fmt.Sprintf("%s %s: ", ui.White("Package name or PID filter"), ui.Gray("[optional]")))
	pkgFilter, ok := readLine(reader)
	if !ok {
		return
	}

	ui.Prompt(fmt.Sprintf("%s %s: ", ui.White("Save output file path"), ui.Gray("[optional]")))
	outFile, ok := readLine(reader)
	if !ok {
		return
	}

	ui.Info("Streaming logcat. Press Ctrl+C to stop...")
	ui.Divider()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := logcat.StreamOptions{
		ADBBinary:   client.BinaryPath(),
		Serial:      serial,
		MinLevel:    lvl,
		TagFilter:   tagFilter,
		PackageName: pkgFilter,
		OutputFile:  outFile,
	}

	err = logcat.Stream(ctx, opts, func(entry logcat.Entry) {
		fmt.Println(logcat.Colorize(entry))
	})

	if err != nil && ctx.Err() == nil {
		ui.Error("Logcat stream error: %v", err)
	} else {
		ui.Success("Logcat stream stopped.")
		if outFile != "" {
			ui.Success("Log output saved to: %s", outFile)
		}
	}
}
