package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/ui"
)

func runInteractiveMenu() {
	reader := bufio.NewReader(os.Stdin)
	pm := platform.DefaultManager()
	adbClient := device.NewClient()
	fbClient := fastboot.NewClient()

	for {
		ui.RenderBanner(version)
		ui.RenderMenuItem(1, "System & USB Doctor (Diagnostics & udev)")
		ui.RenderMenuItem(2, "Platform-Tools Manager (Status / Install / Update)")
		ui.RenderMenuItem(3, "Device Telemetry & Reboot (ADB)")
		ui.RenderMenuItem(4, "Fastboot & Slot A/B Manager")
		ui.RenderMenuItem(5, "Logcat Stream & Crash Harvester")
		ui.RenderMenuItem(6, "APK & Split-Bundle Suite (Inspect / Install / Packages)")
		ui.RenderMenuItem(7, "Firmware Boot Image Inspector (boot.img / init_boot.img)")
		ui.RenderMenuItem(8, "Calculate File Checksum (MD5 / SHA1 / SHA256)")
		ui.RenderMenuItem(9, "Exit")
		ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Enter choice [1-9]:")))

		choiceStr, err := reader.ReadString('\n')
		if err != nil && len(choiceStr) == 0 {
			fmt.Printf("\n%s\n", ui.Gray("Exiting babe."))
			return
		}
		choice := strings.TrimSpace(choiceStr)

		switch choice {
		case "1":
			runDoctorMenu(adbClient, fbClient, pm, reader)
		case "2":
			platformToolsMenu(adbClient, pm, reader)
		case "3":
			deviceMenu(adbClient, reader)
		case "4":
			fastbootMenu(fbClient, pm, reader)
		case "5":
			logcatMenu(adbClient, reader)
		case "6":
			apkSuiteMenu(adbClient, pm, reader)
		case "7":
			bootImageMenu(pm, reader)
		case "8":
			checksumMenu(pm, reader)
		case "9", "q", "exit":
			fmt.Printf("\n%s\n", ui.Gray("Exiting babe."))
			return
		default:
			ui.Warn("Invalid selection. Choose an option from 1 to 9.")
		}

		ui.Prompt(fmt.Sprintf("\n%s", ui.Gray("Press Enter to continue...")))
		if _, err := reader.ReadString('\n'); err != nil {
			fmt.Printf("\n%s\n", ui.Gray("Exiting babe."))
			return
		}
	}
}
