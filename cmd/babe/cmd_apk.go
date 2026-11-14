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
		Run: func(cmd *cobra.Command, args []string) {
			info, err := apk.InspectArchive(args[0])
			if err != nil {
				ui.Error("%v", err)
				os.Exit(1)
			}
			ui.RenderArchiveInfo(info, showAll)
		},
	}
	inspectCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Display all declared permissions")

	installCmd := &cobra.Command{
		Use:   "install <file.apk|.apks|.xapk|.apkm>",
		Short: "Install standard APK or split APK bundle onto a connected device",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			handleInstallCLI(client, args[0], serial)
		},
	}

	var thirdParty bool
	var filter string
	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List installed packages on connected device",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			handlePackagesCLI(client, serial, thirdParty, filter)
		},
	}
	listCmd.Flags().BoolVarP(&thirdParty, "third-party", "3", false, "List third-party (user-installed) packages only")
	listCmd.Flags().StringVarP(&filter, "filter", "f", "", "Filter package names matching substring")

	var outDir string
	pullCmd := &cobra.Command{
		Use:   "pull <package-name> [destination]",
		Short: "Extract and pull installed APK files from device to host",
		Args:  cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			destination := outDir
			if len(args) > 1 && !cmd.Flags().Changed("output") {
				destination = args[1]
			}
			handlePullAPKCLI(client, args[0], destination, serial)
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
		Run: func(cmd *cobra.Command, args []string) {
			handlePackagesCLI(client, serial, thirdParty, filter)
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
		Run: func(cmd *cobra.Command, args []string) {
			destination := outDir
			if len(args) > 1 && !cmd.Flags().Changed("output") {
				destination = args[1]
			}
			handlePullAPKCLI(client, args[0], destination, serial)
		},
	}
	cmd.Flags().StringVarP(&serial, "serial", "s", "", "Target specific device serial")
	cmd.Flags().StringVarP(&outDir, "output", "o", ".", "Destination directory")
	return cmd
}

func handleInstallCLI(client *device.Client, archivePath, serial string) {
	if _, err := os.Stat(archivePath); err != nil {
		ui.Error("Package file not found: %s", archivePath)
		os.Exit(1)
	}

	targetSerial, err := resolveTargetDevice(client, serial)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	fmt.Printf("Installing %s on %s...\n", archivePath, targetSerial)
	opts := apk.InstallOptions{
		Reinstall: true,
		Downgrade: true,
		GrantPerm: true,
	}

	if err := apk.InstallAuto(context.Background(), client, targetSerial, archivePath, opts); err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	ui.Success("Package installed successfully.")
}

func handlePackagesCLI(client *device.Client, serial string, thirdPartyOnly bool, filter string) {
	targetSerial, err := resolveTargetDevice(client, serial)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	pkgs, err := apk.ListPackages(context.Background(), client, targetSerial, thirdPartyOnly)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	lowerFilter := strings.ToLower(strings.TrimSpace(filter))
	for _, p := range pkgs {
		if lowerFilter == "" || strings.Contains(strings.ToLower(p), lowerFilter) {
			fmt.Println(p)
		}
	}
}

func handlePullAPKCLI(client *device.Client, pkgName, outDir, serial string) {
	targetSerial, err := resolveTargetDevice(client, serial)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	savedPath, err := apk.PullPackage(context.Background(), client, targetSerial, pkgName, outDir)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}
	ui.Success("APK extracted to: %s", savedPath)
}
