package main

import (
	"bufio"
	"context"
	"fmt"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/ui"
)

func deviceMenu(client *device.Client, reader *bufio.Reader) {
	ui.Section("Device Telemetry & Reboot")
	ui.RenderMenuItem(1, "Inspect Hardware, OS & Battery Telemetry")
	ui.RenderMenuItem(2, "List Attached ADB Devices")
	ui.RenderMenuItem(3, "Reboot Device (System / Bootloader / Recovery / EDL)")
	ui.RenderMenuItem(4, "Cancel")
	ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Enter choice [1-4]:")))

	input, ok := readLine(reader)
	if !ok {
		return
	}
	switch input {
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

	input, ok := readLine(reader)
	if !ok {
		return
	}
	var target device.RebootTarget
	switch input {
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
