package image

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPayload_AuditRegressionGuards(t *testing.T) {
	var badManifest bytes.Buffer
	writeProtoVarintField(&badManifest, 3, uint64(1)<<32)
	if _, err := decodeManifest(badManifest.Bytes(), false); err == nil || !strings.Contains(err.Error(), "block_size") {
		t.Fatalf("expected invalid block_size error, got: %v", err)
	}

	if _, err := sumExtentBytes([]extent{{StartBlock: 0, NumBlocks: ^uint64(0) / 4096}, {StartBlock: 0, NumBlocks: 10}}, 4096); err == nil {
		t.Fatalf("expected sumExtentBytes overflow error")
	}

	pool := newPayloadCodecPool()
	defer pool.Close()
	buf, n, isZero, err := pool.decodeOp(bytes.NewReader(nil), 0, 0, 4096, installOp{
		Type:       OpZero,
		DstExtents: []extent{{StartBlock: 0, NumBlocks: 20000}},
	}, true)
	if err != nil || !isZero || n != 20000*4096 || buf != nil {
		t.Fatalf("expected >64MiB ZERO op to succeed with nil buf, got err=%v isZero=%v n=%d", err, isZero, n)
	}

	dir := t.TempDir()
	zeroBin := filepath.Join(dir, "zero_part.bin")
	zeroSum := sha256.Sum256(make([]byte, 8192))
	var infoBuf bytes.Buffer
	writeProtoVarintField(&infoBuf, 1, 8192)
	writeProtoBytesField(&infoBuf, 2, zeroSum[:])
	var partBuf bytes.Buffer
	writeProtoBytesField(&partBuf, 1, []byte("empty_zero"))
	writeProtoBytesField(&partBuf, 7, infoBuf.Bytes())
	var mBuf bytes.Buffer
	writeProtoVarintField(&mBuf, 3, 4096)
	writeProtoVarintField(&mBuf, 12, 6)
	writeProtoBytesField(&mBuf, 13, partBuf.Bytes())

	var payload bytes.Buffer
	payload.WriteString(PayloadMagic)
	_ = binary.Write(&payload, binary.BigEndian, uint64(PayloadFormatVersion))
	_ = binary.Write(&payload, binary.BigEndian, uint64(mBuf.Len()))
	_ = binary.Write(&payload, binary.BigEndian, uint32(0))
	payload.Write(mBuf.Bytes())
	_ = os.WriteFile(zeroBin, payload.Bytes(), 0644)

	info, err := InspectPayload(zeroBin)
	if err != nil {
		t.Fatalf("InspectPayload failed: %v", err)
	}
	if info.OTAType != "full" {
		t.Errorf("expected full OTAType when all ops are standalone despite MinorVersion=6, got %q", info.OTAType)
	}
	res, err := ExtractPayload(context.Background(), zeroBin, PayloadExtractOptions{
		OutputDir: filepath.Join(dir, "out"), Partitions: []string{"empty_zero"}, Verify: true,
	})
	if err != nil || !res.Extracted[0].Verified {
		t.Fatalf("extract zero-op partition failed: %+v err=%v", res, err)
	}
	if fi, _ := os.Stat(res.Extracted[0].OutputPath); fi.Size() != 8192 {
		t.Errorf("expected 8192-byte output, got %d", fi.Size())
	}

	if OpZucchini.String() != "ZUCCHINI" || OpLz4diffBsdiff.String() != "LZ4DIFF_BSDIFF" || OpLz4diffPuffdiff.String() != "LZ4DIFF_PUFFDIFF" {
		t.Errorf("unexpected op 11..13 names")
	}

	dupBin := filepath.Join(dir, "dup.bin")
	_ = os.WriteFile(dupBin, buildTestPayloadBytes(t, 4096, 0, "", []testPartSpec{
		{name: "boot", size: 4096, ops: []testOpSpec{{opType: OpZero, dstExtents: []extent{{StartBlock: 0, NumBlocks: 1}}}}},
		{name: "boot", size: 4096, ops: []testOpSpec{{opType: OpZero, dstExtents: []extent{{StartBlock: 0, NumBlocks: 1}}}}},
	}), 0644)
	if _, dErr := ExtractPayload(context.Background(), dupBin, PayloadExtractOptions{OutputDir: dir, All: true}); dErr == nil || !strings.Contains(dErr.Error(), "duplicate partition") {
		t.Fatalf("expected duplicate partition error, got: %v", dErr)
	}
}

func TestPayload_FastbootZipCaseInsensitiveAndMetadataPB(t *testing.T) {
	dir := t.TempDir()

	var dsBuf bytes.Buffer
	writeProtoBytesField(&dsBuf, 1, []byte("husky"))
	writeProtoBytesField(&dsBuf, 6, []byte("2026-11-05"))
	var metaPB bytes.Buffer
	writeProtoVarintField(&metaPB, 1, 1)
	writeProtoBytesField(&metaPB, 6, dsBuf.Bytes())

	zipPath := writeTestOTAZip(t, dir, "fastboot_images.zip", nil, zip.Deflate, map[string][]byte{
		"android-info.txt":                 []byte("require board=husky\n"),
		"META-INF/com/android/metadata.pb": metaPB.Bytes(),
		"images/boot.img":                  bytes.Repeat([]byte{0x12}, 4096),
		"firmware-update/abl.img":          bytes.Repeat([]byte{0x34}, 4096),
		"ODM.TRANSFER.LIST":                []byte("4\n1\n0\n0\nzero 0\nnew 2,0,1\n"),
		"odm.new.dat":                      bytes.Repeat([]byte{0x56}, 4096),
	})

	info, err := InspectPayload(zipPath)
	if err != nil {
		t.Fatalf("InspectPayload fastboot/case-insensitive zip failed: %v", err)
	}
	if info.SecurityPatchLevel != "2026-11-05" || info.OTAMetadata["post-device"] != "husky" {
		t.Errorf("expected metadata.pb fields, got patch=%q meta=%v", info.SecurityPatchLevel, info.OTAMetadata)
	}
	if len(info.Partitions) != 3 {
		t.Fatalf("expected 3 partitions (abl, boot, ODM), got %d: %+v", len(info.Partitions), info.Partitions)
	}

	res, err := ExtractPayload(context.Background(), zipPath, PayloadExtractOptions{
		OutputDir: filepath.Join(dir, "fb_out"), All: true, Verify: true,
	})
	if err != nil || len(res.Extracted) != 3 {
		t.Fatalf("ExtractPayload fastboot zip failed: err=%v res=%+v", err, res)
	}

	if ciRes, ciErr := ExtractPayload(context.Background(), zipPath, PayloadExtractOptions{
		OutputDir: filepath.Join(dir, "ci_out"), Partitions: []string{"odm"}, Verify: false,
	}); ciErr != nil || len(ciRes.Extracted) != 1 || ciRes.Extracted[0].SHA256 != "" {
		t.Fatalf("case-insensitive legacy extract failed: err=%v res=%+v", ciErr, ciRes)
	}

	imgOnlyZip := writeTestOTAZip(t, dir, "images_only.zip", nil, zip.Deflate, map[string][]byte{
		"images/vendor_boot.img": bytes.Repeat([]byte{0x77}, 4096),
	})
	if imgInfo, imgErr := InspectPayload(imgOnlyZip); imgErr != nil || len(imgInfo.Partitions) != 1 || imgInfo.Partitions[0].Name != "vendor_boot" {
		t.Fatalf("expected images/*.img zip to be detected, got info=%+v err=%v", imgInfo, imgErr)
	}

	badSdatZip := writeTestOTAZip(t, dir, "bad_sdat.zip", nil, zip.Deflate, map[string][]byte{
		"system.transfer.list": []byte("invalid\n1\n0\n0\n"),
		"system.new.dat":       make([]byte, 4096),
	})
	if _, bErr := InspectPayload(badSdatZip); bErr == nil || !strings.Contains(bErr.Error(), "version") {
		t.Fatalf("expected transfer.list version parse error, got: %v", bErr)
	}
}
