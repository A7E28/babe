package ui

import (
	"fmt"

	"github.com/A7E28/babe/internal/device"
)

// displays the application banner with the active version.
func RenderBanner(version string) {
	ClearScreen()
	Divider()
	fmt.Printf("  %s %s\n", BoldCyan("babe"), Gray(version))
	Divider()
}

// formats a menu option line with subtle index accent.
func RenderMenuItem(index int, title string) {
	fmt.Printf("  %s  %s\n", Cyan(fmt.Sprintf("[%d]", index)), White(title))
}

// renders device telemetry and hardware properties.
func RenderDeviceInfo(info *device.Info) {
	fmt.Println()
	fmt.Printf("  %s %s\n", Gray("Serial:         "), BoldCyan(info.Serial))
	if info.Brand != "" && info.Model != "" {
		fmt.Printf("  %s %s %s\n", Gray("Model:          "), White(info.Model), Gray("("+info.Brand+")"))
	} else if info.Model != "" {
		fmt.Printf("  %s %s\n", Gray("Model:          "), White(info.Model))
	}
	fmt.Printf("  %s %s\n", Gray("Manufacturer:   "), White(info.Manufacturer))
	fmt.Printf("  %s %s %s\n", Gray("Android Version:"), Green(info.AndroidVersion), Gray(fmt.Sprintf("(API %d)", info.SDKLevel)))
	fmt.Printf("  %s %s\n", Gray("Architecture:   "), White(info.Architecture))
	fmt.Printf("  %s %s\n", Gray("Security Patch: "), White(info.SecurityPatch))
	fmt.Printf("  %s %s\n", Gray("Build ID:       "), Gray(info.BuildID))

	charging := Gray("No")
	if info.BatteryCharging {
		charging = Green("Yes")
	}
	fmt.Printf("  %s %s\n", Gray("Battery Level:  "), Green(fmt.Sprintf("%d%%", info.BatteryLevel)))
	fmt.Printf("  %s %s\n", Gray("Charging:       "), charging)

	rooted := Gray("No")
	if info.IsRooted {
		rooted = Yellow("Yes")
	}
	fmt.Printf("  %s %s\n", Gray("Root Access:    "), rooted)
}

// renders formatted devices for selection.
func RenderDeviceList(devices []device.Device) {
	fmt.Printf("\n%s\n", Gray("Attached devices:"))
	for i, d := range devices {
		modelStr := ""
		if d.Model != "" {
			modelStr = " " + Gray(d.Model)
		}
		stateBadge := formatDeviceState(d.State)
		fmt.Printf("  %s %s %s%s\n",
			Cyan(fmt.Sprintf("[%d]", i+1)),
			Bold(d.Serial),
			stateBadge,
			modelStr,
		)
	}
}

func formatDeviceState(st device.State) string {
	label := fmt.Sprintf("[%s]", st)
	switch st {
	case device.StateDevice:
		return Green(label)
	case device.StateUnauthorized:
		return Yellow(label)
	case device.StateOffline:
		return Red(label)
	default:
		return Cyan(label)
	}
}

// displays calculated cryptographic hashes.
func RenderChecksums(filePath, md5Val, sha1Val, sha256Val string) {
	fmt.Printf("\n  %s %s\n", Gray("File:  "), White(filePath))
	fmt.Printf("  %s %s\n", Gray("MD5:   "), Cyan(md5Val))
	fmt.Printf("  %s %s\n", Gray("SHA1:  "), Cyan(sha1Val))
	fmt.Printf("  %s %s\n", Gray("SHA256:"), Cyan(sha256Val))
}

// prints the current status of the platform-tools ADB binary.
func RenderADBStatus(installed bool, version, binaryPath, defaultDir string, inPath bool) {
	if installed {
		Success("ADB detected on system PATH:")
		if version != "" {
			fmt.Printf("     %s %s\n", Gray("Version:     "), BoldCyan(version))
		}
		if binaryPath != "" {
			fmt.Printf("     %s %s\n", Gray("Binary:      "), White(binaryPath))
		}
		if defaultDir != "" {
			pathState := Yellow("Not in user PATH")
			if inPath {
				pathState = Green("Configured")
			}
			fmt.Printf("     %s %s\n", Gray("Managed PATH:"), pathState)
		}
	} else {
		Warn("ADB not found on system PATH.")
		fmt.Printf("    %s\n", Gray("Use option 2 (or 'babe update') to download platform-tools."))
		if defaultDir != "" {
			fmt.Printf("    %s %s\n", Gray("Target: "), White(defaultDir))
		}
	}
}
