package image

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPayload_InspectAndExtractMultiCoreZip(t *testing.T) {
	dir := t.TempDir()
	const blockSize = 4096

	var ops []testOpSpec
	var expectedBoot bytes.Buffer
	for i := 0; i < 20; i++ {
		block := make([]byte, blockSize)
		opType := OpReplaceXZ
		switch i % 4 {
		case 0:
			opType = OpReplace
		case 1:
			opType = OpReplaceXZ
		case 2:
			opType = OpReplaceZstd
		case 3:
			opType = OpZero
		}
		if opType != OpZero {
			for j := range block {
				block[j] = byte((i*31 + j) & 0xFF)
			}
			if i == 0 {
				copy(block[:8], []byte(BootMagic))
				binary.LittleEndian.PutUint32(block[40:44], 4)
			}
		}
		expectedBoot.Write(block)
		ops = append(ops, testOpSpec{
			opType:     opType,
			rawData:    block,
			dstExtents: []extent{{StartBlock: uint64(i), NumBlocks: 1}},
		})
	}

	vbmeta := bytes.Repeat([]byte{0x42}, blockSize)
	payload := buildTestPayloadBytes(t, blockSize, 0, "2026-10-05", []testPartSpec{
		{name: "boot", size: uint64(expectedBoot.Len()), ops: ops},
		{name: "vbmeta", size: uint64(len(vbmeta)), ops: []testOpSpec{{
			opType: OpReplaceZstd, rawData: vbmeta, dstExtents: []extent{{StartBlock: 0, NumBlocks: 1}},
		}}},
	})

	zipPath := writeTestOTAZip(t, dir, "ota.zip", payload, zip.Store, map[string][]byte{
		"META-INF/com/android/metadata": []byte("ota-type=AB\npre-device=lynx\npost-security-patch-level=2026-10-05\n"),
	})

	info, err := InspectPayload(zipPath)
	if err != nil {
		t.Fatalf("InspectPayload failed: %v", err)
	}
	if info.OTAType != "full" || info.SourceType != "zip" || len(info.Partitions) != 2 {
		t.Fatalf("unexpected payload info: %+v", info)
	}
	if info.OTAMetadata["pre-device"] != "lynx" {
		t.Errorf("expected pre-device=lynx, got %v", info.OTAMetadata)
	}

	outDir := filepath.Join(dir, "out")
	res, err := ExtractPayload(context.Background(), zipPath, PayloadExtractOptions{
		OutputDir:   outDir,
		Partitions:  []string{"boot"},
		Jobs:        4,
		Verify:      true,
		InspectBoot: true,
	})
	if err != nil {
		t.Fatalf("ExtractPayload failed: %v", err)
	}
	if len(res.Extracted) != 1 || !res.Extracted[0].Verified {
		t.Fatalf("unexpected extract result: %+v", res)
	}
	if res.Extracted[0].BootInfo == nil || res.Extracted[0].BootInfo.HeaderVersion != 4 {
		t.Errorf("expected BootInfo v4, got %+v", res.Extracted[0].BootInfo)
	}

	gotBytes, err := os.ReadFile(res.Extracted[0].OutputPath)
	if err != nil {
		t.Fatalf("read extracted boot.img: %v", err)
	}
	if !bytes.Equal(gotBytes, expectedBoot.Bytes()) {
		t.Fatalf("extracted boot.img bytes mismatch (len %d vs %d)", len(gotBytes), expectedBoot.Len())
	}
}

func TestPayload_RandomAccessNonSequentialExtents(t *testing.T) {
	dir := t.TempDir()
	const blockSize = 4096
	b0 := bytes.Repeat([]byte{0x11}, blockSize)
	b1 := bytes.Repeat([]byte{0x22}, blockSize)

	// Reverse extent order forces random-access WriteAt fallback.
	payload := buildTestPayloadBytes(t, blockSize, 0, "", []testPartSpec{{
		name: "init_boot",
		size: 2 * blockSize,
		ops: []testOpSpec{
			{opType: OpReplaceXZ, rawData: b1, dstExtents: []extent{{StartBlock: 1, NumBlocks: 1}}},
			{opType: OpReplace, rawData: b0, dstExtents: []extent{{StartBlock: 0, NumBlocks: 1}}},
		},
	}})

	binPath := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(binPath, payload, 0644); err != nil {
		t.Fatalf("write payload.bin: %v", err)
	}

	expected := append(append([]byte(nil), b0...), b1...)
	wantSum := sha256.Sum256(expected)
	res, err := ExtractPayload(context.Background(), binPath, PayloadExtractOptions{
		OutputDir:  filepath.Join(dir, "out"),
		Partitions: []string{"init_boot"},
		Jobs:       2,
		Verify:     false,
	})
	if err != nil {
		t.Fatalf("ExtractPayload failed: %v", err)
	}
	got, err := os.ReadFile(res.Extracted[0].OutputPath)
	if err != nil {
		t.Fatalf("read extracted init_boot.img: %v", err)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("random-access extraction bytes mismatch")
	}
	if res.Extracted[0].SHA256 != "" {
		t.Errorf("expected empty SHA256 when Verify=false, got %q", res.Extracted[0].SHA256)
	}
	if gotSum := sha256.Sum256(got); gotSum != wantSum {
		t.Errorf("got file SHA256 %x, want %x", gotSum, wantSum)
	}

	// Two extents in one op forces the random-access path.
	multiExtBin := filepath.Join(dir, "multi_ext.bin")
	_ = os.WriteFile(multiExtBin, buildTestPayloadBytes(t, blockSize, 0, "", []testPartSpec{{
		name: "vendor_boot",
		size: 2 * blockSize,
		ops: []testOpSpec{
			{opType: OpReplace, rawData: expected, dstExtents: []extent{{StartBlock: 0, NumBlocks: 1}, {StartBlock: 1, NumBlocks: 1}}},
		},
	}}), 0644)
	vRes, err := ExtractPayload(context.Background(), multiExtBin, PayloadExtractOptions{
		OutputDir:  filepath.Join(dir, "out_v"),
		Partitions: []string{"vendor_boot"},
		Jobs:       2,
		Verify:     true,
	})
	if err != nil || !vRes.Extracted[0].Verified || vRes.Extracted[0].SHA256 != hex.EncodeToString(wantSum[:]) {
		t.Fatalf("multi-extent verified extraction failed: err=%v res=%+v", err, vRes)
	}
}

func TestPayload_ErrorGuardsAndFormats(t *testing.T) {
	dir := t.TempDir()

	deltaBin := filepath.Join(dir, "delta.bin")
	deltaPayload := buildTestPayloadBytes(t, 4096, 2, "", []testPartSpec{
		{name: "system", size: 4096, ops: []testOpSpec{{opType: OpSourceBsdiff, dstExtents: []extent{{StartBlock: 0, NumBlocks: 1}}}}},
	})
	_ = os.WriteFile(deltaBin, deltaPayload, 0644)
	info, err := InspectPayload(deltaBin)
	if err != nil {
		t.Fatalf("InspectPayload delta failed: %v", err)
	}
	if info.OTAType != "delta" || info.Partitions[0].Extractable {
		t.Fatalf("expected delta unextractable partition, got %+v", info)
	}
	if _, err := ExtractPayload(context.Background(), deltaBin, PayloadExtractOptions{OutputDir: dir, Partitions: []string{"system"}, Verify: true}); err == nil || !strings.Contains(err.Error(), "SOURCE_BSDIFF") {
		t.Errorf("expected explicit SOURCE_BSDIFF error, got: %v", err)
	}

	corruptOpBin := filepath.Join(dir, "corrupt_op.bin")
	_ = os.WriteFile(corruptOpBin, buildTestPayloadBytes(t, 4096, 0, "", []testPartSpec{{
		name: "boot", size: 4096, ops: []testOpSpec{{opType: OpReplace, rawData: make([]byte, 4096), dstExtents: []extent{{StartBlock: 0, NumBlocks: 1}}, corruptOp: true}},
	}}), 0644)
	outSub := filepath.Join(dir, "corrupt_out")
	if _, err := ExtractPayload(context.Background(), corruptOpBin, PayloadExtractOptions{OutputDir: outSub, Partitions: []string{"boot"}, Verify: true}); err == nil {
		t.Errorf("expected error for corrupt op hash")
	}
	if entries, _ := os.ReadDir(outSub); len(entries) != 0 {
		t.Errorf("expected no leftover files after failed extraction, got %v", entries)
	}

	evilBin := filepath.Join(dir, "evil.bin")
	_ = os.WriteFile(evilBin, buildTestPayloadBytes(t, 4096, 0, "", []testPartSpec{{
		name: "../evil", size: 4096, ops: []testOpSpec{{opType: OpZero, dstExtents: []extent{{StartBlock: 0, NumBlocks: 1}}}},
	}}), 0644)
	if _, err := ExtractPayload(context.Background(), evilBin, PayloadExtractOptions{OutputDir: dir, All: true}); err == nil || !strings.Contains(err.Error(), "unsafe partition name") {
		t.Errorf("expected unsafe partition name error, got: %v", err)
	}

	legacyZip := writeTestOTAZip(t, dir, "legacy.zip", nil, zip.Store, map[string][]byte{
		"system.new.dat.br": []byte("br"),
		"boot.img":          []byte("ANDROID!"),
	})
	if legacyInfo, lErr := InspectPayload(legacyZip); lErr != nil || legacyInfo.SourceType != "legacy-zip" {
		t.Errorf("expected legacy-zip info, got info=%+v err=%v", legacyInfo, lErr)
	}

	odinZip := writeTestOTAZip(t, dir, "odin.zip", nil, zip.Store, map[string][]byte{
		"AP_SM999.tar.md5": []byte("tar"),
	})
	if _, err := InspectPayload(odinZip); err == nil || !strings.Contains(err.Error(), "Samsung Odin") {
		t.Errorf("expected Samsung Odin error, got: %v", err)
	}

	deflatedZip := writeTestOTAZip(t, dir, "deflated.zip", deltaPayload, zip.Deflate, nil)
	if _, err := InspectPayload(deflatedZip); err == nil || !strings.Contains(err.Error(), "Store") {
		t.Errorf("expected Store compression error, got: %v", err)
	}

	corruptImgBin := filepath.Join(dir, "corrupt_img.bin")
	_ = os.WriteFile(corruptImgBin, buildTestPayloadBytes(t, 4096, 0, "", []testPartSpec{{
		name: "boot", size: 4096, corruptImg: true,
		ops: []testOpSpec{{opType: OpZero, dstExtents: []extent{{StartBlock: 0, NumBlocks: 1}}}},
	}}), 0644)
	if _, err := ExtractPayload(context.Background(), corruptImgBin, PayloadExtractOptions{OutputDir: outSub, Partitions: []string{"boot"}, Verify: true}); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Errorf("expected partition SHA-256 mismatch, got: %v", err)
	}
	if entries, _ := os.ReadDir(outSub); len(entries) != 0 {
		t.Errorf("expected clean directory on partition hash failure, got %v", entries)
	}
}
