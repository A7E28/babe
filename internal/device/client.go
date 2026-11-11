package device

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// State represents the status of an attached Android device.
type State string

const (
	StateDevice       State = "device"
	StateUnauthorized State = "unauthorized"
	StateOffline      State = "offline"
	StateBootloader   State = "bootloader"
	StateRecovery     State = "recovery"
	StateUnknown      State = "unknown"
)

// Device represents an attached physical device or emulator.
type Device struct {
	Serial string
	State  State
	Model  string
}

// Client wraps ADB execution with context timeouts and safe argument handling.
type Client struct {
	adbBinary string
}

// NewClient creates an ADB client, searching system PATH and default managed locations.
func NewClient() *Client {
	return &Client{adbBinary: resolveADBBinary("adb")}
}

// SetBinaryPath allows overriding the adb executable path.
func (c *Client) SetBinaryPath(path string) {
	c.adbBinary = path
}

// BinaryPath returns the currently configured or resolved ADB executable path.
func (c *Client) BinaryPath() string {
	c.adbBinary = resolveADBBinary(c.adbBinary)
	return c.adbBinary
}

func resolveADBBinary(current string) string {
	if current != "" && current != "adb" {
		if _, err := os.Stat(current); err == nil {
			return current
		}
	}
	if p, err := exec.LookPath("adb"); err == nil {
		return p
	}

	binName := "adb"
	if runtime.GOOS == "windows" {
		binName = "adb.exe"
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
	return "adb"
}

// Execute runs an ADB command with the given arguments and returns trimmed stdout.
func (c *Client) Execute(ctx context.Context, args ...string) (string, error) {
	out, err := c.executeBytes(ctx, args...)
	return strings.TrimSpace(string(out)), err
}

// ExecuteOnDevice runs an ADB command specifically targeted at a device by serial number.
func (c *Client) ExecuteOnDevice(ctx context.Context, serial string, args ...string) (string, error) {
	return c.Execute(ctx, append([]string{"-s", serial}, args...)...)
}

// ExecOutOnDevice executes `adb -s <serial> exec-out <args...>` and returns raw binary stdout.
func (c *Client) ExecOutOnDevice(ctx context.Context, serial string, args ...string) ([]byte, error) {
	return c.executeBytes(ctx, append([]string{"-s", serial, "exec-out"}, args...)...)
}

func (c *Client) executeBytes(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.BinaryPath(), args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errOutput := strings.TrimSpace(stderr.String()); errOutput != "" {
			return nil, fmt.Errorf("adb error (%w): %s", err, errOutput)
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

// InstallationDetails contains information about the discovered ADB binary.
type InstallationDetails struct {
	Installed  bool
	Version    string
	BinaryPath string
	RawOutput  string
}

// ParseADBVersionOutput extracts the revision version and binary path from adb version output.
func ParseADBVersionOutput(output string) (version string, binaryPath string) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Version ") {
			v := strings.TrimPrefix(line, "Version ")
			if idx := strings.Index(v, "-"); idx != -1 {
				v = v[:idx]
			}
			version = strings.TrimSpace(v)
		} else if strings.HasPrefix(line, "Installed as ") {
			binaryPath = strings.TrimSpace(strings.TrimPrefix(line, "Installed as "))
		}
	}
	return version, binaryPath
}

// GetInstallationDetails checks if adb is accessible and resolves its version and binary location.
func (c *Client) GetInstallationDetails(ctx context.Context) InstallationDetails {
	output, err := c.Execute(ctx, "version")
	if err != nil {
		return InstallationDetails{Installed: false}
	}
	ver, binPath := ParseADBVersionOutput(output)
	if binPath == "" {
		if p, err := exec.LookPath(c.adbBinary); err == nil {
			binPath = p
		}
	}
	return InstallationDetails{Installed: true, Version: ver, BinaryPath: binPath, RawOutput: output}
}

// CheckInstallation verifies if adb is accessible and returns its version string.
func (c *Client) CheckInstallation() (bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	details := c.GetInstallationDetails(ctx)
	return details.Installed, details.RawOutput, nil
}

// GetConnectedDevices returns all devices currently listed by `adb devices`.
func (c *Client) GetConnectedDevices(ctx context.Context) ([]Device, error) {
	output, err := c.Execute(ctx, "devices", "-l")
	if err != nil {
		return nil, err
	}
	return ParseDevicesOutput(output), nil
}

// ParseDevicesOutput parses the output of `adb devices -l`.
func ParseDevicesOutput(output string) []Device {
	var devices []Device
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices attached") || strings.HasPrefix(line, "*") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		dev := Device{Serial: fields[0], State: State(fields[1])}
		for _, field := range fields[2:] {
			if strings.HasPrefix(field, "model:") {
				dev.Model = strings.TrimPrefix(field, "model:")
				break
			}
		}
		devices = append(devices, dev)
	}
	return devices
}
