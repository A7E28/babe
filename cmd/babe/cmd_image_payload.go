package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/image"
	"github.com/A7E28/babe/internal/ui"
)

func newPayloadCmd() *cobra.Command {
	payloadCmd := &cobra.Command{
		Use:     "payload",
		Aliases: []string{"ota"},
		Short:   "Inspect and selectively extract Android A/B OTA payload.bin",
		Long:    "Pure-Go multi-core inspector and selective partition extractor for CrAU payload.bin and OTA .zip archives.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isJSON(cmd) {
				return fmt.Errorf("payload subcommand required (list or extract)")
			}
			return cmd.Help()
		},
	}

	payloadCmd.AddCommand(newPayloadListCmd(), newPayloadExtractCmd())
	return payloadCmd
}

func newPayloadListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list <ota.zip|payload.bin>",
		Aliases: []string{"ls", "info"},
		Short:   "List partitions, sizes, codecs, and SHA-256 digests in an OTA payload",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPayloadList(cmd, args[0])
		},
	}
}

func newPayloadExtractCmd() *cobra.Command {
	var parts []string
	var outDir string
	var extractAll bool
	var jobs int
	var noVerify bool
	var inspectBoot bool
	var noClobber bool

	extractCmd := &cobra.Command{
		Use:     "extract <ota.zip|payload.bin>",
		Aliases: []string{"dump"},
		Short:   "Extract selected partition(s) from an OTA zip or payload.bin",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			expanded := expandPartitionFlags(parts)
			for _, p := range expanded {
				if strings.HasPrefix(p, "-") {
					return fmt.Errorf("invalid partition name %q after -p (did you mean --all / -a or -p all?)", p)
				}
			}
			opts := image.PayloadExtractOptions{
				OutputDir:   outDir,
				Partitions:  expanded,
				All:         extractAll,
				Jobs:        jobs,
				Verify:      !noVerify,
				InspectBoot: inspectBoot,
				NoClobber:   noClobber,
			}
			return runPayloadExtract(cmd, args[0], opts)
		},
	}

	extractCmd.Flags().StringSliceVarP(&parts, "partition", "p", nil, "Partition name(s) to extract (repeatable, comma-separated, or 'all')")
	extractCmd.Flags().StringVarP(&outDir, "out", "o", ".", "Output directory for extracted .img files")
	extractCmd.Flags().BoolVarP(&extractAll, "all", "a", false, "Extract all extractable partitions")
	extractCmd.Flags().IntVarP(&jobs, "jobs", "j", 0, "Parallel decompression worker count (default: CPU core count)")
	extractCmd.Flags().BoolVar(&noVerify, "no-verify", false, "Skip SHA-256 verification")
	extractCmd.Flags().BoolVar(&inspectBoot, "inspect", false, "Also inspect extracted boot/init_boot headers")
	extractCmd.Flags().BoolVar(&noClobber, "no-clobber", false, "Fail instead of overwriting existing output .img files")

	return extractCmd
}

func expandPartitionFlags(raw []string) []string {
	var out []string
	for _, entry := range raw {
		for _, part := range strings.Split(entry, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}

func runPayloadList(cmd *cobra.Command, filePath string) error {
	info, err := image.InspectPayload(filePath)
	if err != nil {
		return err
	}
	if isJSON(cmd) {
		return writeJSON(cmd.OutOrStdout(), info)
	}
	ui.RenderPayloadInfoTo(cmd.OutOrStdout(), info)
	return nil
}

func runPayloadExtract(cmd *cobra.Command, filePath string, opts image.PayloadExtractOptions) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	res, err := image.ExtractPayload(ctx, filePath, opts)
	if err != nil {
		return err
	}
	if isJSON(cmd) {
		return writeJSON(cmd.OutOrStdout(), res)
	}
	ui.RenderPayloadExtractResultTo(cmd.OutOrStdout(), res)
	return nil
}
