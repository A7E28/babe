package main

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/tools"
)

func collectToolsStatus(client *device.Client, pm platform.PlatformManager) (*toolsStatusReport, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	details := client.GetInstallationDetails(ctx)
	installDir, err := pm.GetDefaultInstallDir()
	inPath := false
	if err == nil {
		inPath, _ = pm.IsInPath(installDir)
	}

	babeStatus, err := tools.GetBabeInstallStatus(pm)
	if err != nil {
		return nil, fmt.Errorf("inspect babe installation status: %w", err)
	}

	return &toolsStatusReport{
		PlatformTools: platformToolsStatus{
			Installed:  details.Installed,
			Version:    details.Version,
			BinaryPath: details.BinaryPath,
			InstallDir: installDir,
			InPath:     inPath,
		},
		Babe: babeStatus,
	}, nil
}

func executeToolsUpdate(client *device.Client, force, checkOnly bool, onProgress func(cur, tot int64)) (*updateReport, error) {
	ctx := context.Background()
	checkCtx, cancelCheck := context.WithTimeout(ctx, 3*time.Second)
	details := client.GetInstallationDetails(checkCtx)
	cancelCheck()

	netCtx, cancelNet := context.WithTimeout(ctx, 10*time.Second)
	release, err := tools.FetchLatestRelease(netCtx)
	cancelNet()

	installed := details.Installed
	report := &updateReport{
		Target:         "tools",
		Installed:      &installed,
		CurrentVersion: details.Version,
		Path:           details.BinaryPath,
	}
	if release != nil {
		report.LatestVersion = release.Version
		if !details.Installed || tools.CompareVersions(details.Version, release.Version) < 0 {
			report.UpdateAvailable = true
		}
	}

	if checkOnly {
		if err != nil {
			return nil, fmt.Errorf("check remote repository: %w", err)
		}
		return report, nil
	}

	if err != nil {
		if details.Installed && !force {
			report.Warning = fmt.Sprintf("could not check remote repository: %v", err)
			return report, nil
		}
		report.Warning = fmt.Sprintf("could not query release metadata: %v", err)
	}

	if details.Installed && release != nil && !force && !report.UpdateAvailable {
		return report, nil
	}

	downloadURL, expectedChecksum := "", ""
	if release != nil {
		downloadURL = release.ZipURL
		expectedChecksum = release.Checksum
	}

	configuredPath, err := tools.DownloadAndConfigurePlatformTools(ctx, downloadURL, expectedChecksum, onProgress)
	if err != nil {
		return nil, fmt.Errorf("download platform-tools: %w", err)
	}

	client.SetBinaryPath("adb")
	report.Updated = true
	report.Path = configuredPath
	return report, nil
}

func executeSelfUpdate(force, checkOnly bool, onStart func(latest, asset string), onProgress func(cur, tot int64)) (*updateReport, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	release, err := tools.FetchLatestBabeRelease(ctx)
	if err != nil {
		return nil, fmt.Errorf("check for updates: %w", err)
	}
	asset, err := tools.FindAssetForPlatform(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, err
	}

	cmp := tools.CompareVersions(version, release.TagName)
	report := &updateReport{
		Target:          "self",
		CurrentVersion:  version,
		LatestVersion:   release.TagName,
		Asset:           asset.Name,
		UpdateAvailable: cmp < 0 || version == "dev",
	}

	if checkOnly || (!report.UpdateAvailable && !force) {
		return report, nil
	}
	if onStart != nil {
		onStart(release.TagName, asset.Name)
	}

	checksums, err := tools.FetchReleaseChecksums(ctx, release)
	expectedSHA := ""
	if err != nil {
		report.Warning = fmt.Sprintf("could not fetch release checksums: %v", err)
	} else {
		expectedSHA = checksums[asset.Name]
	}
	report.SHA256 = expectedSHA

	appliedPath, err := tools.ApplyBabeUpdate(ctx, asset, expectedSHA, onProgress)
	if err != nil {
		return nil, fmt.Errorf("self-update failed: %w", err)
	}

	report.Updated = true
	report.Path = appliedPath
	return report, nil
}

func handleToolsStatusJSON(cmd *cobra.Command, client *device.Client, pm platform.PlatformManager) error {
	report, err := collectToolsStatus(client, pm)
	if err != nil {
		return err
	}
	return writeJSON(cmd.OutOrStdout(), report)
}

func handleSetupJSON(cmd *cobra.Command, pm platform.PlatformManager, force, statusOnly bool) error {
	if statusOnly {
		status, err := tools.GetBabeInstallStatus(pm)
		if err != nil {
			return fmt.Errorf("inspect installation status: %w", err)
		}
		return writeJSON(cmd.OutOrStdout(), status)
	}

	res, err := tools.SetupBabe(pm, force)
	if err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}
	return writeJSON(cmd.OutOrStdout(), res)
}

func handleUpdateToolsJSON(cmd *cobra.Command, client *device.Client, force, checkOnly bool) error {
	report, err := executeToolsUpdate(client, force, checkOnly, nil)
	if err != nil {
		return err
	}
	return writeJSON(cmd.OutOrStdout(), report)
}

func handleSelfUpdateJSON(cmd *cobra.Command, force, checkOnly bool) error {
	report, err := executeSelfUpdate(force, checkOnly, nil, nil)
	if err != nil {
		return err
	}
	return writeJSON(cmd.OutOrStdout(), report)
}
