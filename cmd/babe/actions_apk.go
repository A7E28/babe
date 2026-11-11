package main

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/A7E28/babe/internal/apk"
	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/ui"
)

func apkSuiteMenu(client *device.Client, pm platform.PlatformManager, reader *bufio.Reader) {
	ui.Section("APK & Split-Bundle Suite")
	ui.RenderMenuItem(1, "Inspect APK / Split Bundle Offline (Manifest, SDK, ABIs, Certs)")
	ui.RenderMenuItem(2, "Install APK or Split Bundle (.apk / .apks / .xapk / .apkm)")
	ui.RenderMenuItem(3, "List Installed Packages on Device")
	ui.RenderMenuItem(4, "Pull Installed Package APK to Computer")
	ui.RenderMenuItem(5, "Cancel")
	ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Enter choice [1-5]:")))

	choiceStr, ok := readLine(reader)
	if !ok {
		return
	}
	switch choiceStr {
	case "1":
		archivePath := promptFileWithFallback(pm, reader, "Select APK or Split Bundle")
		if archivePath == "" {
			return
		}
		info, err := apk.InspectArchive(archivePath)
		if err != nil {
			ui.Error("Inspection failed: %v", err)
			return
		}
		ui.RenderArchiveInfo(info, false)
		if len(info.Permissions) > 8 {
			ui.Prompt(fmt.Sprintf("\n%s ", ui.White(fmt.Sprintf("Show all %d permissions? [y/N]:", len(info.Permissions)))))
			ans, ok := readLine(reader)
			if ok && strings.EqualFold(ans, "y") {
				for _, p := range info.Permissions {
					fmt.Printf("    %s %s\n", ui.Gray("-"), p)
				}
			}
		}
	case "2":
		serial, err := selectDevice(client, reader)
		if err != nil {
			ui.Warn("%v", err)
			return
		}
		archivePath := promptFileWithFallback(pm, reader, "Select APK or Split Bundle")
		if archivePath == "" {
			return
		}
		fmt.Printf("Installing %s on %s...\n", archivePath, serial)
		opts := apk.InstallOptions{Reinstall: true, Downgrade: true, GrantPerm: true}
		if err := apk.InstallAuto(context.Background(), client, serial, archivePath, opts); err != nil {
			ui.Error("%v", err)
		} else {
			ui.Success("Package installed successfully.")
		}
	case "3":
		serial, err := selectDevice(client, reader)
		if err != nil {
			ui.Warn("%v", err)
			return
		}
		ui.Prompt(fmt.Sprintf("%s ", ui.White("Third-party packages only? [Y/n]:")))
		tpStr, ok := readLine(reader)
		if !ok {
			return
		}
		thirdPartyOnly := !strings.EqualFold(tpStr, "n")

		ui.Prompt(fmt.Sprintf("%s %s: ", ui.White("Filter substring"), ui.Gray("[optional]")))
		filterStr, ok := readLine(reader)
		if !ok {
			return
		}
		filter := strings.ToLower(filterStr)

		pkgs, err := apk.ListPackages(context.Background(), client, serial, thirdPartyOnly)
		if err != nil {
			ui.Error("%v", err)
			return
		}
		count := 0
		for _, p := range pkgs {
			if filter == "" || strings.Contains(strings.ToLower(p), filter) {
				fmt.Printf("  %s\n", p)
				count++
			}
		}
		ui.Info("Displayed %d package(s).", count)
	case "4":
		serial, err := selectDevice(client, reader)
		if err != nil {
			ui.Warn("%v", err)
			return
		}
		ui.Prompt(fmt.Sprintf("%s: ", ui.White("Package name (e.g. com.android.chrome)")))
		pkgName, ok := readLine(reader)
		if !ok {
			return
		}
		if pkgName == "" {
			ui.Warn("Package name is required.")
			return
		}
		ui.Prompt(fmt.Sprintf("%s %s: ", ui.White("Destination directory"), ui.Gray("[default .]")))
		destDir, ok := readLine(reader)
		if !ok {
			return
		}
		if destDir == "" {
			destDir = "."
		}
		savedPath, err := apk.PullPackage(context.Background(), client, serial, pkgName, destDir)
		if err != nil {
			ui.Error("%v", err)
			return
		}
		ui.Success("APK extracted to: %s", savedPath)
	}
}
