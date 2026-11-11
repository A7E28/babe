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
		ans, ok := readLine(reader)
		if !ok {
			return
		}
		if strings.EqualFold(ans, "y") {
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
	ui.RenderMenuItem(4, "Install babe to System PATH (Global Setup)")
	ui.RenderMenuItem(5, "Cancel")
	ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Enter choice [1-5]:")))

	input, ok := readLine(reader)
	if !ok {
		return
	}
	switch input {
	case "1":
		checkPlatformTools(client, pm)
		runBabeSetup(pm, false, true)
	case "2":
		downloadOrUpdatePlatformTools(client, pm, false, false)
	case "3":
		runSelfUpdateInteractive(reader)
	case "4":
		runBabeSetup(pm, false, false)
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

func downloadOrUpdatePlatformTools(client *device.Client, pm platform.PlatformManager, force, checkOnly bool) {
	ui.Section("Download or Update Platform-Tools")
	ctx := context.Background()

	checkCtx, cancelCheck := context.WithTimeout(ctx, 3*time.Second)
	details := client.GetInstallationDetails(checkCtx)
	cancelCheck()

	fmt.Println("Checking for latest platform-tools release...")
	netCtx, cancelNet := context.WithTimeout(ctx, 10*time.Second)
	release, err := tools.FetchLatestRelease(netCtx)
	cancelNet()

	if checkOnly {
		if err != nil {
			ui.Error("Failed to check remote repository: %v", err)
			return
		}
		if !details.Installed {
			ui.Info("Platform-tools is not installed (latest available: v%s).", release.Version)
			ui.Info("Run 'babe update tools' to download and configure.")
		} else if tools.CompareVersions(details.Version, release.Version) < 0 {
			ui.Info("Update available: v%s -> v%s", details.Version, release.Version)
			ui.Info("Run 'babe update tools' to install.")
		} else {
			ui.Success("Platform-tools is up to date (v%s).", details.Version)
		}
		return
	}

	if err != nil {
		if details.Installed && !force {
			ui.Warn("Could not check remote repository: %v", err)
			ui.Info("Platform-tools v%s is currently installed at:\n     %s", details.Version, details.BinaryPath)
			return
		}
		ui.Warn("Could not query release metadata: %v", err)
		fmt.Println("Attempting download from default repository mirror...")
	}

	if details.Installed && release != nil && !force {
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
		return "", device.ErrNoDevicesFound
	}
	if len(devices) == 1 {
		if err := device.ValidateDeviceState(devices[0]); err != nil {
			return "", err
		}
		return devices[0].Serial, nil
	}

	ui.RenderDeviceList(devices)
	ui.Prompt(fmt.Sprintf("Select device [1-%d]: ", len(devices)))
	input, ok := readLine(reader)
	if !ok {
		return "", fmt.Errorf("device selection cancelled")
	}
	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(devices) {
		return "", fmt.Errorf("invalid device selection")
	}
	selected := devices[idx-1]
	if err := device.ValidateDeviceState(selected); err != nil {
		return "", err
	}
	return selected.Serial, nil
}
