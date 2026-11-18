package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/ui"
)

func newRootCmd(adbClient *device.Client, fbClient *fastboot.Client, pm platform.PlatformManager) *cobra.Command {
	var jsonOut bool
	var showVersion bool

	rootCmd := &cobra.Command{
		Use:   "babe",
		Short: "Android Systems, Firmware, and Diagnostics Toolkit",
		Long: fmt.Sprintf("%s %s\nAndroid Systems, Firmware, and Diagnostics Toolkit for power users.",
			ui.BoldCyan("babe"), ui.Gray(version)),
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				if isJSON(cmd) {
					return writeJSON(cmd.OutOrStdout(), versionReport{Version: version})
				}
				cmd.Printf("babe %s\n", version)
				return nil
			}
			if isJSON(cmd) {
				return fmt.Errorf("subcommand required")
			}
			return cmd.Help()
		},
	}

	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output results as machine-readable JSON")
	rootCmd.Flags().BoolVar(&showVersion, "version", false, "Print babe version")

	rootCmd.AddGroup(
		&cobra.Group{ID: "core", Title: "Core Commands:"},
		&cobra.Group{ID: "device", Title: "Device & Fastboot Commands:"},
		&cobra.Group{ID: "pkg", Title: "Package, Image & Diagnostic Commands:"},
	)

	rootCmd.AddCommand(newDoctorCmd(adbClient, fbClient, pm))
	rootCmd.AddCommand(newToolsCmd(adbClient, pm))
	rootCmd.AddCommand(newUpdateCmd(adbClient, pm))
	rootCmd.AddCommand(newSetupCmd(pm))

	rootCmd.AddCommand(newDeviceCmd(adbClient))
	rootCmd.AddCommand(newFastbootCmd(fbClient))

	rootCmd.AddCommand(newAPKCmd(adbClient))
	rootCmd.AddCommand(newImageCmd())
	rootCmd.AddCommand(newLogcatCmd(adbClient))
	rootCmd.AddCommand(newHashCmd())
	rootCmd.AddCommand(newUdevCmd())

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print babe version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isJSON(cmd) {
				return writeJSON(cmd.OutOrStdout(), versionReport{Version: version})
			}
			cmd.Printf("babe %s\n", version)
			return nil
		},
	})

	rootCmd.AddCommand(newDevicesAliasCmd(adbClient))
	rootCmd.AddCommand(newInfoAliasCmd(adbClient))
	rootCmd.AddCommand(newRebootAliasCmd(adbClient))
	rootCmd.AddCommand(newPackagesAliasCmd(adbClient))
	rootCmd.AddCommand(newPullAPKAliasCmd(adbClient))
	rootCmd.AddCommand(newInstallShortcutCmd(adbClient, pm))

	return rootCmd
}

func executeRootCmd(root *cobra.Command, args []string) error {
	root.SetArgs(args)
	executedCmd, err := root.ExecuteC()
	if err != nil {
		errCmd := executedCmd
		if errCmd == nil {
			errCmd = root
		}
		jsonMode := hasJSONFlag(args)
		if executedCmd != nil && executedCmd.Flags().Changed("json") {
			jsonMode = isJSON(executedCmd)
		}
		if jsonMode {
			_ = writeJSON(errCmd.ErrOrStderr(), errorReport{Error: err.Error()})
		} else {
			ui.ErrorTo(errCmd.ErrOrStderr(), "%v", err)
		}
		return err
	}
	return nil
}

func executeCLI(args []string) error {
	adbClient := device.NewClient()
	fbClient := fastboot.NewClient()
	pm := platform.DefaultManager()

	return executeRootCmd(newRootCmd(adbClient, fbClient, pm), args)
}
