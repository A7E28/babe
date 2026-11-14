package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/ui"
)

func newFastbootCmd(fbClient *fastboot.Client) *cobra.Command {
	var serial string

	cmd := &cobra.Command{
		Use:     "fastboot",
		Aliases: []string{"fb"},
		Short:   "Manage devices in Fastboot / Bootloader mode",
		Long:    "Manage devices in Fastboot / Bootloader mode.",
		GroupID: "device",
	}

	cmd.PersistentFlags().StringVarP(&serial, "serial", "s", "", "Target specific fastboot device serial")

	devsCmd := &cobra.Command{
		Use:     "devices",
		Aliases: []string{"list", "ls"},
		Short:   "List attached fastboot devices",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
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
		},
	}

	infoCmd := &cobra.Command{
		Use:     "info",
		Aliases: []string{"getvar"},
		Short:   "Inspect bootloader lock state, current slot & variables",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			target, err := resolveFastbootDevice(fbClient, serial)
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
		},
	}

	slotCmd := &cobra.Command{
		Use:   "slot <a|b>",
		Short: "Switch active A/B boot slot",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			target, err := resolveFastbootDevice(fbClient, serial)
			if err != nil {
				ui.Error("%v", err)
				os.Exit(1)
			}
			slot := args[0]
			if err := fbClient.SetActiveSlot(context.Background(), target, slot); err != nil {
				ui.Error("%v", err)
				os.Exit(1)
			}
			ui.Success("Active slot set to %s on %s.", strings.ToUpper(slot), target)
		},
	}

	flashCmd := &cobra.Command{
		Use:   "flash <partition> <image-file>",
		Short: "Flash partition image with SHA-256 verification",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			target, err := resolveFastbootDevice(fbClient, serial)
			if err != nil {
				ui.Error("%v", err)
				os.Exit(1)
			}
			partition, imgPath := args[0], args[1]
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
		},
	}

	cmd.AddCommand(devsCmd, infoCmd, slotCmd, flashCmd)
	return cmd
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
