package main

import (
	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/platform"
)

func newToolsCmd(client *device.Client, pm platform.PlatformManager) *cobra.Command {
	toolsCmd := &cobra.Command{
		Use:     "tools",
		Short:   "Manage Android SDK platform-tools (adb and fastboot)",
		Long:    "Manage Android SDK platform-tools (adb and fastboot).",
		GroupID: "core",
		RunE: func(cmd *cobra.Command, args []string) error {
			if isJSON(cmd) {
				return handleToolsStatusJSON(cmd, client, pm)
			}
			checkPlatformTools(client, pm)
			runBabeSetup(pm, false, true)
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"check"},
		Short:   "Check installation status and PATH availability",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isJSON(cmd) {
				return handleToolsStatusJSON(cmd, client, pm)
			}
			checkPlatformTools(client, pm)
			runBabeSetup(pm, false, true)
			return nil
		},
	}

	updateCmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{"install", "download"},
		Short:   "Download or update platform-tools to latest Google release",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isJSON(cmd) {
				return handleUpdateToolsJSON(cmd, client, false, false)
			}
			downloadOrUpdatePlatformTools(client, pm, false, false)
			return nil
		},
	}

	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure babe binary and platform-tools in user PATH",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isJSON(cmd) {
				return handleSetupJSON(cmd, pm, false, false)
			}
			runBabeSetup(pm, false, false)
			return nil
		},
	}

	toolsCmd.AddCommand(statusCmd, updateCmd, setupCmd)
	return toolsCmd
}
