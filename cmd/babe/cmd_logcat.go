package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/logcat"
	"github.com/A7E28/babe/internal/ui"
)

func newLogcatCmd(client *device.Client) *cobra.Command {
	var serial string
	var level string
	var tag string
	var pkg string
	var out string

	cmd := &cobra.Command{
		Use:     "logcat",
		Short:   "Stream and filter device logcat messages in real time",
		Long:    "Stream and filter device logcat messages in real time.",
		GroupID: "pkg",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			parsedLevel, err := logcat.ValidateLevel(level)
			if err != nil {
				ui.Error("%v. Valid levels: V, D, I, W, E, F.", err)
				os.Exit(1)
			}
			handleLogcatCLI(client, logcat.StreamOptions{
				Serial:      serial,
				MinLevel:    parsedLevel,
				TagFilter:   tag,
				PackageName: pkg,
				OutputFile:  out,
			})
		},
	}

	cmd.PersistentFlags().StringVarP(&serial, "serial", "s", "", "Target specific device serial (default: auto-detected)")
	cmd.Flags().StringVarP(&level, "level", "v", "V", "Minimum log level: V, D, I, W, E, F")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Filter log entries matching tag substring")
	cmd.Flags().StringVarP(&pkg, "package", "p", "", "Filter by package name or process ID")
	cmd.Flags().StringVarP(&out, "output", "o", "", "Tee log output to a file")

	var crashOut string
	crashCmd := &cobra.Command{
		Use:   "crash",
		Short: "Dump device crash logs, tombstones, and native crash buffers",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			handleCrashLogCLI(client, serial, crashOut)
		},
	}
	crashCmd.Flags().StringVarP(&crashOut, "output", "o", "", "Save crash report to specified file")

	cmd.AddCommand(crashCmd)
	return cmd
}

func handleLogcatCLI(client *device.Client, opts logcat.StreamOptions) {
	targetSerial, err := resolveTargetDevice(client, opts.Serial)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}
	opts.Serial = targetSerial
	opts.ADBBinary = client.BinaryPath()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err = logcat.Stream(ctx, opts, func(entry logcat.Entry) {
		fmt.Println(logcat.Colorize(entry))
	})
	if err != nil && ctx.Err() == nil {
		ui.Error("Logcat stream error: %v", err)
		os.Exit(1)
	}
}

func handleCrashLogCLI(client *device.Client, serial, outFile string) {
	targetSerial, err := resolveTargetDevice(client, serial)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	count, err := logcat.DumpCrashLogs(context.Background(), client.BinaryPath(), targetSerial, outFile, func(entry logcat.Entry) {
		fmt.Println(logcat.Colorize(entry))
	})
	if err != nil {
		ui.Error("Crash log dump failed: %v", err)
		os.Exit(1)
	}
	if count == 0 {
		ui.Info("No crash entries found in device crash buffer.")
		return
	}
	ui.Info("Dumped %d crash log line(s) from device %s.", count, targetSerial)
	if outFile != "" {
		ui.Success("Saved crash report to %s", outFile)
	}
}
