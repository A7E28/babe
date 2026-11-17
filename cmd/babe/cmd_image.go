package main

import (
	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/image"
	"github.com/A7E28/babe/internal/ui"
)

func newImageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "image <boot.img|init_boot.img>",
		Aliases: []string{"img"},
		Short:   "Offline Android boot.img / init_boot.img header parser",
		Long:    "Offline parser for Android boot.img and init_boot.img headers (v0 through v4).",
		GroupID: "pkg",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				_ = cmd.Help()
				return nil
			}
			return runInspectBootImage(cmd, args[0])
		},
	}

	inspectCmd := &cobra.Command{
		Use:     "inspect <boot.img|init_boot.img>",
		Aliases: []string{"info"},
		Short:   "Inspect boot.img or init_boot.img header",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspectBootImage(cmd, args[0])
		},
	}

	cmd.AddCommand(inspectCmd)
	return cmd
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
