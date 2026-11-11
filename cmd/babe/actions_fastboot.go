package main

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/image"
	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/ui"
)

func selectFastbootDevice(fbClient *fastboot.Client, reader *bufio.Reader) (string, error) {
	devices, err := fbClient.GetConnectedDevices(context.Background())
	if err != nil {
		return "", fmt.Errorf("list fastboot devices: %w", err)
	}
	if len(devices) == 0 {
		return "", fmt.Errorf("no devices detected in Fastboot mode")
	}
	if len(devices) == 1 {
		return devices[0].Serial, nil
	}

	ui.RenderFastbootDevices(devices)
	ui.Prompt(fmt.Sprintf("Select fastboot device [1-%d]: ", len(devices)))
	input, _ := reader.ReadString('\n')
	idx, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || idx < 1 || idx > len(devices) {
		return "", fmt.Errorf("invalid fastboot device selection")
	}
	return devices[idx-1].Serial, nil
}

func fastbootMenu(fbClient *fastboot.Client, pm platform.PlatformManager, reader *bufio.Reader) {
	ui.Section("Fastboot & Slot A/B Manager")
	ui.RenderMenuItem(1, "List Connected Fastboot Devices")
	ui.RenderMenuItem(2, "Inspect Bootloader Lock & Slot Variables")
	ui.RenderMenuItem(3, "Switch Active A/B Boot Slot")
	ui.RenderMenuItem(4, "Flash Partition Image (Hash-Verified)")
	ui.RenderMenuItem(5, "Reboot from Fastboot to System")
	ui.RenderMenuItem(6, "Cancel")
	ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Enter choice [1-6]:")))

	choiceStr, _ := reader.ReadString('\n')
	switch strings.TrimSpace(choiceStr) {
	case "1":
		devs, err := fbClient.GetConnectedDevices(context.Background())
		if err != nil {
			ui.Error("%v", err)
			return
		}
		if len(devs) == 0 {
			ui.Warn("No devices currently attached in Fastboot mode.")
			return
		}
		ui.RenderFastbootDevices(devs)
	case "2":
		serial, err := selectFastbootDevice(fbClient, reader)
		if err != nil {
			ui.Warn("%v", err)
			return
		}
		info, err := fbClient.GetDeviceInfo(context.Background(), serial)
		if err != nil {
			ui.Error("%v", err)
			return
		}
		ui.RenderFastbootInfo(info)
	case "3":
		serial, err := selectFastbootDevice(fbClient, reader)
		if err != nil {
			ui.Warn("%v", err)
			return
		}
		ui.Prompt(fmt.Sprintf("%s ", ui.White("Target slot [a/b]:")))
		slotInput, _ := reader.ReadString('\n')
		slot := strings.TrimSpace(slotInput)
		if err := fbClient.SetActiveSlot(context.Background(), serial, slot); err != nil {
			ui.Error("%v", err)
			return
		}
		ui.Success("Active slot set to %s on %s.", strings.ToUpper(slot), serial)
	case "4":
		serial, err := selectFastbootDevice(fbClient, reader)
		if err != nil {
			ui.Warn("%v", err)
			return
		}
		ui.Prompt(fmt.Sprintf("%s ", ui.White("Target partition name (e.g. boot, init_boot, recovery):")))
		partInput, _ := reader.ReadString('\n')
		partition := strings.TrimSpace(partInput)
		if partition == "" {
			ui.Warn("Partition name is required.")
			return
		}
		imgPath := promptFileWithFallback(pm, reader, "Select Partition Image (.img)")
		if imgPath == "" {
			return
		}
		fmt.Printf("Verifying SHA256 and flashing %s -> %s...\n", imgPath, partition)
		digest, err := fbClient.FlashPartition(context.Background(), serial, partition, imgPath)
		if digest != "" {
			ui.Info("Image SHA256: %s", digest)
		}
		if err != nil {
			ui.Error("%v", err)
			return
		}
		ui.Success("Partition %q flashed successfully.", partition)
	case "5":
		serial, err := selectFastbootDevice(fbClient, reader)
		if err != nil {
			ui.Warn("%v", err)
			return
		}
		if _, err := fbClient.ExecuteOnDevice(context.Background(), serial, "reboot"); err != nil {
			ui.Error("%v", err)
			return
		}
		ui.Success("Fastboot reboot command sent to %s.", serial)
	}
}

func bootImageMenu(pm platform.PlatformManager, reader *bufio.Reader) {
	ui.Section("Firmware Boot Image Inspector (Offline)")
	imgPath := promptFileWithFallback(pm, reader, "Select boot.img or init_boot.img")
	if imgPath == "" {
		return
	}

	info, err := image.InspectBootImage(imgPath)
	if err != nil {
		ui.Error("Boot image inspection failed: %v", err)
		return
	}
	ui.RenderBootImageInfo(info)
}

func promptFileWithFallback(pm platform.PlatformManager, reader *bufio.Reader, title string) string {
	filePath, err := pm.OpenFilePicker(title)
	if err != nil {
		ui.Warn("File dialog unavailable: %v", err)
		ui.Prompt("Enter file path: ")
		input, _ := reader.ReadString('\n')
		filePath = strings.TrimSpace(input)
	}
	if filePath == "" {
		ui.Warn("No file selected.")
	}
	return filePath
}
