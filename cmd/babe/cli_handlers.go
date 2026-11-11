package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/doctor"
	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/logcat"
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
		return explicitSerial, nil
	}

	if len(devices) == 0 {
		return "", fmt.Errorf("no connected ADB devices detected")
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
	if explicitSerial != "" {
		return explicitSerial, nil
	}
	devs, err := fbClient.GetConnectedDevices(context.Background())
	if err != nil {
		return "", fmt.Errorf("list fastboot devices: %w", err)
	}
	if len(devs) == 0 {
		return "", fmt.Errorf("no devices detected in Fastboot mode")
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
		fs := flag.NewFlagSet("device info", flag.ExitOnError)
		serial := fs.String("s", "", "Device serial")
		fs.StringVar(serial, "serial", "", "Device serial")
		_ = fs.Parse(reorderFlags(rest, nil))
		handleDeviceInfoCLI(client, *serial)
	case "reboot":
		fs := flag.NewFlagSet("device reboot", flag.ExitOnError)
		serial := fs.String("s", "", "Device serial")
		fs.StringVar(serial, "serial", "", "Device serial")
		_ = fs.Parse(reorderFlags(rest, nil))
		target := ""
		if fs.NArg() > 0 {
			target = strings.ToLower(fs.Arg(0))
		}
		handleRebootCLI(client, target, *serial)
	default:
		ui.Error("Unknown device subcommand %q (valid: list, info, reboot)", sub)
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

func handleLogcatCLI(client *device.Client, opts logcat.StreamOptions) {
	targetSerial, err := resolveTargetDevice(client, opts.Serial)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}
	opts.Serial = targetSerial
	opts.ADBBinary = client.BinaryPath()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err = logcat.Stream(ctx, opts, func(entry logcat.Entry) {
		fmt.Println(logcat.Colorize(entry))
	})
	if err != nil && ctx.Err() == nil {
		ui.Error("Logcat stream error: %v", err)
		os.Exit(1)
	}
}

func handleCrashLogCLI(client *device.Client, serial, outFile string) {
	targetSerial, err := resolveTargetDevice(client, serial)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	count, err := logcat.DumpCrashLogs(context.Background(), client.BinaryPath(), targetSerial, outFile, func(entry logcat.Entry) {
		fmt.Println(logcat.Colorize(entry))
	})
	if err != nil {
		ui.Error("Crash log harvest failed: %v", err)
		os.Exit(1)
	}
	if count == 0 {
		ui.Info("No crash entries found in device crash buffer.")
		return
	}
	ui.Info("Harvested %d crash log line(s) from device %s.", count, targetSerial)
	if outFile != "" {
		ui.Success("Saved crash report to %s", outFile)
	}
}

func handleUdevCLI() {
	if err := platform.InstallUdevRules(); err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}
	ui.Success("Android udev rules installed and reloaded.")
}
