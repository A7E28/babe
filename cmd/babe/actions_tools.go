package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/A7E28/babe/internal/apk"
	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/logcat"
	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/ui"
)

func logcatMenu(client *device.Client, reader *bufio.Reader) {
	ui.Section("Logcat & Crash Forensics")
	ui.RenderMenuItem(1, "Live Colored Logcat Stream")
	ui.RenderMenuItem(2, "Harvest Crash & Native Tombstone Buffer (Instant Dump)")
	ui.RenderMenuItem(3, "Cancel")
	ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Enter choice [1-3]:")))

	choiceStr, _ := reader.ReadString('\n')
	switch strings.TrimSpace(choiceStr) {
	case "1":
		startLogcatStream(client, reader)
	case "2":
		harvestCrashLogs(client, reader)
	}
}

func harvestCrashLogs(client *device.Client, reader *bufio.Reader) {
	serial, err := selectDevice(client, reader)
	if err != nil {
		ui.Warn("%v", err)
		return
	}

	ui.Prompt(fmt.Sprintf("%s %s: ", ui.White("Save crash report to file"), ui.Gray("[optional]")))
	fileInput, _ := reader.ReadString('\n')
	outFile := strings.TrimSpace(fileInput)

	ui.Divider()
	count, err := logcat.DumpCrashLogs(context.Background(), client.BinaryPath(), serial, outFile, func(entry logcat.Entry) {
		fmt.Println(logcat.Colorize(entry))
	})
	ui.Divider()

	if err != nil {
		ui.Error("Crash harvest failed: %v", err)
		return
	}
	if count == 0 {
		ui.Info("No crash entries found in device crash buffer.")
		return
	}
	ui.Success("Harvested %d crash log line(s).", count)
	if outFile != "" {
		ui.Success("Crash report saved to: %s", outFile)
	}
}

func startLogcatStream(client *device.Client, reader *bufio.Reader) {
	serial, err := selectDevice(client, reader)
	if err != nil {
		ui.Warn("%v", err)
		return
	}

	fmt.Println(ui.Gray("Select minimum log level:"))
	ui.RenderMenuItem(1, "Verbose (V)")
	ui.RenderMenuItem(2, "Debug (D)")
	ui.RenderMenuItem(3, "Info (I)")
	ui.RenderMenuItem(4, "Warning (W)")
	ui.RenderMenuItem(5, "Error (E)")
	ui.RenderMenuItem(6, "Fatal (F)")
	ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Select log level [1-6, default 1]:")))
	lvlInput, _ := reader.ReadString('\n')
	lvl := logcat.LevelVerbose
	switch strings.TrimSpace(lvlInput) {
	case "2":
		lvl = logcat.LevelDebug
	case "3":
		lvl = logcat.LevelInfo
	case "4":
		lvl = logcat.LevelWarn
	case "5":
		lvl = logcat.LevelError
	case "6":
		lvl = logcat.LevelFatal
	}

	ui.Prompt(fmt.Sprintf("%s %s: ", ui.White("Tag filter"), ui.Gray("[optional]")))
	tagInput, _ := reader.ReadString('\n')
	tagFilter := strings.TrimSpace(tagInput)

	ui.Prompt(fmt.Sprintf("%s %s: ", ui.White("Package name or PID filter"), ui.Gray("[optional]")))
	pkgInput, _ := reader.ReadString('\n')
	pkgFilter := strings.TrimSpace(pkgInput)

	ui.Prompt(fmt.Sprintf("%s %s: ", ui.White("Save output file path"), ui.Gray("[optional]")))
	fileInput, _ := reader.ReadString('\n')
	outFile := strings.TrimSpace(fileInput)

	ui.Info("Streaming logcat. Press Ctrl+C to stop...")
	ui.Divider()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := logcat.StreamOptions{
		ADBBinary:   client.BinaryPath(),
		Serial:      serial,
		MinLevel:    lvl,
		TagFilter:   tagFilter,
		PackageName: pkgFilter,
		OutputFile:  outFile,
	}

	err = logcat.Stream(ctx, opts, func(entry logcat.Entry) {
		fmt.Println(logcat.Colorize(entry))
	})

	if err != nil && ctx.Err() == nil {
		ui.Error("Logcat stream error: %v", err)
	} else {
		ui.Success("Logcat stream stopped.")
		if outFile != "" {
			ui.Success("Log output saved to: %s", outFile)
		}
	}
}

func apkSuiteMenu(client *device.Client, pm platform.PlatformManager, reader *bufio.Reader) {
	ui.Section("APK & Split-Bundle Suite")
	ui.RenderMenuItem(1, "Inspect APK / Split Bundle Offline (Manifest, SDK, ABIs, Certs)")
	ui.RenderMenuItem(2, "Install APK or Split Bundle (.apk / .apks / .xapk / .apkm)")
	ui.RenderMenuItem(3, "List Installed Packages on Device")
	ui.RenderMenuItem(4, "Pull Installed Package APK to Computer")
	ui.RenderMenuItem(5, "Cancel")
	ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Enter choice [1-5]:")))

	choiceStr, _ := reader.ReadString('\n')
	switch strings.TrimSpace(choiceStr) {
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
			ans, _ := reader.ReadString('\n')
			if strings.EqualFold(strings.TrimSpace(ans), "y") {
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
		tpStr, _ := reader.ReadString('\n')
		thirdPartyOnly := !strings.EqualFold(strings.TrimSpace(tpStr), "n")

		ui.Prompt(fmt.Sprintf("%s %s: ", ui.White("Filter substring"), ui.Gray("[optional]")))
		filterStr, _ := reader.ReadString('\n')
		filter := strings.ToLower(strings.TrimSpace(filterStr))

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
		pkgInput, _ := reader.ReadString('\n')
		pkgName := strings.TrimSpace(pkgInput)
		if pkgName == "" {
			ui.Warn("Package name is required.")
			return
		}
		ui.Prompt(fmt.Sprintf("%s %s: ", ui.White("Destination directory"), ui.Gray("[default .]")))
		dirInput, _ := reader.ReadString('\n')
		destDir := strings.TrimSpace(dirInput)
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
