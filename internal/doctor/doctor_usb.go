package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/fastboot"
)

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
			remedy := "Unlock phone screen, check 'Always allow from this computer', or replug USB cable"
			if errors.Is(err, device.ErrDeviceUnauthorized) {
				remedy = "Unlock phone screen and tap 'Allow' on the RSA USB debugging prompt"
			} else if errors.Is(err, device.ErrDeviceOffline) {
				remedy = "Reconnect USB cable, toggle USB debugging, or run 'adb reconnect'"
			}
			return CheckResult{
				Name:    "Connected Devices",
				Status:  StatusWarn,
				Summary: err.Error(),
				Remedy:  remedy,
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
