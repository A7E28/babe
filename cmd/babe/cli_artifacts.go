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

// moves flag tokens before positional arguments so flags work anywhere on the command line.
func reorderFlags(args []string, boolFlags map[string]bool) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			if !strings.Contains(arg, "=") && !boolFlags[arg] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			pos = append(pos, arg)
		}
	}
	return append(flags, pos...)
}

func handleToolsCLI(client *device.Client, pm platform.PlatformManager, args []string) {
	sub := "status"
	if len(args) > 0 {
		sub = strings.ToLower(args[0])
	}
	switch sub {
	case "status", "check":
		checkPlatformTools(client, pm)
	case "update", "install", "download":
		downloadOrUpdatePlatformTools(client, pm)
	default:
		ui.Error("Unknown tools subcommand %q (valid: status, update)", sub)
		os.Exit(1)
	}
}

func handleUpdateCLI(client *device.Client, pm platform.PlatformManager, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: babe update <tools|self> [-f|--force] [-c|--check]")
		fmt.Println("\nTargets:")
		fmt.Println("  tools, platform-tools   Install or update Google Android Platform-Tools (adb/fastboot)")
		fmt.Println("  self, babe              Check and update babe executable to latest GitHub release")
		fmt.Println("\nFlags:")
		fmt.Println("  -f, --force             Force download/reinstall even if already up to date")
		fmt.Println("  -c, --check             Check for updates without downloading")
		os.Exit(1)
	}

	fs := flag.NewFlagSet("update", flag.ExitOnError)
	force := fs.Bool("f", false, "Force reinstall even if up to date")
	fs.BoolVar(force, "force", false, "Force reinstall even if up to date")
	check := fs.Bool("c", false, "Check for updates only")
	fs.BoolVar(check, "check", false, "Check for updates only")
	toolsFlag := fs.Bool("tools", false, "Update platform-tools")
	selfFlag := fs.Bool("self", false, "Update babe executable")

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
		downloadOrUpdatePlatformTools(client, pm)
	case "self", "babe":
		runSelfUpdate(*force, *check)
	default:
		ui.Error("Unknown update target %q (valid: tools, self)", target)
		os.Exit(1)
	}
}

func handleFastbootDomainCLI(fbClient *fastboot.Client, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: babe fastboot <devices|info|slot|flash> [args] [-s <serial>]")
		os.Exit(1)
	}
	sub := strings.ToLower(args[0])
	rest := args[1:]

	fs := flag.NewFlagSet("fastboot", flag.ExitOnError)
	serial := fs.String("s", "", "Fastboot device serial")
	fs.StringVar(serial, "serial", "", "Fastboot device serial")
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
		if fs.NArg() < 1 {
			fmt.Println("Usage: babe fastboot slot <a|b> [-s <serial>]")
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
		if fs.NArg() < 2 {
			fmt.Println("Usage: babe fastboot flash <partition> <image.img> [-s <serial>]")
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
		ui.Error("Unknown fastboot subcommand %q (valid: devices, info, slot, flash)", sub)
		os.Exit(1)
	}
}

func handleImageDomainCLI(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: babe image inspect <boot.img|init_boot.img>")
		os.Exit(1)
	}
	sub := strings.ToLower(args[0])
	if sub == "inspect" || sub == "info" {
		if len(args) < 2 {
			fmt.Println("Usage: babe image inspect <boot.img|init_boot.img>")
			os.Exit(1)
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
