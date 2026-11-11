package main

import (
	"bufio"
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/doctor"
	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/tools"
	"github.com/A7E28/babe/internal/ui"
)

func runDoctorMenu(adbClient *device.Client, fbClient *fastboot.Client, pm platform.PlatformManager, reader *bufio.Reader) {
	ui.Section("System & USB Doctor")
	results := doctor.RunDiagnostics(context.Background(), adbClient, fbClient, pm)
	ui.RenderDoctorResults(results)

	if runtime.GOOS == "linux" {
		ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Install/reload Linux Android USB udev rules now? [y/N]:")))
		ans, _ := reader.ReadString('\n')
		if strings.EqualFold(strings.TrimSpace(ans), "y") {
			if err := platform.InstallUdevRules(); err != nil {
				ui.Error("%v", err)
			} else {
				ui.Success("Android udev rules installed and reloaded.")
			}
		}
	}
}

func platformToolsMenu(client *device.Client, pm platform.PlatformManager, reader *bufio.Reader) {
	ui.Section("Platform-Tools & babe Updates")
	ui.RenderMenuItem(1, "Check Installed Status & PATH")
	ui.RenderMenuItem(2, "Download or Update Google Platform-Tools")
	ui.RenderMenuItem(3, "Check & Update babe (Self-Update)")
	ui.RenderMenuItem(4, "Cancel")
	ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Enter choice [1-4]:")))

	input, _ := reader.ReadString('\n')
	switch strings.TrimSpace(input) {
	case "1":
		checkPlatformTools(client, pm)
	case "2":
		downloadOrUpdatePlatformTools(client, pm)
	case "3":
		runSelfUpdateInteractive(reader)
	}
}

func checkPlatformTools(client *device.Client, pm platform.PlatformManager) {
	ui.Section("Platform-Tools Status")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	details := client.GetInstallationDetails(ctx)

	installDir, err := pm.GetDefaultInstallDir()
	inPath := false
	if err == nil {
		inPath, _ = pm.IsInPath(installDir)
	}

	ui.RenderADBStatus(details.Installed, details.Version, details.BinaryPath, installDir, inPath)
}

func downloadOrUpdatePlatformTools(client *device.Client, pm platform.PlatformManager) {
	ui.Section("Download or Update Platform-Tools")
	ctx := context.Background()

	checkCtx, cancelCheck := context.WithTimeout(ctx, 3*time.Second)
	details := client.GetInstallationDetails(checkCtx)
	cancelCheck()

	fmt.Println("Checking for latest platform-tools release...")
	netCtx, cancelNet := context.WithTimeout(ctx, 10*time.Second)
	release, err := tools.FetchLatestRelease(netCtx)
	cancelNet()

	if err != nil {
		if details.Installed {
			ui.Warn("Could not check remote repository: %v", err)
			ui.Info("Platform-tools v%s is currently installed at:\n     %s", details.Version, details.BinaryPath)
			return
		}
		ui.Warn("Could not query release metadata: %v", err)
		fmt.Println("Attempting download from default repository mirror...")
	}

	if details.Installed && release != nil {
		if tools.CompareVersions(details.Version, release.Version) >= 0 {
			ui.Success("Platform-tools is already up to date: v%s", details.Version)
			fmt.Printf("     Binary: %s\n", details.BinaryPath)
			return
		}
		ui.Info("Current version: v%s -> Latest version: v%s", details.Version, release.Version)
	} else if !details.Installed {
		ui.Info("Downloading Google Android Platform-Tools...")
	}

	downloadURL, expectedChecksum := "", ""
	if release != nil {
		downloadURL = release.ZipURL
		expectedChecksum = release.Checksum
	}

	var lastPercent int64 = -1
	configuredPath, err := tools.DownloadAndConfigurePlatformTools(ctx, downloadURL, expectedChecksum, func(current, total int64) {
		if total > 0 {
			percent := (current * 100) / total
			if percent != lastPercent {
				lastPercent = percent
				fmt.Printf("\rDownloading: %3d%% [%d / %d KB]", percent, current/1024, total/1024)
			}
		} else {
			fmt.Printf("\rDownloading: %d KB", current/1024)
		}
	})

	fmt.Println()
	if err != nil {
		ui.Error("Download failed: %v", err)
		return
	}

	client.SetBinaryPath("adb")
	ui.Success("Platform-tools configured at:\n     %s", configuredPath)
	ui.Success("System PATH environment updated.")
}

func selectDevice(client *device.Client, reader *bufio.Reader) (string, error) {
	devices, err := client.GetConnectedDevices(context.Background())
	if err != nil {
		return "", fmt.Errorf("list devices: %w", err)
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
	ui.Prompt(fmt.Sprintf("Select device [1-%d]: ", len(devices)))
	input, _ := reader.ReadString('\n')
	idx, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || idx < 1 || idx > len(devices) {
		return "", fmt.Errorf("invalid device selection")
	}
	selected := devices[idx-1]
	if err := device.ValidateDeviceState(selected); err != nil {
		return "", err
	}
	return selected.Serial, nil
}

func deviceMenu(client *device.Client, reader *bufio.Reader) {
	ui.Section("Device Telemetry & Reboot")
	ui.RenderMenuItem(1, "Inspect Hardware, OS & Battery Telemetry")
	ui.RenderMenuItem(2, "List Attached ADB Devices")
	ui.RenderMenuItem(3, "Reboot Device (System / Bootloader / Recovery / EDL)")
	ui.RenderMenuItem(4, "Cancel")
	ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Enter choice [1-4]:")))

	input, _ := reader.ReadString('\n')
	switch strings.TrimSpace(input) {
	case "1":
		serial, err := selectDevice(client, reader)
		if err != nil {
			ui.Warn("%v", err)
			return
		}
		info, err := client.GetDeviceInfo(context.Background(), serial)
		if err != nil {
			ui.Error("Querying device info: %v", err)
			return
		}
		ui.RenderDeviceInfo(info)
	case "2":
		devs, err := client.GetConnectedDevices(context.Background())
		if err != nil {
			ui.Error("%v", err)
			return
		}
		if len(devs) == 0 {
			ui.Warn("No ADB devices attached.")
			return
		}
		ui.RenderDeviceList(devs)
	case "3":
		rebootMenu(client, reader)
	}
}

func rebootMenu(client *device.Client, reader *bufio.Reader) {
	serial, err := selectDevice(client, reader)
	if err != nil {
		ui.Warn("%v", err)
		return
	}

	ui.RenderMenuItem(1, "System")
	ui.RenderMenuItem(2, "Bootloader (Fastboot)")
	ui.RenderMenuItem(3, "Recovery")
	ui.RenderMenuItem(4, "Qualcomm EDL")
	ui.RenderMenuItem(5, "Cancel")
	ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Enter choice [1-5]:")))

	input, _ := reader.ReadString('\n')
	var target device.RebootTarget
	switch strings.TrimSpace(input) {
	case "1":
		target = device.RebootSystem
	case "2":
		target = device.RebootBootloader
	case "3":
		target = device.RebootRecovery
	case "4":
		target = device.RebootEDL
	default:
		return
	}

	if err := client.Reboot(context.Background(), serial, target); err != nil {
		ui.Error("%v", err)
		return
	}
	ui.Success("Reboot command dispatched to device %s.", serial)
}
