package apk

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/A7E28/babe/internal/device"
)

// InstallOptions contains options for APK installation.
type InstallOptions struct {
	Reinstall bool // -r
	Downgrade bool // -d
	GrantPerm bool // -g
}

// Install installs an APK file onto the targeted Android device.
func Install(ctx context.Context, client *device.Client, serial, apkPath string, opts InstallOptions) error {
	args := []string{"install"}
	if opts.Reinstall {
		args = append(args, "-r")
	}
	if opts.Downgrade {
		args = append(args, "-d")
	}
	if opts.GrantPerm {
		args = append(args, "-g")
	}
	args = append(args, apkPath)

	out, err := client.ExecuteOnDevice(ctx, serial, args...)
	if err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	if !strings.Contains(out, "Success") {
		return fmt.Errorf("installation failed: %s", out)
	}

	return nil
}

// ListPackages returns the installed packages on the device.
func ListPackages(ctx context.Context, client *device.Client, serial string, thirdPartyOnly bool) ([]string, error) {
	args := []string{"shell", "pm", "list", "packages"}
	if thirdPartyOnly {
		args = append(args, "-3")
	}

	out, err := client.ExecuteOnDevice(ctx, serial, args...)
	if err != nil {
		return nil, fmt.Errorf("list packages: %w", err)
	}

	return ParsePackageList(out), nil
}

// ParsePackageList extracts package names from `pm list packages` output.
func ParsePackageList(out string) []string {
	var packages []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package:") {
			pkg := strings.TrimSpace(strings.TrimPrefix(line, "package:"))
			if pkg != "" {
				packages = append(packages, pkg)
			}
		}
	}
	return packages
}

// PullPackage extracts an installed package from the device into destDir.
// Standard single-APK apps are saved as <package>.apk; Split APK apps are bundled into <package>.apks.
func PullPackage(ctx context.Context, client *device.Client, serial, packageName, destDir string) (string, error) {
	out, err := client.ExecuteOnDevice(ctx, serial, "shell", "pm", "path", packageName)
	if err != nil {
		return "", fmt.Errorf("find apk path: %w", err)
	}

	devicePaths := ParsePackagePaths(out)
	if len(devicePaths) == 0 {
		return "", fmt.Errorf("package %s not found on device", packageName)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("create destination directory %s: %w", destDir, err)
	}

	if len(devicePaths) == 1 {
		localFile := filepath.Join(destDir, packageName+".apk")
		if _, err := client.ExecuteOnDevice(ctx, serial, "pull", devicePaths[0], localFile); err != nil {
			return "", fmt.Errorf("pull apk from %s: %w", devicePaths[0], err)
		}
		return localFile, nil
	}

	tempDir, err := os.MkdirTemp("", "babe-pull-splits-*")
	if err != nil {
		return "", fmt.Errorf("create temp staging directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	var localSplits []string
	for i, devPath := range devicePaths {
		baseName := path.Base(devPath)
		if baseName == "" || baseName == "." || baseName == "/" {
			baseName = fmt.Sprintf("split_%d.apk", i)
		}
		localSplit := filepath.Join(tempDir, baseName)
		if _, err := os.Stat(localSplit); err == nil {
			localSplit = filepath.Join(tempDir, fmt.Sprintf("split_%d_%s", i, baseName))
		}
		if _, err := client.ExecuteOnDevice(ctx, serial, "pull", devPath, localSplit); err != nil {
			return "", fmt.Errorf("pull split apk %s: %w", devPath, err)
		}
		localSplits = append(localSplits, localSplit)
	}

	bundleFile := filepath.Join(destDir, packageName+".apks")
	if err := PackSplitBundle(localSplits, bundleFile); err != nil {
		return "", fmt.Errorf("pack split apks into %s: %w", bundleFile, err)
	}

	return bundleFile, nil
}

// ParsePackagePaths extracts all APK device paths from `pm path <pkg>` output.
func ParsePackagePaths(out string) []string {
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package:") {
			p := strings.TrimSpace(strings.TrimPrefix(line, "package:"))
			if p != "" {
				paths = append(paths, p)
			}
		}
	}
	return paths
}

// ParsePackagePath extracts the primary APK device path from `pm path <pkg>` output.
func ParsePackagePath(out string) string {
	paths := ParsePackagePaths(out)
	if len(paths) > 0 {
		return paths[0]
	}
	return ""
}
