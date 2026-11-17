package main

import (
	"context"
	"fmt"
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
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isJSON(cmd) {
				return fmt.Errorf("subcommand required")
			}
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().StringVarP(&serial, "serial", "s", "", "Target specific fastboot device serial")

	devsCmd := &cobra.Command{
		Use:     "devices",
		Aliases: []string{"list", "ls"},
		Short:   "List attached fastboot devices",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			devs, err := fbClient.GetConnectedDevices(context.Background())
			if err != nil {
				return err
			}
			if isJSON(cmd) {
				if devs == nil {
					devs = []fastboot.Device{}
				}
				return writeJSON(cmd.OutOrStdout(), fastbootListReport{Count: len(devs), Devices: devs})
			}
			if len(devs) == 0 {
				ui.Warn("No devices attached in Fastboot mode.")
				return nil
			}
			ui.RenderFastbootDevices(devs)
			return nil
		},
	}

	infoCmd := &cobra.Command{
		Use:     "info",
		Aliases: []string{"getvar"},
		Short:   "Inspect bootloader lock state, current slot & variables",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveFastbootDevice(cmd, fbClient, serial)
			if err != nil {
				return err
			}
			info, err := fbClient.GetDeviceInfo(context.Background(), target)
			if err != nil {
				return err
			}
			if isJSON(cmd) {
				return writeJSON(cmd.OutOrStdout(), info)
			}
			ui.RenderFastbootInfo(info)
			return nil
		},
	}

	slotCmd := &cobra.Command{
		Use:   "slot <a|b>",
		Short: "Switch active A/B boot slot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveFastbootDevice(cmd, fbClient, serial)
			if err != nil {
				return err
			}
			slot := strings.ToLower(strings.TrimSpace(args[0]))
			if err := fbClient.SetActiveSlot(context.Background(), target, slot); err != nil {
				return err
			}
			if isJSON(cmd) {
				return writeJSON(cmd.OutOrStdout(), slotReport{Serial: target, ActiveSlot: slot})
			}
			ui.Success("Active slot set to %s on %s.", strings.ToUpper(slot), target)
			return nil
		},
	}

	flashCmd := &cobra.Command{
		Use:   "flash <partition> <image-file>",
		Short: "Flash partition image with SHA-256 verification",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveFastbootDevice(cmd, fbClient, serial)
			if err != nil {
				return err
			}
			partition, imgPath := args[0], args[1]
			jsonMode := isJSON(cmd)
			if !jsonMode {
				cmd.Printf("Verifying SHA256 and flashing %s -> %s on %s...\n", imgPath, partition, target)
			}
			digest, err := fbClient.FlashPartition(context.Background(), target, partition, imgPath)
			if !jsonMode && digest != "" {
				ui.Info("Image SHA256: %s", digest)
			}
			if err != nil {
				return err
			}
			if jsonMode {
				return writeJSON(cmd.OutOrStdout(), flashReport{
					Serial:    target,
					Partition: partition,
					Image:     imgPath,
					SHA256:    digest,
					Flashed:   true,
				})
			}
			ui.Success("Partition %q flashed successfully.", partition)
			return nil
		},
	}

	cmd.AddCommand(devsCmd, infoCmd, slotCmd, flashCmd)
	return cmd
}

func resolveFastbootDevice(cmd *cobra.Command, fbClient *fastboot.Client, explicitSerial string) (string, error) {
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
	if !isJSON(cmd) {
		ui.RenderFastbootDevices(devs)
	}
	return "", fmt.Errorf("multiple fastboot devices detected; specify target with -s <serial>")
}
