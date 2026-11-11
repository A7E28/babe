package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/platform"
)

// represents the health level of a diagnostic check.
type Status string

const (
	StatusOK    Status = "OK"
	StatusWarn  Status = "WARN"
	StatusError Status = "ERROR"
)

// describes a single diagnostic inspection item.
type CheckResult struct {
	Name    string
	Status  Status
	Summary string
	Remedy  string
}

// scans a PATH string and returns all matching executable paths in resolution order.
func FindExecutablesInPath(binBaseName, pathEnv string) []string {
	binName := binBaseName
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(binName), ".exe") {
		binName += ".exe"
	}

	segments := platform.CleanPathString(pathEnv, rune(os.PathListSeparator))
	seen := make(map[string]bool)
	var found []string

	for _, dir := range segments {
		cleanDir := filepath.Clean(dir)
		key := cleanDir
		if runtime.GOOS == "windows" {
			key = strings.ToLower(cleanDir)
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		candidate := filepath.Join(cleanDir, binName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			found = append(found, candidate)
		}
	}
	return found
}

// executes a comprehensive suite of host, toolchain, PATH, and USB device checks.
func RunDiagnostics(ctx context.Context, adbClient *device.Client, fbClient *fastboot.Client, pm platform.PlatformManager) []CheckResult {
	var results []CheckResult
	pathEnv := os.Getenv("PATH")

	adbCtx, cancelADB := context.WithTimeout(ctx, 4*time.Second)
	adbDetails := adbClient.GetInstallationDetails(adbCtx)
	cancelADB()

	adbMatches := FindExecutablesInPath("adb", pathEnv)
	if !adbDetails.Installed {
		results = append(results, CheckResult{
			Name:    "ADB Toolchain",
			Status:  StatusError,
			Summary: "ADB executable not found on system PATH or managed directory",
			Remedy:  "Run 'babe tools update' to download and configure Google platform-tools",
		})
	} else if len(adbMatches) > 1 {
		results = append(results, CheckResult{
			Name:    "ADB Toolchain",
			Status:  StatusWarn,
			Summary: fmt.Sprintf("ADB v%s active (%s), but %d conflicting 'adb' binaries found in PATH", adbDetails.Version, adbDetails.BinaryPath, len(adbMatches)),
			Remedy:  fmt.Sprintf("Shadowed binary: %s — remove stale copies to prevent ADB server version mismatches", adbMatches[1]),
		})
	} else {
		results = append(results, CheckResult{
			Name:    "ADB Toolchain",
			Status:  StatusOK,
			Summary: fmt.Sprintf("ADB v%s (%s)", adbDetails.Version, adbDetails.BinaryPath),
		})
	}

	fbCtx, cancelFB := context.WithTimeout(ctx, 4*time.Second)
	fbOut, fbErr := fbClient.Execute(fbCtx, "--version")
	cancelFB()

	if fbErr != nil {
		results = append(results, CheckResult{
			Name:    "Fastboot Toolchain",
			Status:  StatusWarn,
			Summary: "Fastboot binary not detected",
			Remedy:  "Run 'babe tools update' to install the full platform-tools suite",
		})
	} else {
		firstLine := strings.TrimSpace(strings.Split(fbOut, "\n")[0])
		results = append(results, CheckResult{
			Name:    "Fastboot Toolchain",
			Status:  StatusOK,
			Summary: fmt.Sprintf("%s (%s)", firstLine, fbClient.BinaryPath()),
		})
	}

	installDir, err := pm.GetDefaultInstallDir()
	if err == nil {
		inPath, _ := pm.IsInPath(installDir)
		if inPath {
			results = append(results, CheckResult{
				Name:    "Managed PATH",
				Status:  StatusOK,
				Summary: fmt.Sprintf("Configured (%s)", installDir),
			})
		} else {
			results = append(results, CheckResult{
				Name:    "Managed PATH",
				Status:  StatusWarn,
				Summary: fmt.Sprintf("Managed directory (%s) is not registered in user PATH", installDir),
				Remedy:  "Run 'babe tools update' to register managed platform-tools in PATH",
			})
		}
	}

	results = append(results, checkBabeCLIPath(pm, pathEnv))

	results = append(results, checkOSUSBReadiness())

	results = append(results, checkConnectedDevices(ctx, adbClient, fbClient))

	return results
}

func checkBabeCLIPath(pm platform.PlatformManager, pathEnv string) CheckResult {
	binDir, err := pm.GetBabeBinDir()
	if err != nil {
		return CheckResult{Name: "babe CLI Path", Status: StatusWarn, Summary: err.Error()}
	}
	binName := "babe"
	if runtime.GOOS == "windows" {
		binName = "babe.exe"
	}
	targetBin := filepath.Join(binDir, binName)
	inPath, _ := pm.IsInPath(binDir)
	_, statErr := os.Stat(targetBin)

	if statErr != nil || !inPath {
		return CheckResult{
			Name:    "babe CLI Path",
			Status:  StatusWarn,
			Summary: fmt.Sprintf("babe is not installed globally in PATH (%s)", targetBin),
			Remedy:  "Run 'babe setup' to install babe into your user PATH",
		}
	}

	babeMatches := FindExecutablesInPath("babe", pathEnv)
	if len(babeMatches) > 1 {
		return CheckResult{
			Name:    "babe CLI Path",
			Status:  StatusWarn,
			Summary: fmt.Sprintf("Configured (%s), but %d conflicting 'babe' binaries found in PATH", targetBin, len(babeMatches)),
			Remedy:  fmt.Sprintf("Shadowed binary: %s — remove duplicate copies", babeMatches[1]),
		}
	}

	return CheckResult{
		Name:    "babe CLI Path",
		Status:  StatusOK,
		Summary: fmt.Sprintf("Configured (%s)", targetBin),
	}
}

func checkOSUSBReadiness() CheckResult {
	if runtime.GOOS == "linux" {
		const rulesPath = "/etc/udev/rules.d/51-android.rules"
		if _, err := os.Stat(rulesPath); err != nil {
			return CheckResult{
				Name:    "Linux USB udev Rules",
				Status:  StatusWarn,
				Summary: "/etc/udev/rules.d/51-android.rules is missing",
				Remedy:  "Run 'sudo babe doctor --fix-udev' to install Android USB permission rules",
			}
		}
		return CheckResult{
			Name:    "Linux USB udev Rules",
			Status:  StatusOK,
			Summary: "/etc/udev/rules.d/51-android.rules is installed",
		}
	}

	return CheckResult{
		Name:    "Host Environment",
		Status:  StatusOK,
		Summary: fmt.Sprintf("%s/%s Virtual Terminal & Win32 environment ready", runtime.GOOS, runtime.GOARCH),
	}
}

func checkConnectedDevices(ctx context.Context, adbClient *device.Client, fbClient *fastboot.Client) CheckResult {
	devCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	adbDevs, _ := adbClient.GetConnectedDevices(devCtx)
	fbDevs, _ := fbClient.GetConnectedDevices(devCtx)

	for _, d := range adbDevs {
		if err := device.ValidateDeviceState(d); err != nil {
			return CheckResult{
				Name:    "Connected Devices",
				Status:  StatusWarn,
				Summary: err.Error(),
				Remedy:  "Unlock phone screen, check 'Always allow from this computer', or replug USB cable",
			}
		}
	}

	if len(adbDevs) == 0 && len(fbDevs) == 0 {
		return CheckResult{
			Name:    "Connected Devices",
			Status:  StatusOK,
			Summary: "No devices currently attached (0 ADB, 0 Fastboot)",
		}
	}

	return CheckResult{
		Name:    "Connected Devices",
		Status:  StatusOK,
		Summary: fmt.Sprintf("%d ADB device(s), %d Fastboot device(s) ready", len(adbDevs), len(fbDevs)),
	}
}
