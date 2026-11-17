package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/apk"
	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/ui"
)

func newAPKCmd(client *device.Client) *cobra.Command {
	var serial string

	cmd := &cobra.Command{
		Use:     "apk",
		Short:   "Inspect, install, list, and extract Android packages",
		Long:    "Inspect, install, list, and extract Android packages.",
		GroupID: "pkg",
	}

	cmd.PersistentFlags().StringVarP(&serial, "serial", "s", "", "Target specific device serial (default: auto-detected)")

	var showAll bool
	inspectCmd := &cobra.Command{
		Use:     "inspect <file.apk|.apks|.xapk|.apkm>",
		Aliases: []string{"info"},
		Short:   "Offline manifest, SDK, ABI & certificate analysis",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := apk.InspectArchive(args[0])
			if err != nil {
				return err
			}
			if isJSON(cmd) {
				return writeJSON(cmd.OutOrStdout(), info)
			}
			ui.RenderArchiveInfo(info, showAll)
			return nil
		},
	}
	inspectCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Display all declared permissions")

	installCmd := &cobra.Command{
		Use:   "install <file.apk|.apks|.xapk|.apkm>",
		Short: "Install standard APK or split APK bundle onto a connected device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleInstallCLI(cmd, client, args[0], serial)
		},
	}

	var thirdParty bool
	var filter string
	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List installed packages on connected device",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return handlePackagesCLI(cmd, client, serial, thirdParty, filter)
		},
	}
	listCmd.Flags().BoolVarP(&thirdParty, "third-party", "3", false, "List third-party (user-installed) packages only")
	listCmd.Flags().StringVarP(&filter, "filter", "f", "", "Filter package names matching substring")

	var outDir string
	pullCmd := &cobra.Command{
		Use:   "pull <package-name> [destination]",
		Short: "Extract and pull installed APK files from device to host",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			destination := outDir
			if len(args) > 1 && !cmd.Flags().Changed("output") {
				destination = args[1]
			}
			return handlePullAPKCLI(cmd, client, args[0], destination, serial)
		},
	}
	pullCmd.Flags().StringVarP(&outDir, "output", "o", ".", "Destination directory")

	cmd.AddCommand(inspectCmd, installCmd, listCmd, pullCmd)
	return cmd
}

func newPackagesAliasCmd(client *device.Client) *cobra.Command {
	var serial string
	var thirdParty bool
	var filter string

	cmd := &cobra.Command{
		Use:     "packages",
		Aliases: []string{"pkgs"},
		Hidden:  true,
		Short:   "List installed packages on connected device",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return handlePackagesCLI(cmd, client, serial, thirdParty, filter)
		},
	}
	cmd.Flags().StringVarP(&serial, "serial", "s", "", "Target specific device serial")
	cmd.Flags().BoolVarP(&thirdParty, "third-party", "3", false, "List third-party packages only")
	cmd.Flags().StringVarP(&filter, "filter", "f", "", "Filter package names")
	return cmd
}

func newPullAPKAliasCmd(client *device.Client) *cobra.Command {
	var serial string
	var outDir string

	cmd := &cobra.Command{
		Use:    "pull-apk <package-name> [destination]",
		Hidden: true,
		Short:  "Extract installed package APK from device",
		Args:   cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			destination := outDir
			if len(args) > 1 && !cmd.Flags().Changed("output") {
				destination = args[1]
			}
			return handlePullAPKCLI(cmd, client, args[0], destination, serial)
		},
	}
	cmd.Flags().StringVarP(&serial, "serial", "s", "", "Target specific device serial")
	cmd.Flags().StringVarP(&outDir, "output", "o", ".", "Destination directory")
	return cmd
}

func handleInstallCLI(cmd *cobra.Command, client *device.Client, archivePath, serial string) error {
	if _, err := os.Stat(archivePath); err != nil {
		return fmt.Errorf("package file not found: %s", archivePath)
	}

	targetSerial, err := resolveTargetDevice(cmd, client, serial)
	if err != nil {
		return err
	}

	jsonMode := isJSON(cmd)
	if !jsonMode {
		fmt.Printf("Installing %s on %s...\n", archivePath, targetSerial)
	}
	opts := apk.InstallOptions{Reinstall: true, Downgrade: true, GrantPerm: true}
	if err := apk.InstallAuto(context.Background(), client, targetSerial, archivePath, opts); err != nil {
		return err
	}

	if jsonMode {
		return writeJSON(cmd.OutOrStdout(), apkInstallReport{Serial: targetSerial, File: archivePath, Installed: true})
	}
	ui.Success("Package installed successfully.")
	return nil
}

func handlePackagesCLI(cmd *cobra.Command, client *device.Client, serial string, thirdPartyOnly bool, filter string) error {
	targetSerial, err := resolveTargetDevice(cmd, client, serial)
	if err != nil {
		return err
	}

	pkgs, err := apk.ListPackages(context.Background(), client, targetSerial, thirdPartyOnly)
	if err != nil {
		return err
	}

	lowerFilter := strings.ToLower(strings.TrimSpace(filter))
	matched := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		if lowerFilter == "" || strings.Contains(strings.ToLower(p), lowerFilter) {
			matched = append(matched, p)
		}
	}

	if isJSON(cmd) {
		return writeJSON(cmd.OutOrStdout(), packageListReport{
			Serial:   targetSerial,
			Count:    len(matched),
			Packages: matched,
		})
	}
	for _, p := range matched {
		fmt.Println(p)
	}
	return nil
}

func handlePullAPKCLI(cmd *cobra.Command, client *device.Client, pkgName, outDir, serial string) error {
	targetSerial, err := resolveTargetDevice(cmd, client, serial)
	if err != nil {
		return err
	}

	savedPath, err := apk.PullPackage(context.Background(), client, targetSerial, pkgName, outDir)
	if err != nil {
		return err
	}
	if isJSON(cmd) {
		return writeJSON(cmd.OutOrStdout(), apkPullReport{
			Serial:     targetSerial,
			Package:    pkgName,
			OutputPath: savedPath,
		})
	}
	ui.Success("APK extracted to: %s", savedPath)
	return nil
}
