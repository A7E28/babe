package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/doctor"
	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/platform"
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
		if err := checkPlatformTools(os.Stdout, client, pm); err != nil {
			ui.Error("%v", err)
			return
		}
		if err := runBabeSetup(os.Stdout, pm, false, true); err != nil {
			ui.Error("%v", err)
		}
	case "2":
		if err := downloadOrUpdatePlatformTools(os.Stdout, client, pm, false, false); err != nil {
			ui.Error("%v", err)
		}
	case "3":
		runSelfUpdateInteractive(reader)
	case "4":
		if err := runBabeSetup(os.Stdout, pm, false, false); err != nil {
			ui.Error("%v", err)
		}
	}
}

func checkPlatformTools(w io.Writer, client *device.Client, pm platform.PlatformManager) error {
	if w == nil {
		w = os.Stdout
	}
	report, err := collectToolsStatus(client, pm)
	if err != nil {
		return err
	}
	ui.Section("Platform-Tools Status")
	pt := report.PlatformTools
	ui.RenderADBStatus(pt.Installed, pt.Version, pt.BinaryPath, pt.InstallDir, pt.InPath)
	return nil
}

func downloadOrUpdatePlatformTools(w io.Writer, client *device.Client, pm platform.PlatformManager, force, checkOnly bool) error {
	if w == nil {
		w = os.Stdout
	}
	ui.Section("Download or Update Platform-Tools")
	fmt.Fprintln(w, "Checking for latest platform-tools release...")

	var lastPercent int64 = -1
	var startedDownload bool
	report, err := executeToolsUpdate(client, force, checkOnly, func(msg string) {
		ui.Warn("%s", msg)
	}, func(current, total int64) {
		if !startedDownload {
			startedDownload = true
			ui.Info("Downloading Google Android Platform-Tools...")
		}
		if total > 0 {
			if percent := (current * 100) / total; percent != lastPercent {
				lastPercent = percent
				fmt.Fprintf(w, "\rDownloading: %3d%% [%d / %d KB]", percent, current/1024, total/1024)
			}
		} else {
			fmt.Fprintf(w, "\rDownloading: %d KB", current/1024)
		}
	})
	if startedDownload {
		fmt.Fprintln(w)
	}
	if err != nil {
		return err
	}

	if checkOnly {
		if report.Installed != nil && !*report.Installed {
			ui.Info("Platform-tools is not installed (latest available: v%s).", report.LatestVersion)
			ui.Info("Run 'babe update tools' to download and configure.")
		} else if report.UpdateAvailable {
			ui.Info("Update available: v%s -> v%s", report.CurrentVersion, report.LatestVersion)
			ui.Info("Run 'babe update tools' to install.")
		} else {
			ui.Success("Platform-tools is up to date (v%s).", report.CurrentVersion)
		}
		return nil
	}

	if !report.Updated {
		if report.Warning != "" && report.Installed != nil && *report.Installed {
			ui.Info("Platform-tools v%s is currently installed at:\n     %s", report.CurrentVersion, report.Path)
			return nil
		}
		ui.Success("Platform-tools is already up to date: v%s", report.CurrentVersion)
		fmt.Fprintf(w, "     Binary: %s\n", report.Path)
		return nil
	}

	ui.Success("Platform-tools configured at:\n     %s", report.Path)
	ui.Success("System PATH environment updated.")
	return nil
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
