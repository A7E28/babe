package fastboot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/A7E28/babe/internal/hash"
)

var (
	ErrNoFastbootDevices = errors.New("no devices detected in Fastboot mode")
	ErrFastbootNotFound  = errors.New("target Fastboot device not found")
)

type Device struct {
	Serial string `json:"serial"`
	Mode   string `json:"mode"`
}

type DeviceInfo struct {
	Serial            string `json:"serial"`
	Product           string `json:"product,omitempty"`
	BootloaderVersion string `json:"bootloader_version,omitempty"`
	BasebandVersion   string `json:"baseband_version,omitempty"`
	SerialNumber      string `json:"serial_number,omitempty"`
	Secure            string `json:"secure,omitempty"`
	Unlocked          string `json:"unlocked,omitempty"`
	CurrentSlot       string `json:"current_slot,omitempty"`
	SlotCount         string `json:"slot_count,omitempty"`
	MaxDownloadSize   string `json:"max_download_size,omitempty"`
}

type Client struct {
	mu             sync.RWMutex
	fastbootBinary string
}

func NewClient() *Client {
	return &Client{fastbootBinary: resolveFastbootBinary("fastboot")}
}

func (c *Client) SetBinaryPath(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fastbootBinary = path
}

func (c *Client) BinaryPath() string {
	c.mu.RLock()
	cur := c.fastbootBinary
	c.mu.RUnlock()

	resolved := resolveFastbootBinary(cur)
	if resolved != cur {
		c.mu.Lock()
		if c.fastbootBinary == cur {
			c.fastbootBinary = resolved
		}
		cur = c.fastbootBinary
		c.mu.Unlock()
		return cur
	}
	return resolved
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

func (c *Client) Execute(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.BinaryPath(), args...)
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	if err := cmd.Run(); err != nil {
		if out := strings.TrimSpace(combined.String()); out != "" {
			return out, fmt.Errorf("fastboot error (%w): %s", err, out)
		}
		return "", err
	}
	return strings.TrimSpace(combined.String()), nil
}

func (c *Client) ExecuteOnDevice(ctx context.Context, serial string, args ...string) (string, error) {
	if serial != "" {
		args = append([]string{"-s", serial}, args...)
	}
	return c.Execute(ctx, args...)
}

func (c *Client) GetConnectedDevices(ctx context.Context) ([]Device, error) {
	out, err := c.Execute(ctx, "devices")
	if err != nil {
		return nil, err
	}
	return ParseDevicesOutput(out), nil
}

func ParseDevicesOutput(output string) []Device {
	var devices []Device
	for _, line := range strings.Split(output, "\n") {
		if fields := strings.Fields(strings.TrimSpace(line)); len(fields) >= 2 {
			devices = append(devices, Device{Serial: fields[0], Mode: fields[1]})
		}
	}
	return devices
}

func (c *Client) GetDeviceInfo(ctx context.Context, serial string) (*DeviceInfo, error) {
	out, err := c.ExecuteOnDevice(ctx, serial, "getvar", "all")
	if err != nil && out == "" {
		return nil, fmt.Errorf("query fastboot variables: %w", err)
	}
	return ParseGetvarAll(out, serial), nil
}

func ParseGetvarAll(output, serial string) *DeviceInfo {
	vars := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "(bootloader)"))
		if idx := strings.LastIndex(line, ":"); idx != -1 {
			k := strings.ToLower(strings.TrimSpace(line[:idx]))
			v := strings.TrimSpace(line[idx+1:])
			if k != "" && v != "" {
				vars[k] = v
			}
		}
	}
	return &DeviceInfo{
		Serial:            serial,
		Product:           vars["product"],
		BootloaderVersion: vars["version-bootloader"],
		BasebandVersion:   vars["version-baseband"],
		SerialNumber:      vars["serialno"],
		Secure:            vars["secure"],
		Unlocked:          vars["unlocked"],
		CurrentSlot:       vars["current-slot"],
		SlotCount:         vars["slot-count"],
		MaxDownloadSize:   vars["max-download-size"],
	}
}

func (c *Client) SetActiveSlot(ctx context.Context, serial, slot string) error {
	clean := strings.ToLower(strings.TrimSpace(slot))
	if clean != "a" && clean != "b" {
		return fmt.Errorf("invalid boot slot %q: must be 'a' or 'b'", slot)
	}
	_, err := c.ExecuteOnDevice(ctx, serial, "--set-active="+clean)
	return err
}

// FlashPartition flashes imagePath to partition and returns the image's SHA-256 hex digest.
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
