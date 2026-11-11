package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/A7E28/babe/internal/apk"
	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/ui"
)

func handleAPKDomainCLI(client *device.Client, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: babe apk <inspect|install|list|pull> [args]")
		os.Exit(1)
	}
	sub := strings.ToLower(args[0])
	rest := args[1:]

	switch sub {
	case "inspect", "info":
		fs := flag.NewFlagSet("apk inspect", flag.ExitOnError)
		showAll := fs.Bool("a", false, "Show all declared permissions")
		fs.BoolVar(showAll, "all", false, "Show all declared permissions")
		_ = fs.Parse(reorderFlags(rest, map[string]bool{"-a": true, "--a": true, "-all": true, "--all": true}))
		if fs.NArg() < 1 {
			fmt.Println("Usage: babe apk inspect <file.apk|.apks|.xapk> [-a|--all]")
			os.Exit(1)
		}
		info, err := apk.InspectArchive(fs.Arg(0))
		if err != nil {
			ui.Error("%v", err)
			os.Exit(1)
		}
		ui.RenderArchiveInfo(info, *showAll)
	case "install":
		fs := flag.NewFlagSet("apk install", flag.ExitOnError)
		serial := fs.String("s", "", "Device serial")
		fs.StringVar(serial, "serial", "", "Device serial")
		_ = fs.Parse(reorderFlags(rest, nil))
		if fs.NArg() < 1 {
			fmt.Println("Usage: babe apk install <file.apk|.apks|.xapk> [-s <serial>]")
			os.Exit(1)
		}
		handleInstallCLI(client, fs.Arg(0), *serial)
	case "list", "ls":
		fs := flag.NewFlagSet("apk list", flag.ExitOnError)
		serial := fs.String("s", "", "Device serial")
		fs.StringVar(serial, "serial", "", "Device serial")
		thirdParty := fs.Bool("3", false, "Third-party packages only")
		filter := fs.String("f", "", "Filter substring")
		fs.StringVar(filter, "filter", "", "Filter substring")
		_ = fs.Parse(reorderFlags(rest, map[string]bool{"-3": true, "--3": true}))
		handlePackagesCLI(client, *serial, *thirdParty, *filter)
	case "pull":
		fs := flag.NewFlagSet("apk pull", flag.ExitOnError)
		serial := fs.String("s", "", "Device serial")
		fs.StringVar(serial, "serial", "", "Device serial")
		outDir := fs.String("o", ".", "Destination directory")
		fs.StringVar(outDir, "output", ".", "Destination directory")
		_ = fs.Parse(reorderFlags(rest, nil))
		if fs.NArg() < 1 {
			fmt.Println("Usage: babe apk pull <package-name> [-o <dir>] [-s <serial>]")
			os.Exit(1)
		}
		handlePullAPKCLI(client, fs.Arg(0), *outDir, *serial)
	default:
		ui.Error("Unknown apk subcommand %q (valid: inspect, install, list, pull)", sub)
		os.Exit(1)
	}
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
