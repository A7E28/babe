package fastboot

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/A7E28/babe/internal/hash"
)

// represents a device connected in Fastboot/Bootloader mode.
type Device struct {
	Serial string
	Mode   string
}

// holds bootloader and partition telemetry queried via `fastboot getvar all`.
type DeviceInfo struct {
	Serial            string
	Product           string
	BootloaderVersion string
	BasebandVersion   string
	SerialNumber      string
	Secure            string
	Unlocked          string
	CurrentSlot       string
	SlotCount         string
	MaxDownloadSize   string
}

// wraps fastboot binary execution and output parsing.
type Client struct {
	fastbootBinary string
}

// creates a Fastboot client, searching system PATH and managed directories.
func NewClient() *Client {
	return &Client{fastbootBinary: resolveFastbootBinary("fastboot")}
}

// returns the resolved fastboot executable path.
func (c *Client) BinaryPath() string {
	c.fastbootBinary = resolveFastbootBinary(c.fastbootBinary)
	return c.fastbootBinary
}

func resolveFastbootBinary(current string) string {
	if current != "" && current != "fastboot" {
		if _, err := os.Stat(current); err == nil {
			return current
		}
	}
	if p, err := exec.LookPath("fastboot"); err == nil {
		return p
	}

	binName := "fastboot"
	if runtime.GOOS == "windows" {
		binName = "fastboot.exe"
	}

	var candidates []string
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		candidates = append(candidates, filepath.Join(localAppData, "babe", "platform-tools", binName))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, "AppData", "Local", "babe", "platform-tools", binName),
			filepath.Join(home, ".local", "share", "babe", "platform-tools", binName),
			filepath.Join(home, ".local", "bin", binName),
		)
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return "fastboot"
}

// runs a fastboot command and returns combined stdout and stderr.
// Note: fastboot writes most telemetry (including getvar all) to stderr.
func (c *Client) Execute(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.BinaryPath(), args...)

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	if err := cmd.Run(); err != nil {
		out := strings.TrimSpace(combined.String())
		if out != "" {
			return out, fmt.Errorf("fastboot error (%w): %s", err, out)
		}
		return "", err
	}

	return strings.TrimSpace(combined.String()), nil
}

// runs a fastboot command targeting a specific device serial.
func (c *Client) ExecuteOnDevice(ctx context.Context, serial string, args ...string) (string, error) {
	if serial != "" {
		args = append([]string{"-s", serial}, args...)
	}
	return c.Execute(ctx, args...)
}

// returns all devices listed by `fastboot devices`.
func (c *Client) GetConnectedDevices(ctx context.Context) ([]Device, error) {
	out, err := c.Execute(ctx, "devices")
	if err != nil {
		return nil, err
	}
	return ParseDevicesOutput(out), nil
}

// parses `fastboot devices` output lines.
func ParseDevicesOutput(output string) []Device {
	var devices []Device
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 {
			devices = append(devices, Device{
				Serial: fields[0],
				Mode:   fields[1],
			})
		}
	}
	return devices
}

// queries `fastboot getvar all` and extracts key bootloader variables.
func (c *Client) GetDeviceInfo(ctx context.Context, serial string) (*DeviceInfo, error) {
	out, err := c.ExecuteOnDevice(ctx, serial, "getvar", "all")
	if err != nil && out == "" {
		return nil, fmt.Errorf("query fastboot variables: %w", err)
	}
	return ParseGetvarAll(out, serial), nil
}

// parses `(bootloader) key: value` lines from `fastboot getvar all`.
func ParseGetvarAll(output, serial string) *DeviceInfo {
	info := &DeviceInfo{Serial: serial}
	vars := make(map[string]string)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "(bootloader)")
		line = strings.TrimSpace(line)

		idx := strings.LastIndex(line, ":")
		if idx != -1 {
			k := strings.ToLower(strings.TrimSpace(line[:idx]))
			v := strings.TrimSpace(line[idx+1:])
			if k != "" && v != "" {
				vars[k] = v
			}
		}
	}

	info.Product = vars["product"]
	info.BootloaderVersion = vars["version-bootloader"]
	info.BasebandVersion = vars["version-baseband"]
	info.SerialNumber = vars["serialno"]
	info.Secure = vars["secure"]
	info.Unlocked = vars["unlocked"]
	info.CurrentSlot = vars["current-slot"]
	info.SlotCount = vars["slot-count"]
	info.MaxDownloadSize = vars["max-download-size"]
	return info
}

// switches the active A/B boot slot ("a" or "b").
func (c *Client) SetActiveSlot(ctx context.Context, serial, slot string) error {
	clean := strings.ToLower(strings.TrimSpace(slot))
	if clean != "a" && clean != "b" {
		return fmt.Errorf("invalid boot slot %q: must be 'a' or 'b'", slot)
	}
	_, err := c.ExecuteOnDevice(ctx, serial, "--set-active="+clean)
	return err
}

// verifies the image file, computes its SHA-256 digest, and flashes it to the target partition.
func (c *Client) FlashPartition(ctx context.Context, serial, partition, imagePath string) (string, error) {
	partition = strings.TrimSpace(partition)
	if partition == "" {
		return "", fmt.Errorf("partition name is required")
	}

	fi, err := os.Stat(imagePath)
	if err != nil {
		return "", fmt.Errorf("stat image file: %w", err)
	}
	if fi.IsDir() || fi.Size() == 0 {
		return "", fmt.Errorf("invalid or empty image file: %s", imagePath)
	}

	digest, err := hash.CalculateFileHash(imagePath, hash.SHA256)
	if err != nil {
		return "", fmt.Errorf("verify image hash: %w", err)
	}

	if _, err := c.ExecuteOnDevice(ctx, serial, "flash", partition, imagePath); err != nil {
		return digest, fmt.Errorf("flash partition %s: %w", partition, err)
	}
	return digest, nil
}
