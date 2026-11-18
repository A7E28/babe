package ui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/image"
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

func TestErrorTo(t *testing.T) {
	var buf bytes.Buffer
	ErrorTo(&buf, "failed %s: %d", "check", 42)
	got := buf.String()
	if !strings.Contains(got, "[ERROR]") || !strings.Contains(got, "failed check: 42") {
		t.Errorf("unexpected ErrorTo output: %q", got)
	}
}

func TestRenderPayload_SanitizationAndEmptyBootInfo(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	RenderPayloadInfoTo(&buf, &image.PayloadInfo{
		FilePath:   "ota\x1b[31m.zip",
		SourceType: "zip",
		OTAType:    "full",
		BlockSize:  4096,
		Partitions: []image.PayloadPartition{{
			Name:        "boot\x1b[2J",
			Size:        4096,
			Codecs:      []string{"xz"},
			Extractable: true,
		}},
	})
	infoOut := buf.String()
	if strings.Contains(infoOut, "\x1b") {
		t.Errorf("expected ANSI escapes stripped from payload info output, got %q", infoOut)
	}
	if !strings.Contains(infoOut, "boot[2J") {
		t.Errorf("expected sanitized partition name, got %q", infoOut)
	}

	buf.Reset()
	RenderPayloadExtractResultTo(&buf, &image.PayloadExtractResult{
		SourceType: "zip",
		Jobs:       1,
		Extracted: []image.PayloadExtractItem{{
			Partition:  "boot",
			OutputPath: "boot.img",
			Size:       4096,
			Verified:   false,
			BootInfo:   &image.BootImageInfo{HeaderVersion: 4},
		}},
	})
	extOut := buf.String()
	if strings.Contains(extOut, "(OS , Patch )") || strings.Contains(extOut, "SHA256:") {
		t.Errorf("unexpected empty OS/Patch or blank SHA256 in output: %q", extOut)
	}
	if !strings.Contains(extOut, "Boot Header: v4") {
		t.Errorf("expected 'Boot Header: v4', got %q", extOut)
	}
}

func TestRenderPayloadInfo_DynamicColumnsDeltaAndOverflow(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	RenderPayloadInfoTo(&buf, &image.PayloadInfo{
		FilePath:   "ota.zip",
		SourceType: "zip",
		OTAType:    "delta",
		BlockSize:  4096,
		TotalSize:  ^uint64(0),
		Partitions: []image.PayloadPartition{
			{
				Name:        "vendor_kernel_boot_a",
				Size:        1024 * 1024,
				Codecs:      []string{"xz"},
				Extractable: true,
			},
			{
				Name:              "system",
				Size:              2 * 1024 * 1024,
				Codecs:            []string{"zstd"},
				Extractable:       false,
				UnsupportedReason: "requires source blocks\x1b[31m (SOURCE_COPY)",
			},
		},
	})
	out := buf.String()
	if !strings.Contains(out, "(size overflow)") || strings.Contains(out, "17592186044416.00") {
		t.Errorf("expected size overflow marker, got %q", out)
	}
	if strings.Contains(out, "\x1b") {
		t.Errorf("expected ANSI escapes stripped, got %q", out)
	}
	if !strings.Contains(out, "[DELTA] requires source blocks[31m (SOURCE_COPY)") {
		t.Errorf("expected sanitized DELTA reason, got %q", out)
	}

	var header, row1, row2 string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.Contains(line, "NAME") && strings.Contains(line, "STATUS"):
			header = line
		case strings.Contains(line, "vendor_kernel_boot_a"):
			row1 = line
		case strings.Contains(line, "system"):
			row2 = line
		}
	}
	if header == "" || row1 == "" || row2 == "" {
		t.Fatalf("missing table lines in output:\n%s", out)
	}
	if strings.Index(header, "CODECS") != strings.Index(row1, "xz") || strings.Index(header, "CODECS") != strings.Index(row2, "zstd") {
		t.Errorf("CODECS column misaligned:\n%s\n%s\n%s", header, row1, row2)
	}
	if strings.Index(header, "STATUS") != strings.Index(row1, "[OK]") || strings.Index(header, "STATUS") != strings.Index(row2, "[DELTA]") {
		t.Errorf("STATUS column misaligned:\n%s\n%s\n%s", header, row1, row2)
	}
}

func TestRenderPayloadExtractResult_PartialBootInfo(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	RenderPayloadExtractResultTo(&buf, &image.PayloadExtractResult{
		SourceType: "zip",
		Jobs:       1,
		Extracted: []image.PayloadExtractItem{{
			Partition:  "boot",
			OutputPath: "boot.img",
			Size:       4096,
			Verified:   true,
			BootInfo:   &image.BootImageInfo{HeaderVersion: 4, OSVersion: "14"},
		}},
	})
	osOnly := buf.String()
	if !strings.Contains(osOnly, "Boot Header: v4 (OS 14)") || strings.Contains(osOnly, "Patch") {
		t.Errorf("unexpected OS-only Boot Header output: %q", osOnly)
	}

	buf.Reset()
	RenderPayloadExtractResultTo(&buf, &image.PayloadExtractResult{
		SourceType: "zip",
		Jobs:       1,
		Extracted: []image.PayloadExtractItem{{
			Partition:  "init_boot",
			OutputPath: "init_boot.img",
			Size:       4096,
			Verified:   true,
			BootInfo:   &image.BootImageInfo{HeaderVersion: 3, OSPatchLevel: "2026-05"},
		}},
	})
	patchOnly := buf.String()
	if !strings.Contains(patchOnly, "Boot Header: v3 (Patch 2026-05)") || strings.Contains(patchOnly, "OS ") {
		t.Errorf("unexpected Patch-only Boot Header output: %q", patchOnly)
	}
}
