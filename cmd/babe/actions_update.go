package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/tools"
	"github.com/A7E28/babe/internal/ui"
)

func runBabeSetup(w io.Writer, pm platform.PlatformManager, force, statusOnly bool) error {
	if w == nil {
		w = os.Stdout
	}

	if statusOnly {
		status, err := tools.GetBabeInstallStatus(pm)
		if err != nil {
			return fmt.Errorf("failed to inspect installation status: %w", err)
		}
		ui.Section("babe Environment Setup")
		fmt.Fprintf(w, "  %-18s %s\n", ui.Gray("Running Binary:"), status.CurrentExecutable)
		fmt.Fprintf(w, "  %-18s %s\n", ui.Gray("Canonical Path:"), status.CanonicalBinPath)
		if status.BinaryInstalled && status.UpToDate {
			ui.Success("Canonical binary is installed and matches running version.")
		} else if status.BinaryInstalled {
			ui.Warn("Canonical binary exists but differs from running binary (run 'babe setup' to sync).")
		} else {
			ui.Warn("babe is not yet installed in canonical directory.")
		}
		if status.InPath {
			ui.Success("Directory %s is registered in user PATH.", status.CanonicalBinDir)
		} else {
			ui.Warn("Directory %s is not in user PATH (run 'babe setup').", status.CanonicalBinDir)
		}
		return nil
	}

	res, err := tools.SetupBabe(pm, force)
	if err != nil {
		if res != nil && res.BinaryUpdated {
			return fmt.Errorf("installed babe binary at %s, but setup failed: %w", res.TargetBinPath, err)
		}
		return fmt.Errorf("setup failed: %w", err)
	}

	ui.Section("babe Environment Setup")
	if res.AlreadyInstalled && res.PathAlreadySet {
		ui.Success("babe is already installed and configured in PATH.")
		ui.Info("Location: %s", res.TargetBinPath)
		return nil
	}

	if res.BinaryUpdated {
		ui.Success("Installed babe binary to: %s", res.TargetBinPath)
	} else {
		ui.Info("Binary already up to date at: %s", res.TargetBinPath)
	}

	if !res.PathAlreadySet && res.PathConfigured {
		ui.Success("Registered %s in user PATH.", res.TargetBinDir)
		ui.Info("Open a new terminal window to invoke 'babe' directly from any directory.")
	} else if res.PathAlreadySet {
		ui.Success("User PATH already contains %s.", res.TargetBinDir)
	}
	return nil
}

func runSelfUpdate(w io.Writer, force, checkOnly bool) error {
	return runSelfUpdateWithRelease(w, nil, nil, force, checkOnly)
}

func runSelfUpdateWithRelease(w io.Writer, release *tools.GitHubRelease, asset *tools.GitHubAsset, force, checkOnly bool) error {
	if w == nil {
		w = os.Stdout
	}
	if release == nil {
		ui.Section("babe Self-Update")
		fmt.Fprintln(w, "Checking for latest release on GitHub...")
	}

	var lastPct int64 = -1
	var startedDownload bool
	report, err := executeSelfUpdateWithRelease(release, asset, force, checkOnly, func(latest, assetName string) {
		fmt.Fprintf(w, "Updating babe %s -> %s (%s)...\n", version, latest, assetName)
	}, func(cur, tot int64) {
		startedDownload = true
		if tot > 0 {
			if pct := (cur * 100) / tot; pct != lastPct {
				lastPct = pct
				fmt.Fprintf(w, "\rDownloading: %3d%% [%d / %d KB]", pct, cur/1024, tot/1024)
			}
		} else {
			fmt.Fprintf(w, "\rDownloading: %d KB", cur/1024)
		}
	})
	if startedDownload {
		fmt.Fprintln(w)
	}
	if err != nil {
		return err
	}

	if checkOnly {
		if report.UpdateAvailable {
			ui.Info("Update available: %s -> %s", report.CurrentVersion, report.LatestVersion)
			ui.Info("Run 'babe update self' to install.")
		} else {
			ui.Success("babe is up to date (%s).", report.CurrentVersion)
		}
		return nil
	}

	if !report.UpdateAvailable && !force {
		ui.Success("babe is already up to date: %s", report.CurrentVersion)
		ui.Info("Latest release: %s", report.LatestVersion)
		return nil
	}

	if report.Warning != "" {
		ui.Warn("%s", report.Warning)
	} else if report.SHA256 != "" {
		ui.Info("Verified SHA256: %s", report.SHA256)
	}

	ui.Success("Successfully updated babe to %s!", report.LatestVersion)
	ui.Info("Binary updated at: %s", report.Path)
	return nil
}

func runSelfUpdateInteractive(reader *bufio.Reader) {
	ui.Section("babe Self-Update")
	fmt.Println("Checking for latest release on GitHub...")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	release, err := tools.FetchLatestBabeRelease(ctx)
	cancel()
	if err != nil {
		ui.Error("Failed to check for updates: %v", err)
		return
	}
	tools.RecordLatestVersionCache(release.TagName)

	asset, err := tools.FindAssetForPlatform(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		ui.Error("%v", err)
		return
	}

	cmp := tools.CompareVersions(version, release.TagName)
	force := false
	if cmp >= 0 && version != "dev" {
		ui.Success("babe is already up to date: %s", version)
		ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Force reinstall latest release anyway? [y/N]:")))
		ans, ok := readLine(reader)
		if !ok || !strings.EqualFold(ans, "y") {
			return
		}
		force = true
	} else {
		ui.Info("Current version: %s -> Latest version: %s (%s)", version, release.TagName, asset.Name)
		ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Download and install this update now? [Y/n]:")))
		ans, ok := readLine(reader)
		if !ok || (ans != "" && !strings.EqualFold(ans, "y")) {
			return
		}
	}

	if err := runSelfUpdateWithRelease(os.Stdout, release, asset, force, false); err != nil {
		ui.Error("%v", err)
	}
}
