package main

import (
	"bufio"
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/A7E28/babe/internal/tools"
	"github.com/A7E28/babe/internal/ui"
)

func runSelfUpdate(force, checkOnly bool) {
	ui.Section("babe Self-Update")
	fmt.Println("Checking for latest release on GitHub...")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	release, err := tools.FetchLatestBabeRelease(ctx)
	if err != nil {
		ui.Error("Failed to check for updates: %v", err)
		return
	}

	asset, err := tools.FindAssetForPlatform(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		ui.Error("%v", err)
		return
	}

	cmp := tools.CompareVersions(version, release.TagName)
	if checkOnly {
		if cmp < 0 || version == "dev" {
			ui.Info("Update available: %s -> %s", version, release.TagName)
			ui.Info("Run 'babe update self' to install.")
		} else {
			ui.Success("babe is up to date (%s).", version)
		}
		return
	}

	if cmp >= 0 && version != "dev" && !force {
		ui.Success("babe is already up to date: %s", version)
		ui.Info("Latest release: %s", release.TagName)
		return
	}

	fmt.Printf("Updating babe %s -> %s (%s)...\n", version, release.TagName, asset.Name)
	checksums, err := tools.FetchReleaseChecksums(ctx, release)
	expectedSHA := ""
	if err != nil {
		ui.Warn("Could not fetch release checksums: %v", err)
	} else {
		expectedSHA = checksums[asset.Name]
		if expectedSHA != "" {
			ui.Info("Verified SHA256: %s", expectedSHA)
		}
	}

	var lastPct int64 = -1
	appliedPath, err := tools.ApplyBabeUpdate(ctx, asset, expectedSHA, func(cur, tot int64) {
		if tot > 0 {
			pct := (cur * 100) / tot
			if pct != lastPct {
				lastPct = pct
				fmt.Printf("\rDownloading: %3d%% [%d / %d KB]", pct, cur/1024, tot/1024)
			}
		} else {
			fmt.Printf("\rDownloading: %d KB", cur/1024)
		}
	})
	fmt.Println()

	if err != nil {
		ui.Error("Self-update failed: %v", err)
		return
	}

	ui.Success("Successfully updated babe to %s!", release.TagName)
	ui.Info("Binary updated at: %s", appliedPath)
}

func runSelfUpdateInteractive(reader *bufio.Reader) {
	ui.Section("babe Self-Update")
	fmt.Println("Checking for latest release on GitHub...")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	release, err := tools.FetchLatestBabeRelease(ctx)
	if err != nil {
		ui.Error("Failed to check for updates: %v", err)
		return
	}

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
		ans, _ := reader.ReadString('\n')
		if !strings.EqualFold(strings.TrimSpace(ans), "y") {
			return
		}
		force = true
	} else {
		ui.Info("Current version: %s -> Latest version: %s (%s)", version, release.TagName, asset.Name)
		ui.Prompt(fmt.Sprintf("\n%s ", ui.White("Download and install this update now? [Y/n]:")))
		ans, _ := reader.ReadString('\n')
		ans = strings.TrimSpace(ans)
		if ans != "" && !strings.EqualFold(ans, "y") {
			return
		}
	}

	runSelfUpdate(force, false)
}
