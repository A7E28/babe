package main

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/platform"
)

func newUpdateCmd(client *device.Client, pm platform.PlatformManager) *cobra.Command {
	var force bool
	var check bool
	var toolsFlag bool
	var selfFlag bool

	updateCmd := &cobra.Command{
		Use:     "update [tools|self]",
		Short:   "Update Android SDK platform-tools or the babe executable",
		Long:    "Update Android SDK platform-tools or the babe executable.",
		GroupID: "core",
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if toolsFlag {
				target = "tools"
			} else if selfFlag {
				target = "self"
			} else if len(args) > 0 {
				target = strings.ToLower(strings.TrimLeft(args[0], "-"))
			}

			switch target {
			case "tools", "platform-tools", "adb", "fastboot":
				if isJSON(cmd) {
					return handleUpdateToolsJSON(cmd, client, force, check)
				}
				downloadOrUpdatePlatformTools(client, pm, force, check)
			case "self", "babe":
				if isJSON(cmd) {
					return handleSelfUpdateJSON(cmd, force, check)
				}
				runSelfUpdate(force, check)
			default:
				_ = cmd.Help()
			}
			return nil
		},
	}

	updateCmd.PersistentFlags().BoolVarP(&force, "force", "f", false, "Force download/reinstall even if already up to date")
	updateCmd.PersistentFlags().BoolVarP(&check, "check", "c", false, "Check for updates without downloading")
	updateCmd.Flags().BoolVar(&toolsFlag, "tools", false, "Target platform-tools")
	updateCmd.Flags().BoolVar(&selfFlag, "self", false, "Target babe self executable")

	toolsSubCmd := &cobra.Command{
		Use:     "tools",
		Aliases: []string{"platform-tools", "adb", "fastboot"},
		Short:   "Install or update Google Android platform-tools (adb/fastboot)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if isJSON(cmd) {
				return handleUpdateToolsJSON(cmd, client, force, check)
			}
			downloadOrUpdatePlatformTools(client, pm, force, check)
			return nil
		},
	}

	selfSubCmd := &cobra.Command{
		Use:     "self",
		Aliases: []string{"babe"},
		Short:   "Check and update babe executable to latest GitHub release",
		RunE: func(cmd *cobra.Command, args []string) error {
			if isJSON(cmd) {
				return handleSelfUpdateJSON(cmd, force, check)
			}
			runSelfUpdate(force, check)
			return nil
		},
	}

	updateCmd.AddCommand(toolsSubCmd, selfSubCmd)
	return updateCmd
}
