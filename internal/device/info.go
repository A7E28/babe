package device

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrDeviceUnauthorized = errors.New("unauthorized: accept the USB debugging prompt on the device screen")
	ErrDeviceOffline      = errors.New("offline: reconnect USB cable or run 'adb reconnect'")
	ErrDeviceNotReady     = errors.New("not ready for standard ADB commands")
	ErrNoDevicesFound     = errors.New("no connected ADB devices detected")
	ErrDeviceNotFound     = errors.New("target ADB device not found")
)

type Info struct {
	Serial          string `json:"serial"`
	Model           string `json:"model,omitempty"`
	Brand           string `json:"brand,omitempty"`
	Manufacturer    string `json:"manufacturer,omitempty"`
	AndroidVersion  string `json:"android_version,omitempty"`
	SDKLevel        int    `json:"sdk_level,omitempty"`
	BuildID         string `json:"build_id,omitempty"`
	SecurityPatch   string `json:"security_patch,omitempty"`
	Architecture    string `json:"architecture,omitempty"`
	BatteryLevel    int    `json:"battery_level"`
	BatteryCharging bool   `json:"battery_charging"`
	IsRooted        bool   `json:"is_rooted"`
}

func ValidateDeviceState(d Device) error {
	switch d.State {
	case StateDevice:
		return nil
	case StateUnauthorized:
		return fmt.Errorf("device %s is %w", d.Serial, ErrDeviceUnauthorized)
	case StateOffline:
		return fmt.Errorf("device %s is %w", d.Serial, ErrDeviceOffline)
	default:
		return fmt.Errorf("device %s is in %q state (%w)", d.Serial, d.State, ErrDeviceNotReady)
	}
}

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
				if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
					if lvl, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
						info.BatteryLevel = lvl
					}
				}
			} else if strings.HasPrefix(line, "status:") {
				if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
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
	for _, line := range strings.Split(output, "\n") {
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

func (c *Client) Reboot(ctx context.Context, serial string, target RebootTarget) error {
	args := []string{"reboot"}
	if target != RebootSystem {
		args = append(args, string(target))
	}

	if _, err := c.ExecuteOnDevice(ctx, serial, args...); err != nil {
		return fmt.Errorf("reboot target %s: %w", target, err)
	}
	return nil
}
