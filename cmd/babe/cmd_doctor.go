package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/doctor"
	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/ui"
)

func newDoctorCmd(adbClient *device.Client, fbClient *fastboot.Client, pm platform.PlatformManager) *cobra.Command {
	var fixUdev bool

	cmd := &cobra.Command{
		Use:     "doctor",
		Short:   "Run system, PATH conflict & USB diagnostics",
		Long:    "Run system readiness, PATH shadowing conflicts, and USB device diagnostics.",
		GroupID: "core",
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleDoctorCLI(cmd, adbClient, fbClient, pm, fixUdev)
		},
	}

	cmd.Flags().BoolVar(&fixUdev, "fix-udev", false, "Install and reload Linux Android USB udev rules")

	return cmd
}

func handleDoctorCLI(cmd *cobra.Command, adbClient *device.Client, fbClient *fastboot.Client, pm platform.PlatformManager, fixUdev bool) error {
	results := doctor.RunDiagnostics(context.Background(), adbClient, fbClient, pm)
	if isJSON(cmd) {
		report := doctorReport{
			Status: summarizeDoctorStatus(results),
			Checks: results,
		}
		if fixUdev {
			if err := platform.InstallUdevRules(); err != nil {
				return err
			}
			fixed := true
			report.UdevFixed = &fixed
		}
		return writeJSON(cmd.OutOrStdout(), report)
	}

	ui.Section("System & USB Doctor")
	ui.RenderDoctorResults(results)

	if fixUdev {
		fmt.Println()
		return handleUdevCLI(cmd)
	}
	return nil
}
