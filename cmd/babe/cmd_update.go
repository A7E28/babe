package main

import (
	"fmt"
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
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if (toolsFlag || selfFlag) && len(args) > 0 {
				return fmt.Errorf("cannot combine --tools or --self with positional target %q", args[0])
			}
			if !toolsFlag && !selfFlag && len(args) == 0 {
				if isJSON(cmd) {
					return fmt.Errorf("update target required (valid: tools, self)")
				}
				return cmd.Help()
			}

			target := ""
			if toolsFlag {
				target = "tools"
			} else if selfFlag {
				target = "self"
			} else {
				target = strings.ToLower(strings.TrimSpace(args[0]))
			}

			switch target {
			case "tools", "platform-tools", "adb", "fastboot":
				if isJSON(cmd) {
					return handleUpdateToolsJSON(cmd, client, force, check)
				}
				return downloadOrUpdatePlatformTools(cmd.OutOrStdout(), client, pm, force, check)
			case "self", "babe":
				if isJSON(cmd) {
					return handleSelfUpdateJSON(cmd, force, check)
				}
				return runSelfUpdate(cmd.OutOrStdout(), force, check)
			default:
				return fmt.Errorf("unknown update target %q (valid: tools, self)", args[0])
			}
		},
	}

	updateCmd.PersistentFlags().BoolVarP(&force, "force", "f", false, "Force download/reinstall even if already up to date")
	updateCmd.PersistentFlags().BoolVarP(&check, "check", "c", false, "Check for updates without downloading")
	updateCmd.Flags().BoolVar(&toolsFlag, "tools", false, "Target platform-tools")
	updateCmd.Flags().BoolVar(&selfFlag, "self", false, "Target babe self executable")
	updateCmd.MarkFlagsMutuallyExclusive("tools", "self")

	toolsSubCmd := &cobra.Command{
		Use:     "tools",
		Aliases: []string{"platform-tools", "adb", "fastboot"},
		Short:   "Install or update Google Android platform-tools (adb/fastboot)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isJSON(cmd) {
				return handleUpdateToolsJSON(cmd, client, force, check)
			}
			return downloadOrUpdatePlatformTools(cmd.OutOrStdout(), client, pm, force, check)
		},
	}

	selfSubCmd := &cobra.Command{
		Use:     "self",
		Aliases: []string{"babe"},
		Short:   "Check and update babe executable to latest GitHub release",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isJSON(cmd) {
				return handleSelfUpdateJSON(cmd, force, check)
			}
			return runSelfUpdate(cmd.OutOrStdout(), force, check)
		},
	}

	updateCmd.AddCommand(toolsSubCmd, selfSubCmd)
	return updateCmd
}
