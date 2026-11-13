package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/image"
	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/ui"
)

func handleToolsCLI(client *device.Client, pm platform.PlatformManager, args []string) {
	if len(args) > 0 && isHelpArg(args[0]) {
		printToolsHelp()
		return
	}
	sub := "status"
	if len(args) > 0 {
		sub = strings.ToLower(args[0])
	}
	switch sub {
	case "status", "check":
		checkPlatformTools(client, pm)
		runBabeSetup(pm, false, true)
	case "update", "install", "download":
		downloadOrUpdatePlatformTools(client, pm, false, false)
	case "setup":
		runBabeSetup(pm, false, false)
	default:
		ui.Error("Unknown tools subcommand %q. Run 'babe tools --help' for usage.", sub)
		os.Exit(1)
	}
}

func handleUpdateCLI(client *device.Client, pm platform.PlatformManager, args []string) {
	if len(args) == 0 {
		printUpdateHelp()
		os.Exit(1)
	}
	if isHelpArg(args[0]) {
		printUpdateHelp()
		return
	}
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	fs.Usage = printUpdateHelp
	force := fs.Bool("f", false, "")
	fs.BoolVar(force, "force", false, "")
	check := fs.Bool("c", false, "")
	fs.BoolVar(check, "check", false, "")
	toolsFlag := fs.Bool("tools", false, "")
	selfFlag := fs.Bool("self", false, "")
	boolFlags := map[string]bool{"-f": true, "--force": true, "-c": true, "--check": true, "--tools": true, "--self": true}
	_ = fs.Parse(reorderFlags(args, boolFlags))

	target := ""
	if *toolsFlag {
		target = "tools"
	} else if *selfFlag {
		target = "self"
	} else if fs.NArg() > 0 {
		target = strings.ToLower(strings.TrimLeft(fs.Arg(0), "-"))
	}

	switch target {
	case "tools", "platform-tools", "adb", "fastboot":
		downloadOrUpdatePlatformTools(client, pm, *force, *check)
	case "self", "babe":
		runSelfUpdate(*force, *check)
	default:
		ui.Error("Unknown update target %q. Run 'babe update --help' for usage.", target)
		os.Exit(1)
	}
}

func handleFastbootDomainCLI(fbClient *fastboot.Client, args []string) {
	if len(args) == 0 {
		printFastbootHelp()
		os.Exit(1)
	}
	if isHelpArg(args[0]) {
		printFastbootHelp()
		return
	}
	sub := strings.ToLower(args[0])
	rest := args[1:]

	fs := flag.NewFlagSet("fastboot", flag.ExitOnError)
	fs.Usage = printFastbootHelp
	serial := fs.String("s", "", "")
	fs.StringVar(serial, "serial", "", "")
	_ = fs.Parse(reorderFlags(rest, nil))

	switch sub {
	case "devices", "list":
		devs, err := fbClient.GetConnectedDevices(context.Background())
		if err != nil {
			ui.Error("%v", err)
			os.Exit(1)
		}
		if len(devs) == 0 {
			ui.Warn("No devices attached in Fastboot mode.")
			return
		}
		ui.RenderFastbootDevices(devs)
	case "info", "getvar":
		target, err := resolveFastbootDevice(fbClient, *serial)
		if err != nil {
			ui.Error("%v", err)
			os.Exit(1)
		}
		info, err := fbClient.GetDeviceInfo(context.Background(), target)
		if err != nil {
			ui.Error("%v", err)
			os.Exit(1)
		}
		ui.RenderFastbootInfo(info)
	case "slot":
		if len(rest) > 0 && isHelpArg(rest[0]) {
			printFastbootSlotHelp()
			return
		}
		if fs.NArg() < 1 {
			printFastbootSlotHelp()
			os.Exit(1)
		}
		target, err := resolveFastbootDevice(fbClient, *serial)
		if err != nil {
			ui.Error("%v", err)
			os.Exit(1)
		}
		if err := fbClient.SetActiveSlot(context.Background(), target, fs.Arg(0)); err != nil {
			ui.Error("%v", err)
			os.Exit(1)
		}
		ui.Success("Active slot set to %s on %s.", strings.ToUpper(fs.Arg(0)), target)
	case "flash":
		if len(rest) > 0 && isHelpArg(rest[0]) {
			printFastbootFlashHelp()
			return
		}
		if fs.NArg() < 2 {
			printFastbootFlashHelp()
			os.Exit(1)
		}
		target, err := resolveFastbootDevice(fbClient, *serial)
		if err != nil {
			ui.Error("%v", err)
			os.Exit(1)
		}
		partition, imgPath := fs.Arg(0), fs.Arg(1)
		fmt.Printf("Verifying SHA256 and flashing %s -> %s on %s...\n", imgPath, partition, target)
		digest, err := fbClient.FlashPartition(context.Background(), target, partition, imgPath)
		if digest != "" {
			ui.Info("Image SHA256: %s", digest)
		}
		if err != nil {
			ui.Error("%v", err)
			os.Exit(1)
		}
		ui.Success("Partition %q flashed successfully.", partition)
	default:
		ui.Error("Unknown fastboot subcommand %q. Run 'babe fastboot --help' for usage.", sub)
		os.Exit(1)
	}
}

func handleImageDomainCLI(args []string) {
	if len(args) == 0 {
		printImageHelp()
		os.Exit(1)
	}
	if isHelpArg(args[0]) {
		printImageHelp()
		return
	}
	sub := strings.ToLower(args[0])
	if sub == "inspect" || sub == "info" {
		if len(args) < 2 || isHelpArg(args[1]) {
			printImageHelp()
			if len(args) < 2 {
				os.Exit(1)
			}
			return
		}
		sub = args[1]
	} else {
		sub = args[0]
	}
	info, err := image.InspectBootImage(sub)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}
	ui.RenderBootImageInfo(info)
}
