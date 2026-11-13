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
		Run: func(cmd *cobra.Command, args []string) {
			checkPlatformTools(client, pm)
			runBabeSetup(pm, false, true)
		},
	}

	statusCmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"check"},
		Short:   "Check installation status and PATH availability",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			checkPlatformTools(client, pm)
			runBabeSetup(pm, false, true)
		},
	}

	updateCmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{"install", "download"},
		Short:   "Download or update platform-tools to latest Google release",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			downloadOrUpdatePlatformTools(client, pm, false, false)
		},
	}

	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure babe binary and platform-tools in user PATH",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			runBabeSetup(pm, false, false)
		},
	}

	toolsCmd.AddCommand(statusCmd, updateCmd, setupCmd)
	return toolsCmd
}
