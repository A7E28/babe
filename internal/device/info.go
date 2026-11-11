package device

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// holds parsed device telemetry and hardware information.
type Info struct {
	Serial          string
	Model           string
	Brand           string
	Manufacturer    string
	AndroidVersion  string
	SDKLevel        int
	BuildID         string
	SecurityPatch   string
	Architecture    string
	BatteryLevel    int
	BatteryCharging bool
	IsRooted        bool
}

// checks if an attached device is ready to accept ADB commands.
func ValidateDeviceState(d Device) error {
	switch d.State {
	case StateDevice:
		return nil
	case StateUnauthorized:
		return fmt.Errorf("device %s is unauthorized: accept the USB debugging prompt on the device screen", d.Serial)
	case StateOffline:
		return fmt.Errorf("device %s is offline: reconnect USB cable or run 'adb reconnect'", d.Serial)
	default:
		return fmt.Errorf("device %s is in %q state and not ready for standard ADB commands", d.Serial, d.State)
	}
}

// queries a device for comprehensive hardware and OS properties.
func (c *Client) GetDeviceInfo(ctx context.Context, serial string) (*Info, error) {
	info := &Info{Serial: serial}

	getpropOut, err := c.ExecuteOnDevice(ctx, serial, "shell", "getprop")
	if err != nil {
		return nil, fmt.Errorf("query device properties: %w", err)
	}

	props := parseGetprop(getpropOut)
	info.Model = props["ro.product.model"]
	info.Brand = props["ro.product.brand"]
	info.Manufacturer = props["ro.product.manufacturer"]
	info.AndroidVersion = props["ro.build.version.release"]
	info.BuildID = props["ro.build.display.id"]
	info.SecurityPatch = props["ro.build.version.security_patch"]
	info.Architecture = props["ro.product.cpu.abi"]

	if sdkStr, ok := props["ro.build.version.sdk"]; ok {
		if sdk, err := strconv.Atoi(sdkStr); err == nil {
			info.SDKLevel = sdk
		}
	}

	batteryOut, err := c.ExecuteOnDevice(ctx, serial, "shell", "dumpsys", "battery")
	if err == nil {
		for _, line := range strings.Split(batteryOut, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "level:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					if lvl, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
						info.BatteryLevel = lvl
					}
				}
			} else if strings.HasPrefix(line, "status:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					st := strings.TrimSpace(parts[1])
					if st == "2" || st == "5" {
						info.BatteryCharging = true
					}
				}
			}
		}
	}

	suCheck, err := c.ExecuteOnDevice(ctx, serial, "shell", "which", "su")
	if err == nil && strings.Contains(suCheck, "su") {
		info.IsRooted = true
	}

	return info, nil
}

func parseGetprop(output string) map[string]string {
	props := make(map[string]string)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.Contains(line, "]: [") {
			parts := strings.SplitN(line, "]: [", 2)
			if len(parts) == 2 {
				key := strings.TrimPrefix(parts[0], "[")
				val := strings.TrimSuffix(parts[1], "]")
				props[key] = val
			}
		}
	}

	return props
}

type RebootTarget string

const (
	RebootSystem     RebootTarget = ""
	RebootBootloader RebootTarget = "bootloader"
	RebootRecovery   RebootTarget = "recovery"
	RebootEDL        RebootTarget = "edl"
)

// triggers a system or special mode restart on the device.
func (c *Client) Reboot(ctx context.Context, serial string, target RebootTarget) error {
	args := []string{"reboot"}
	if target != RebootSystem {
		args = append(args, string(target))
	}

	_, err := c.ExecuteOnDevice(ctx, serial, args...)
	if err != nil {
		return fmt.Errorf("reboot target %s: %w", target, err)
	}

	return nil
}
