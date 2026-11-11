package apk

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/A7E28/babe/internal/device"
)

// contains options for APK installation.
type InstallOptions struct {
	Reinstall bool // -r
	Downgrade bool // -d
	GrantPerm bool // -g
}

// installs an APK file onto the targeted Android device.
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

// returns the installed packages on the device.
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

// extracts package names from `pm list packages` output.
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

// extracts an installed APK from the phone to a local destination directory.
func PullPackage(ctx context.Context, client *device.Client, serial, packageName, destDir string) (string, error) {
	// Find APK path on device
	out, err := client.ExecuteOnDevice(ctx, serial, "shell", "pm", "path", packageName)
	if err != nil {
		return "", fmt.Errorf("find apk path: %w", err)
	}

	devicePath := ParsePackagePath(out)
	if devicePath == "" {
		return "", fmt.Errorf("package %s not found on device", packageName)
	}

	localFile := filepath.Join(destDir, packageName+".apk")
	_, err = client.ExecuteOnDevice(ctx, serial, "pull", devicePath, localFile)
	if err != nil {
		return "", fmt.Errorf("pull apk from %s: %w", devicePath, err)
	}

	return localFile, nil
}

// extracts the primary APK device path from `pm path <pkg>` output.
func ParsePackagePath(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "package:"))
		}
	}
	return ""
}
