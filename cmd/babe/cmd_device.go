package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/ui"
)

func newDeviceCmd(client *device.Client) *cobra.Command {
	var serial string

	cmd := &cobra.Command{
		Use:     "device",
		Aliases: []string{"devices"},
		Short:   "Manage and inspect connected ADB devices",
		Long:    "Manage and inspect connected ADB devices.",
		GroupID: "device",
		Run: func(cmd *cobra.Command, args []string) {
			handleDeviceListCLI(client)
		},
	}

	cmd.PersistentFlags().StringVarP(&serial, "serial", "s", "", "Target specific device serial (default: auto-detected)")

	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List attached ADB devices and their state",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			handleDeviceListCLI(client)
		},
	}

	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Display device hardware, OS, battery & root status",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			handleDeviceInfoCLI(client, serial)
		},
	}

	rebootCmd := &cobra.Command{
		Use:   "reboot [system|bootloader|recovery|edl]",
		Short: "Reboot device into specified target mode",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			target := ""
			if len(args) > 0 {
				target = strings.ToLower(args[0])
			}
			handleRebootCLI(client, target, serial)
		},
	}

	cmd.AddCommand(listCmd, infoCmd, rebootCmd)
	return cmd
}

func newDevicesAliasCmd(client *device.Client) *cobra.Command {
	return &cobra.Command{
		Use:    "devices",
		Hidden: true,
		Short:  "List attached ADB devices",
		Args:   cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			handleDeviceListCLI(client)
		},
	}
}

func newInfoAliasCmd(client *device.Client) *cobra.Command {
	var serial string
	cmd := &cobra.Command{
		Use:    "info",
		Hidden: true,
		Short:  "Display device hardware, OS, battery & root status",
		Run: func(cmd *cobra.Command, args []string) {
			handleDeviceInfoCLI(client, serial)
		},
	}
	cmd.Flags().StringVarP(&serial, "serial", "s", "", "Target specific device serial")
	return cmd
}

func newRebootAliasCmd(client *device.Client) *cobra.Command {
	var serial string
	cmd := &cobra.Command{
		Use:    "reboot [system|bootloader|recovery|edl]",
		Hidden: true,
		Short:  "Reboot device into specified target mode",
		Args:   cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			target := ""
			if len(args) > 0 {
				target = strings.ToLower(args[0])
			}
			handleRebootCLI(client, target, serial)
		},
	}
	cmd.Flags().StringVarP(&serial, "serial", "s", "", "Target specific device serial")
	return cmd
}

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
