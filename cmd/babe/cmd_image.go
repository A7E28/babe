package main

import (
	"os"

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
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				_ = cmd.Help()
				return
			}
			runInspectBootImage(args[0])
		},
	}

	inspectCmd := &cobra.Command{
		Use:     "inspect <boot.img|init_boot.img>",
		Aliases: []string{"info"},
		Short:   "Inspect boot.img or init_boot.img header",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runInspectBootImage(args[0])
		},
	}

	cmd.AddCommand(inspectCmd)
	return cmd
}

func runInspectBootImage(filePath string) {
	info, err := image.InspectBootImage(filePath)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}
	ui.RenderBootImageInfo(info)
}
