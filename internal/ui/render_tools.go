package ui

import (
	"fmt"
	"strings"

	"github.com/A7E28/babe/internal/apk"
	"github.com/A7E28/babe/internal/doctor"
	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/image"
)

// renders the diagnostic check suite results.
func RenderDoctorResults(results []doctor.CheckResult) {
	fmt.Println()
	for _, r := range results {
		switch r.Status {
		case doctor.StatusOK:
			Success("%-22s %s", r.Name+":", r.Summary)
		case doctor.StatusWarn:
			Warn("%-22s %s", r.Name+":", r.Summary)
		case doctor.StatusError:
			Error("%-22s %s", r.Name+":", r.Summary)
		}
		if r.Remedy != "" {
			fmt.Printf("     %s %s\n", Gray("Fix:"), White(r.Remedy))
		}
	}
}

// renders static analysis metadata for an APK or Split APK bundle.
func RenderArchiveInfo(info *apk.ArchiveInfo, showAllPerms bool) {
	fmt.Println()
	fmt.Printf("  %s %s %s\n", Gray("File:           "), White(info.FilePath), Gray(fmt.Sprintf("(%d KB)", info.FileSize/1024)))
	kind := Green("Standard Android APK")
	if info.IsBundle {
		kind = Cyan(fmt.Sprintf("Split APK Bundle (%d splits)", len(info.SplitAPKs)))
	}
	fmt.Printf("  %s %s\n", Gray("Archive Type:   "), kind)

	if info.PackageName != "" {
		fmt.Printf("  %s %s\n", Gray("Package Name:   "), BoldCyan(info.PackageName))
	}
	if info.VersionName != "" || info.VersionCode > 0 {
		fmt.Printf("  %s %s %s\n", Gray("Version:        "), White(info.VersionName), Gray(fmt.Sprintf("(code %d)", info.VersionCode)))
	}
	if info.MinSDK > 0 || info.TargetSDK > 0 || info.MinSDKCodename != "" || info.TargetSDKCodename != "" {
		minStr := fmt.Sprintf("%d", info.MinSDK)
		if info.MinSDKCodename != "" {
			minStr = info.MinSDKCodename
		}
		targetStr := fmt.Sprintf("%d", info.TargetSDK)
		if info.TargetSDKCodename != "" {
			targetStr = info.TargetSDKCodename
		}
		fmt.Printf("  %s %s\n", Gray("SDK Levels:     "), White(fmt.Sprintf("minSdk %s / targetSdk %s", minStr, targetStr)))
	}
	if len(info.ABIs) > 0 {
		fmt.Printf("  %s %s\n", Gray("Native ABIs:    "), Cyan(strings.Join(info.ABIs, ", ")))
	}
	if info.DexCount > 0 {
		fmt.Printf("  %s %s\n", Gray("DEX Files:      "), White(fmt.Sprintf("%d", info.DexCount)))
	}
	if len(info.SignatureFiles) > 0 {
		fmt.Printf("  %s %s\n", Gray("Signatures:     "), Green(strings.Join(info.SignatureFiles, ", ")))
	}
	if len(info.Permissions) > 0 {
		fmt.Printf("  %s %s\n", Gray("Permissions:    "), White(fmt.Sprintf("%d declared", len(info.Permissions))))
		limit := len(info.Permissions)
		if !showAllPerms && limit > 8 {
			limit = 8
		}
		for _, p := range info.Permissions[:limit] {
			fmt.Printf("    %s %s\n", Gray("-"), p)
		}
		if len(info.Permissions) > limit {
			fmt.Printf("    %s\n", Gray(fmt.Sprintf("... and %d more (pass -a / --all to list all)", len(info.Permissions)-limit)))
		}
	}
}

// renders parsed Android boot.img or init_boot.img header telemetry.
func RenderBootImageInfo(info *image.BootImageInfo) {
	fmt.Println()
	fmt.Printf("  %s %s %s\n", Gray("Image File:     "), White(info.FilePath), Gray(fmt.Sprintf("(%d KB)", info.FileSize/1024)))
	fmt.Printf("  %s %s\n", Gray("Header Version: "), BoldCyan(fmt.Sprintf("v%d", info.HeaderVersion)))
	fmt.Printf("  %s %s\n", Gray("Page Size:      "), White(fmt.Sprintf("%d bytes", info.PageSize)))
	fmt.Printf("  %s %s\n", Gray("Kernel Size:    "), Cyan(fmt.Sprintf("%d bytes (%.2f MB)", info.KernelSize, float64(info.KernelSize)/(1024*1024))))
	fmt.Printf("  %s %s\n", Gray("Ramdisk Size:   "), Cyan(fmt.Sprintf("%d bytes (%.2f MB)", info.RamdiskSize, float64(info.RamdiskSize)/(1024*1024))))
	if info.OSVersion != "" {
		fmt.Printf("  %s %s\n", Gray("OS Version:     "), Green(info.OSVersion))
	}
	if info.OSPatchLevel != "" {
		fmt.Printf("  %s %s\n", Gray("Security Patch: "), White(info.OSPatchLevel))
	}
	if info.BoardName != "" {
		fmt.Printf("  %s %s\n", Gray("Board Name:     "), White(info.BoardName))
	}
	if info.Cmdline != "" {
		fmt.Printf("  %s %s\n", Gray("Kernel Cmdline: "), Gray(info.Cmdline))
	}
}

// renders attached Fastboot/Bootloader devices.
func RenderFastbootDevices(devices []fastboot.Device) {
	fmt.Printf("\n%s\n", Gray("Fastboot devices:"))
	for i, d := range devices {
		fmt.Printf("  %s %s %s\n",
			Cyan(fmt.Sprintf("[%d]", i+1)),
			Bold(d.Serial),
			Yellow(fmt.Sprintf("[%s]", d.Mode)),
		)
	}
}

// renders bootloader variables for a Fastboot device.
func RenderFastbootInfo(info *fastboot.DeviceInfo) {
	fmt.Println()
	fmt.Printf("  %s %s\n", Gray("Serial:         "), BoldCyan(info.Serial))
	fmt.Printf("  %s %s\n", Gray("Product:        "), White(info.Product))
	fmt.Printf("  %s %s\n", Gray("Bootloader Ver: "), White(info.BootloaderVersion))
	fmt.Printf("  %s %s\n", Gray("Baseband Ver:   "), White(info.BasebandVersion))
	unlocked := Green("No (Locked)")
	if strings.EqualFold(info.Unlocked, "yes") {
		unlocked = Yellow("Yes (Unlocked)")
	}
	fmt.Printf("  %s %s\n", Gray("Unlocked:       "), unlocked)
	fmt.Printf("  %s %s\n", Gray("Secure Boot:    "), White(info.Secure))
	if info.CurrentSlot != "" {
		fmt.Printf("  %s %s\n", Gray("Active Slot:    "), BoldCyan(strings.ToUpper(info.CurrentSlot)))
	}
	if info.MaxDownloadSize != "" {
		fmt.Printf("  %s %s\n", Gray("Max Download:   "), Gray(info.MaxDownloadSize))
	}
}
