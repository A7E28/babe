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
		Run: func(cmd *cobra.Command, args []string) {
			handleDoctorCLI(adbClient, fbClient, pm, fixUdev)
		},
	}

	cmd.Flags().BoolVar(&fixUdev, "fix-udev", false, "Install and reload Linux Android USB udev rules")

	return cmd
}

func handleDoctorCLI(adbClient *device.Client, fbClient *fastboot.Client, pm platform.PlatformManager, fixUdev bool) {
	ui.Section("System & USB Doctor")
	results := doctor.RunDiagnostics(context.Background(), adbClient, fbClient, pm)
	ui.RenderDoctorResults(results)

	if fixUdev {
		fmt.Println()
		handleUdevCLI()
	}
}
