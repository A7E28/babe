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
		RunE: func(cmd *cobra.Command, args []string) error {
			parsedLevel, err := logcat.ValidateLevel(level)
			if err != nil {
				return fmt.Errorf("%w. Valid levels: V, D, I, W, E, F", err)
			}
			return handleLogcatCLI(cmd, client, logcat.StreamOptions{
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleCrashLogCLI(cmd, client, serial, crashOut)
		},
	}
	crashCmd.Flags().StringVarP(&crashOut, "output", "o", "", "Save crash report to specified file")

	cmd.AddCommand(crashCmd)
	return cmd
}

func handleLogcatCLI(cmd *cobra.Command, client *device.Client, opts logcat.StreamOptions) error {
	targetSerial, err := resolveTargetDevice(cmd, client, opts.Serial)
	if err != nil {
		return err
	}
	opts.Serial = targetSerial
	opts.ADBBinary = client.BinaryPath()

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(sigCtx)
	defer cancel()

	jsonMode := isJSON(cmd)
	outWriter := cmd.OutOrStdout()
	var writeErr error
	err = logcat.Stream(ctx, opts, func(entry logcat.Entry) {
		if jsonMode {
			if wErr := writeJSONLine(outWriter, entry); wErr != nil {
				writeErr = wErr
				cancel()
			}
			return
		}
		fmt.Fprintln(outWriter, logcat.Colorize(entry))
	})
	if writeErr != nil {
		return writeErr
	}
	if err != nil && ctx.Err() == nil {
		return fmt.Errorf("logcat stream error: %w", err)
	}
	return nil
}

func handleCrashLogCLI(cmd *cobra.Command, client *device.Client, serial, outFile string) error {
	targetSerial, err := resolveTargetDevice(cmd, client, serial)
	if err != nil {
		return err
	}

	jsonMode := isJSON(cmd)
	outWriter := cmd.OutOrStdout()
	entries := []logcat.Entry{}
	count, err := logcat.DumpCrashLogs(context.Background(), client.BinaryPath(), targetSerial, outFile, func(entry logcat.Entry) {
		if jsonMode {
			entries = append(entries, entry)
			return
		}
		fmt.Fprintln(outWriter, logcat.Colorize(entry))
	})
	if err != nil {
		return fmt.Errorf("crash log dump failed: %w", err)
	}
	if jsonMode {
		return writeJSON(outWriter, crashLogReport{
			Serial:     targetSerial,
			Count:      count,
			OutputFile: outFile,
			Entries:    entries,
		})
	}
	if count == 0 {
		ui.Info("No crash entries found in device crash buffer.")
		return nil
	}
	ui.Info("Dumped %d crash log line(s) from device %s.", count, targetSerial)
	if outFile != "" {
		ui.Success("Saved crash report to %s", outFile)
	}
	return nil
}
