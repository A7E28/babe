package ui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/A7E28/babe/internal/device"
)

func captureOutput(f func()) string {
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	f()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestRenderDeviceInfo(t *testing.T) {
	info := &device.Info{
		Serial:          "TEST1234",
		Model:           "Pixel 7",
		Brand:           "Google",
		Manufacturer:    "Google",
		AndroidVersion:  "14",
		SDKLevel:        34,
		Architecture:    "arm64-v8a",
		SecurityPatch:   "2026-05-01",
		BuildID:         "UP1A.231005.007",
		BatteryLevel:    88,
		BatteryCharging: true,
		IsRooted:        false,
	}

	out := captureOutput(func() {
		RenderDeviceInfo(info)
	})

	if !strings.Contains(out, "TEST1234") {
		t.Errorf("expected serial in output, got: %s", out)
	}
	if !strings.Contains(out, "Pixel 7") {
		t.Errorf("expected model in output, got: %s", out)
	}
	if !strings.Contains(out, "Charging:") || !strings.Contains(out, "Yes") {
		t.Errorf("expected charging status in output, got: %s", out)
	}
}

func TestRenderChecksums(t *testing.T) {
	out := captureOutput(func() {
		RenderChecksums("test.apk", "md5test", "sha1test", "sha256test")
	})

	if !strings.Contains(out, "test.apk") || !strings.Contains(out, "sha256test") {
		t.Errorf("unexpected checksum output: %s", out)
	}
}

func TestRenderADBStatus(t *testing.T) {
	outInstalled := captureOutput(func() {
		RenderADBStatus(true, "37.0.1", "/usr/bin/adb", "/opt/babe", true)
	})
	if !strings.Contains(outInstalled, "ADB detected on system PATH") || !strings.Contains(outInstalled, "37.0.1") {
		t.Errorf("unexpected installed output: %s", outInstalled)
	}

	outNotInstalled := captureOutput(func() {
		RenderADBStatus(false, "", "", "/opt/babe", false)
	})
	if !strings.Contains(outNotInstalled, "ADB not found on system PATH") || !strings.Contains(outNotInstalled, "option 2") {
		t.Errorf("unexpected not installed output: %s", outNotInstalled)
	}
}
