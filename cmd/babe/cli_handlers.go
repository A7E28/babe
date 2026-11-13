package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/doctor"
	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/ui"
)

func resolveTargetDevice(client *device.Client, explicitSerial string) (string, error) {
	devices, err := client.GetConnectedDevices(context.Background())
	if err != nil {
		return "", fmt.Errorf("list devices: %w", err)
	}

	if explicitSerial != "" {
		for _, d := range devices {
			if d.Serial == explicitSerial {
				if err := device.ValidateDeviceState(d); err != nil {
					return "", err
				}
				return d.Serial, nil
			}
		}
		return "", fmt.Errorf("device %q: %w", explicitSerial, device.ErrDeviceNotFound)
	}

	if len(devices) == 0 {
		return "", device.ErrNoDevicesFound
	}
	if len(devices) == 1 {
		if err := device.ValidateDeviceState(devices[0]); err != nil {
			return "", err
		}
		return devices[0].Serial, nil
	}

	ui.RenderDeviceList(devices)
	return "", fmt.Errorf("multiple devices detected; specify target with -s <serial>")
}

func resolveFastbootDevice(fbClient *fastboot.Client, explicitSerial string) (string, error) {
	devs, err := fbClient.GetConnectedDevices(context.Background())
	if err != nil {
		return "", fmt.Errorf("list fastboot devices: %w", err)
	}
	if explicitSerial != "" {
		for _, d := range devs {
			if d.Serial == explicitSerial {
				return d.Serial, nil
			}
		}
		return "", fmt.Errorf("fastboot device %q: %w", explicitSerial, fastboot.ErrFastbootNotFound)
	}
	if len(devs) == 0 {
		return "", fastboot.ErrNoFastbootDevices
	}
	if len(devs) == 1 {
		return devs[0].Serial, nil
	}
	ui.RenderFastbootDevices(devs)
	return "", fmt.Errorf("multiple fastboot devices detected; specify target with -s <serial>")
}

func handleDoctorCLI(adbClient *device.Client, fbClient *fastboot.Client, pm platform.PlatformManager, fixUdev bool) {
	ui.Section("System & USB Doctor")
	results := doctor.RunDiagnostics(context.Background(), adbClient, fbClient, pm)
	ui.RenderDoctorResults(results)

	if fixUdev {
		fmt.Println()
		handleUdevCLI()
	}
}

func handleDeviceListCLI(client *device.Client) {
	devices, err := client.GetConnectedDevices(context.Background())
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}
	if len(devices) == 0 {
		ui.Warn("No ADB devices attached.")
		return
	}
	ui.RenderDeviceList(devices)
}

func handleDeviceDomainCLI(client *device.Client, args []string) {
	if len(args) > 0 && isHelpArg(args[0]) {
		printDeviceHelp()
		return
	}
	sub := "list"
	var rest []string
	if len(args) > 0 {
		sub = strings.ToLower(args[0])
		rest = args[1:]
	}

	switch sub {
	case "list", "ls":
		handleDeviceListCLI(client)
	case "info":
		if len(rest) > 0 && isHelpArg(rest[0]) {
			printDeviceInfoHelp()
			return
		}
		fs := flag.NewFlagSet("device info", flag.ExitOnError)
		fs.Usage = printDeviceInfoHelp
		serial := fs.String("s", "", "Device serial")
		fs.StringVar(serial, "serial", "", "Device serial")
		_ = fs.Parse(reorderFlags(rest, nil))
		handleDeviceInfoCLI(client, *serial)
	case "reboot":
		if len(rest) > 0 && isHelpArg(rest[0]) {
			printDeviceRebootHelp()
			return
		}
		fs := flag.NewFlagSet("device reboot", flag.ExitOnError)
		fs.Usage = printDeviceRebootHelp
		serial := fs.String("s", "", "Device serial")
		fs.StringVar(serial, "serial", "", "Device serial")
		_ = fs.Parse(reorderFlags(rest, nil))
		target := ""
		if fs.NArg() > 0 {
			target = strings.ToLower(fs.Arg(0))
		}
		handleRebootCLI(client, target, *serial)
	default:
		ui.Error("Unknown device subcommand %q. Run 'babe device --help' for usage.", sub)
		os.Exit(1)
	}
}

func handleDeviceInfoCLI(client *device.Client, serial string) {
	targetSerial, err := resolveTargetDevice(client, serial)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}
	info, err := client.GetDeviceInfo(context.Background(), targetSerial)
	if err != nil {
		ui.Error("Query device info: %v", err)
		os.Exit(1)
	}
	ui.RenderDeviceInfo(info)
}

func handleRebootCLI(client *device.Client, targetStr, serial string) {
	targetSerial, err := resolveTargetDevice(client, serial)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	var target device.RebootTarget
	switch targetStr {
	case "", "system", "normal":
		target = device.RebootSystem
	case "bootloader", "fastboot":
		target = device.RebootBootloader
	case "recovery":
		target = device.RebootRecovery
	case "edl":
		target = device.RebootEDL
	default:
		ui.Error("Unknown reboot target %q (valid: system, bootloader, recovery, edl)", targetStr)
		os.Exit(1)
	}

	if err := client.Reboot(context.Background(), targetSerial, target); err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}
	ui.Success("Reboot command sent to device %s.", targetSerial)
}

func handleUdevCLI() {
	if err := platform.InstallUdevRules(); err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}
	ui.Success("Android udev rules installed and reloaded.")
}
