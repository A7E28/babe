package main

import (
	"context"
	"fmt"
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
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleDeviceListCLI(cmd, client)
		},
	}

	cmd.PersistentFlags().StringVarP(&serial, "serial", "s", "", "Target specific device serial (default: auto-detected)")

	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List attached ADB devices and their state",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleDeviceListCLI(cmd, client)
		},
	}

	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Display device hardware, OS, battery & root status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleDeviceInfoCLI(cmd, client, serial)
		},
	}

	rebootCmd := &cobra.Command{
		Use:   "reboot [system|bootloader|recovery|edl]",
		Short: "Reboot device into specified target mode",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) > 0 {
				target = strings.ToLower(args[0])
			}
			return handleRebootCLI(cmd, client, target, serial)
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleDeviceListCLI(cmd, client)
		},
	}
}

func newInfoAliasCmd(client *device.Client) *cobra.Command {
	var serial string
	cmd := &cobra.Command{
		Use:    "info",
		Hidden: true,
		Short:  "Display device hardware, OS, battery & root status",
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleDeviceInfoCLI(cmd, client, serial)
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
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) > 0 {
				target = strings.ToLower(args[0])
			}
			return handleRebootCLI(cmd, client, target, serial)
		},
	}
	cmd.Flags().StringVarP(&serial, "serial", "s", "", "Target specific device serial")
	return cmd
}

func resolveTargetDevice(cmd *cobra.Command, client *device.Client, explicitSerial string) (string, error) {
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

	if !isJSON(cmd) {
		ui.RenderDeviceList(devices)
	}
	return "", fmt.Errorf("multiple devices detected; specify target with -s <serial>")
}

func handleDeviceListCLI(cmd *cobra.Command, client *device.Client) error {
	devices, err := client.GetConnectedDevices(context.Background())
	if err != nil {
		return err
	}
	if isJSON(cmd) {
		if devices == nil {
			devices = []device.Device{}
		}
		return writeJSON(cmd.OutOrStdout(), deviceListReport{Count: len(devices), Devices: devices})
	}
	if len(devices) == 0 {
		ui.Warn("No ADB devices attached.")
		return nil
	}
	ui.RenderDeviceList(devices)
	return nil
}

func handleDeviceInfoCLI(cmd *cobra.Command, client *device.Client, serial string) error {
	targetSerial, err := resolveTargetDevice(cmd, client, serial)
	if err != nil {
		return err
	}
	info, err := client.GetDeviceInfo(context.Background(), targetSerial)
	if err != nil {
		return fmt.Errorf("query device info: %w", err)
	}
	if isJSON(cmd) {
		return writeJSON(cmd.OutOrStdout(), info)
	}
	ui.RenderDeviceInfo(info)
	return nil
}

func handleRebootCLI(cmd *cobra.Command, client *device.Client, targetStr, serial string) error {
	targetSerial, err := resolveTargetDevice(cmd, client, serial)
	if err != nil {
		return err
	}

	var target device.RebootTarget
	switch targetStr {
	case "", "system", "normal":
		target = device.RebootSystem
		targetStr = "system"
	case "bootloader", "fastboot":
		target = device.RebootBootloader
	case "recovery":
		target = device.RebootRecovery
	case "edl":
		target = device.RebootEDL
	default:
		return fmt.Errorf("unknown reboot target %q (valid: system, bootloader, recovery, edl)", targetStr)
	}

	if err := client.Reboot(context.Background(), targetSerial, target); err != nil {
		return err
	}
	if isJSON(cmd) {
		return writeJSON(cmd.OutOrStdout(), rebootReport{Serial: targetSerial, Target: targetStr, Rebooted: true})
	}
	ui.Success("Reboot command sent to device %s.", targetSerial)
	return nil
}
