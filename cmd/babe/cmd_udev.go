package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/ui"
)

func newUdevCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "udev",
		Hidden: true,
		Short:  "Install and reload Linux Android USB udev rules",
		Args:   cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			handleUdevCLI()
		},
	}
}

func handleUdevCLI() {
	if err := platform.InstallUdevRules(); err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}
	ui.Success("Android udev rules installed and reloaded.")
}
