package main

import (
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleUdevCLI(cmd)
		},
	}
}

func handleUdevCLI(cmd *cobra.Command) error {
	if err := platform.InstallUdevRules(); err != nil {
		return err
	}
	if isJSON(cmd) {
		return writeJSON(cmd.OutOrStdout(), udevReport{Installed: true})
	}
	ui.Success("Android udev rules installed and reloaded.")
	return nil
}
