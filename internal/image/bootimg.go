package image

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

const BootMagic = "ANDROID!"

// holds metadata extracted from an Android boot.img or init_boot.img header.
type BootImageInfo struct {
	FilePath      string
	FileSize      int64
	HeaderVersion uint32
	PageSize      uint32
	KernelSize    uint32
	RamdiskSize   uint32
	SecondSize    uint32
	OSVersion     string
	OSPatchLevel  string
	BoardName     string
	Cmdline       string
}

// parses an Android boot.img or init_boot.img file header (versions 0 to 4).
func InspectBootImage(filePath string) (*BootImageInfo, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open image file: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat image file: %w", err)
	}

	info, err := ParseBootImageHeader(f)
	if err != nil {
		return nil, err
	}
	info.FilePath = filePath
	info.FileSize = fi.Size()
	return info, nil
}

// reads and decodes the binary Android boot image header from r.
func ParseBootImageHeader(r io.Reader) (*BootImageInfo, error) {
	buf := make([]byte, 2048)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("read boot image header: %w", err)
	}

	if string(buf[:8]) != BootMagic {
		return nil, fmt.Errorf("invalid boot image magic (expected %q)", BootMagic)
	}

	headerVer := binary.LittleEndian.Uint32(buf[40:44])

	info := &BootImageInfo{
		HeaderVersion: headerVer,
	}

	if headerVer >= 3 {
		// Boot image header v3 / v4 layout:
		// [0..8] magic, [8..12] kernel_size, [12..16] ramdisk_size, [16..20] os_version,
		// [20..24] header_size, [24..40] reserved, [40..44] header_version, [44..1580] cmdline
		info.KernelSize = binary.LittleEndian.Uint32(buf[8:12])
		info.RamdiskSize = binary.LittleEndian.Uint32(buf[12:16])
		osRaw := binary.LittleEndian.Uint32(buf[16:20])
		info.OSVersion, info.OSPatchLevel = decodeOSVersion(osRaw)
		info.PageSize = 4096
		info.Cmdline = cleanNullTerminated(buf[44 : 44+1536])
	} else {
		// Boot image header v0 / v1 / v2 layout:
		// [0..8] magic, [8..12] kernel_size, [12..16] kernel_addr, [16..20] ramdisk_size,
		// [20..24] ramdisk_addr, [24..28] second_size, [28..32] second_addr, [32..36] tags_addr,
		// [36..40] page_size, [40..44] header_version, [44..48] os_version, [48..64] name, [64..576] cmdline
		info.KernelSize = binary.LittleEndian.Uint32(buf[8:12])
		info.RamdiskSize = binary.LittleEndian.Uint32(buf[16:20])
		info.SecondSize = binary.LittleEndian.Uint32(buf[24:28])
		info.PageSize = binary.LittleEndian.Uint32(buf[36:40])
		osRaw := binary.LittleEndian.Uint32(buf[44:48])
		info.OSVersion, info.OSPatchLevel = decodeOSVersion(osRaw)
		info.BoardName = cleanNullTerminated(buf[48:64])
		info.Cmdline = cleanNullTerminated(buf[64 : 64+512])
	}

	return info, nil
}

// unpacks the 32-bit Android boot image os_version field:
// top 21 bits: A (7 bits), B (7 bits), C (7 bits).
// bottom 11 bits: Y (7 bits, offset from 2000), M (4 bits).
func decodeOSVersion(raw uint32) (version string, patch string) {
	if raw == 0 {
		return "", ""
	}
	v := raw >> 11
	p := raw & 0x7FF

	a := (v >> 14) & 0x7F
	b := (v >> 7) & 0x7F
	c := v & 0x7F

	if a > 0 || b > 0 || c > 0 {
		version = fmt.Sprintf("%d.%d.%d", a, b, c)
	}

	year := (p >> 4) + 2000
	month := p & 0x0F
	if month >= 1 && month <= 12 {
		patch = fmt.Sprintf("%04d-%02d", year, month)
	}
	return version, patch
}

func cleanNullTerminated(b []byte) string {
	if idx := bytes.IndexByte(b, 0); idx != -1 {
		b = b[:idx]
	}
	return strings.TrimSpace(string(b))
}
