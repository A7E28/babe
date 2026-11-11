package image

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func encodeOSVersion(a, b, c, year, month uint32) uint32 {
	ver := ((a & 0x7F) << 14) | ((b & 0x7F) << 7) | (c & 0x7F)
	patch := (((year - 2000) & 0x7F) << 4) | (month & 0x0F)
	return (ver << 11) | patch
}

func TestParseBootImageHeader_V4(t *testing.T) {
	buf := make([]byte, 2048)
	copy(buf[0:8], []byte("ANDROID!"))
	binary.LittleEndian.PutUint32(buf[8:12], 14500000)
	binary.LittleEndian.PutUint32(buf[12:16], 3200000)
	binary.LittleEndian.PutUint32(buf[16:20], encodeOSVersion(14, 0, 0, 2026, 5))
	binary.LittleEndian.PutUint32(buf[40:44], 4)
	copy(buf[44:], []byte("console=ttyMSM0,115200n8 androidboot.hardware=qcom\x00"))

	info, err := ParseBootImageHeader(bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("ParseBootImageHeader failed: %v", err)
	}

	if info.HeaderVersion != 4 {
		t.Errorf("got HeaderVersion %d, want 4", info.HeaderVersion)
	}
	if info.KernelSize != 14500000 {
		t.Errorf("got KernelSize %d, want 14500000", info.KernelSize)
	}
	if info.RamdiskSize != 3200000 {
		t.Errorf("got RamdiskSize %d, want 3200000", info.RamdiskSize)
	}
	if info.OSVersion != "14.0.0" {
		t.Errorf("got OSVersion %q, want 14.0.0", info.OSVersion)
	}
	if info.OSPatchLevel != "2026-05" {
		t.Errorf("got OSPatchLevel %q, want 2026-05", info.OSPatchLevel)
	}
	if info.Cmdline != "console=ttyMSM0,115200n8 androidboot.hardware=qcom" {
		t.Errorf("got Cmdline %q", info.Cmdline)
	}
}

func TestParseBootImageHeader_V2(t *testing.T) {
	buf := make([]byte, 2048)
	copy(buf[0:8], []byte("ANDROID!"))
	binary.LittleEndian.PutUint32(buf[8:12], 1024000)
	binary.LittleEndian.PutUint32(buf[16:20], 512000)
	binary.LittleEndian.PutUint32(buf[36:40], 4096)
	binary.LittleEndian.PutUint32(buf[40:44], 2)
	binary.LittleEndian.PutUint32(buf[44:48], encodeOSVersion(13, 0, 0, 2025, 11))
	copy(buf[48:64], []byte("kona\x00"))
	copy(buf[64:], []byte("buildvariant=userdebug\x00"))

	info, err := ParseBootImageHeader(bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("ParseBootImageHeader failed: %v", err)
	}

	if info.HeaderVersion != 2 || info.BoardName != "kona" || info.OSVersion != "13.0.0" || info.OSPatchLevel != "2025-11" {
		t.Errorf("unexpected v2 boot info: %+v", info)
	}
}

func TestParseBootImageHeader_InvalidMagic(t *testing.T) {
	buf := make([]byte, 2048)
	copy(buf[0:8], []byte("NOTBOOT!"))
	if _, err := ParseBootImageHeader(bytes.NewReader(buf)); err == nil {
		t.Errorf("expected error for invalid boot magic")
	}
}
