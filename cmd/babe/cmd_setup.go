package main

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/platform"
)

func newSetupCmd(pm platform.PlatformManager) *cobra.Command {
	var force bool
	var status bool

	cmd := &cobra.Command{
		Use:     "setup",
		Short:   "Install babe executable globally into user PATH",
		Long:    "Install babe executable globally into user PATH.",
		GroupID: "core",
		RunE: func(cmd *cobra.Command, args []string) error {
			if isJSON(cmd) {
				return handleSetupJSON(cmd, pm, force, status)
			}
			runBabeSetup(pm, force, status)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force overwrite canonical binary")
	cmd.Flags().BoolVarP(&status, "status", "s", false, "Check setup status without modifying system")

	return cmd
}

func newInstallShortcutCmd(client *device.Client, pm platform.PlatformManager) *cobra.Command {
	var serial string

	cmd := &cobra.Command{
		Use:   "install [file|self]",
		Short: "Install package on device, or install babe globally",
		Long:  "Install package on connected device, or install babe into user PATH ('babe install self').",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || strings.EqualFold(args[0], "self") || strings.EqualFold(args[0], "babe") {
				if isJSON(cmd) {
					return handleSetupJSON(cmd, pm, false, false)
				}
				runBabeSetup(pm, false, false)
				return nil
			}
			return handleInstallCLI(cmd, client, args[0], serial)
		},
	}

	cmd.Flags().StringVarP(&serial, "serial", "s", "", "Target specific device serial")
	return cmd
}
