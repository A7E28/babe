package image

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
)

func TestPayload_LegacyNonABZipInspectAndExtract(t *testing.T) {
	dir := t.TempDir()

	bootImg := make([]byte, 4096)
	copy(bootImg[:8], []byte(BootMagic))
	binary.LittleEndian.PutUint32(bootImg[40:44], 2)
	copy(bootImg[48:], []byte("raphael\x00"))

	sysBlock0 := bytes.Repeat([]byte{0xAA}, 4096)
	sysBlock2 := bytes.Repeat([]byte{0xBB}, 4096)
	var brBuf bytes.Buffer
	bw := brotli.NewWriter(&brBuf)
	_, _ = bw.Write(sysBlock0)
	_, _ = bw.Write(sysBlock2)
	_ = bw.Close()

	odmBlock0 := bytes.Repeat([]byte{0xCC}, 4096)

	zipPath := writeTestOTAZip(t, dir, "legacy_full.zip", nil, zip.Deflate, map[string][]byte{
		"META-INF/com/android/metadata": []byte("ota-type=BLOCK\npre-device=raphael\npost-security-patch-level=2026-11-01\n"),
		"boot.img":                      bootImg,
		"system.new.dat.br":             brBuf.Bytes(),
		"system.transfer.list":          []byte("4\n3\n0\n0\nnew 2,0,1\nzero 2,1,2\nnew 2,2,3\n"),
		"odm.new.dat":                   odmBlock0,
		"odm.transfer.list":             []byte("4\n1\n0\n0\nnew 2,0,1\n"),
		"system.patch.dat":              nil,
		"unsparse_super_empty.img":      []byte("ignore-me"),
	})

	info, err := InspectPayload(zipPath)
	if err != nil {
		t.Fatalf("InspectPayload legacy failed: %v", err)
	}
	if info.SourceType != "legacy-zip" || info.OTAType != "full" || len(info.Partitions) != 3 {
		t.Fatalf("unexpected legacy payload info: %+v", info)
	}

	outDir := filepath.Join(dir, "out")
	res, err := ExtractPayload(context.Background(), zipPath, PayloadExtractOptions{
		OutputDir:   outDir,
		Partitions:  []string{"boot", "system", "odm"},
		Jobs:        3,
		Verify:      true,
		InspectBoot: true,
	})
	if err != nil {
		t.Fatalf("ExtractPayload legacy failed: %v", err)
	}
	if len(res.Extracted) != 3 || res.SourceType != "legacy-zip" {
		t.Fatalf("unexpected extract result: %+v", res)
	}
	var bootItem *PayloadExtractItem
	for i := range res.Extracted {
		if res.Extracted[i].Partition == "boot" {
			bootItem = &res.Extracted[i]
			break
		}
	}
	if bootItem == nil || bootItem.BootInfo == nil || bootItem.BootInfo.HeaderVersion != 2 {
		t.Errorf("expected boot header v2 on boot.img, got %+v", bootItem)
	}

	gotSys, err := os.ReadFile(filepath.Join(outDir, "system.img"))
	if err != nil {
		t.Fatalf("read system.img: %v", err)
	}
	wantSys := append(append(append([]byte(nil), sysBlock0...), make([]byte, 4096)...), sysBlock2...)
	if !bytes.Equal(gotSys, wantSys) {
		t.Fatalf("reconstructed system.img mismatch (len %d vs %d)", len(gotSys), len(wantSys))
	}

	gotOdm, err := os.ReadFile(filepath.Join(outDir, "odm.img"))
	if err != nil || !bytes.Equal(gotOdm, odmBlock0) {
		t.Fatalf("reconstructed odm.img mismatch: err=%v", err)
	}
}

func TestPayload_LegacyIncrementalRejection(t *testing.T) {
	dir := t.TempDir()
	zipPath := writeTestOTAZip(t, dir, "legacy_delta.zip", nil, zip.Deflate, map[string][]byte{
		"system.new.dat":       make([]byte, 4096),
		"system.patch.dat":     []byte("patch-bytes"),
		"system.transfer.list": []byte("4\n2\n0\n0\nbsdiff 0,10,0,0,0\n"),
	})

	info, err := InspectPayload(zipPath)
	if err != nil {
		t.Fatalf("InspectPayload failed: %v", err)
	}
	if info.OTAType != "delta" || info.Partitions[0].Extractable {
		t.Fatalf("expected delta unextractable partition, got %+v", info)
	}
	if _, err := ExtractPayload(context.Background(), zipPath, PayloadExtractOptions{
		OutputDir:  dir,
		Partitions: []string{"system"},
	}); err == nil || !strings.Contains(err.Error(), "incremental block OTA") {
		t.Errorf("expected incremental block OTA error, got: %v", err)
	}
}
