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

func TestPayload_TrailerAndProtobufGuards(t *testing.T) {
	bzBlob, _ := hex.DecodeString("425a68393141592653599f6e33c10000234400c04032000008200020aa0683008130a54e99a1e2ee48a70a1213edc67820")
	// Flip a trailing stream-end CRC byte while keeping the compressed payload decodable.
	corruptBZ := append([]byte(nil), bzBlob...)
	corruptBZ[len(corruptBZ)-2] ^= 0xFF
	var out [4096]byte
	pool := newPayloadCodecPool()
	defer pool.Close()
	if err := pool.decompressOp(installOp{Type: OpReplaceBZ}, corruptBZ, func() *[]byte { s := out[:]; return &s }(), 4096); err == nil {
		t.Fatalf("expected bzip2 corrupted trailer error, got nil")
	}

	// op.Type = 1<<32 must fail rather than truncating to OpReplace(0).
	var opBuf bytes.Buffer
	writeProtoVarintField(&opBuf, 1, uint64(1)<<32)
	if _, err := decodeInstallOp(opBuf.Bytes(), false); err == nil {
		t.Fatalf("expected invalid op.Type overflow error")
	}

	if err := walkProtoFields([]byte{0x00, 0x01}, func(int, int, uint64, []byte) error { return nil }); err == nil {
		t.Fatalf("expected invalid fieldNum=0 error")
	}

	for _, bad := range []string{"CON", "nul.txt", "COM1", "lpt9", "boot:evil", "bad?", "trail.", "trail "} {
		if isSafePartitionName(bad) {
			t.Errorf("expected isSafePartitionName(%q) == false", bad)
		}
	}
	for _, good := range []string{"init_boot", "boot_a", "vendor_dlkm", "odm.new"} {
		if !isSafePartitionName(good) {
			t.Errorf("expected isSafePartitionName(%q) == true", good)
		}
	}
}

func TestPayload_DiscardPartialAndNoClobber(t *testing.T) {
	dir := t.TempDir()
	// OpReplace with DstLength < extentBytes followed by OpDiscard and unaligned NewInfo.Size.
	raw := bytes.Repeat([]byte{0xAB}, 3000)
	padded := make([]byte, 4096)
	copy(padded, raw)
	fullExpected := append(padded, make([]byte, 4096)...)
	fullExpected = fullExpected[:7000]
	sum := sha256.Sum256(fullExpected)

	var ext0, ext1, op0, op1, pInfo, pUpd, man bytes.Buffer
	writeProtoVarintField(&ext0, 1, 0)
	writeProtoVarintField(&ext0, 2, 1)
	writeProtoVarintField(&ext1, 1, 1)
	writeProtoVarintField(&ext1, 2, 1)

	op0Hash := sha256.Sum256(raw)
	writeProtoVarintField(&op0, 1, uint64(OpReplace))
	writeProtoVarintField(&op0, 2, 0)
	writeProtoVarintField(&op0, 3, uint64(len(raw)))
	writeProtoBytesField(&op0, 6, ext0.Bytes())
	writeProtoVarintField(&op0, 7, uint64(len(raw)))
	writeProtoBytesField(&op0, 8, op0Hash[:])

	writeProtoVarintField(&op1, 1, uint64(OpDiscard))
	writeProtoBytesField(&op1, 6, ext1.Bytes())

	writeProtoVarintField(&pInfo, 1, 7000)
	writeProtoBytesField(&pInfo, 2, sum[:])
	writeProtoBytesField(&pUpd, 1, []byte("misc"))
	writeProtoBytesField(&pUpd, 7, pInfo.Bytes())
	writeProtoBytesField(&pUpd, 8, op0.Bytes())
	writeProtoBytesField(&pUpd, 8, op1.Bytes())

	writeProtoVarintField(&man, 3, 4096)
	writeProtoBytesField(&man, 13, pUpd.Bytes())

	var payload bytes.Buffer
	payload.WriteString(PayloadMagic)
	_ = binary.Write(&payload, binary.BigEndian, uint64(PayloadFormatVersion))
	_ = binary.Write(&payload, binary.BigEndian, uint64(man.Len()))
	_ = binary.Write(&payload, binary.BigEndian, uint32(0))
	payload.Write(man.Bytes())
	payload.Write(raw)

	binPath := filepath.Join(dir, "partial.bin")
	_ = os.WriteFile(binPath, payload.Bytes(), 0644)

	outDir := filepath.Join(dir, "out")
	res, err := ExtractPayload(context.Background(), binPath, PayloadExtractOptions{
		OutputDir: outDir, Partitions: []string{"misc"}, Verify: true,
	})
	if err != nil || !res.Extracted[0].Verified {
		t.Fatalf("partial + discard extract failed: err=%v res=%+v", err, res)
	}
	got, _ := os.ReadFile(res.Extracted[0].OutputPath)
	if !bytes.Equal(got, fullExpected) {
		t.Fatalf("expected 7000-byte unaligned output mismatch")
	}

	if _, err := ExtractPayload(context.Background(), binPath, PayloadExtractOptions{
		OutputDir: outDir, Partitions: []string{"misc"}, NoClobber: true,
	}); err == nil || !strings.Contains(err.Error(), "no-clobber") {
		t.Fatalf("expected --no-clobber error, got: %v", err)
	}

	if _, err := ExtractPayload(context.Background(), binPath, PayloadExtractOptions{
		OutputDir: outDir, All: true, Partitions: []string{"misc"},
	}); err == nil {
		t.Fatalf("expected --all + -p conflict error")
	}
	if _, err := ExtractPayload(context.Background(), binPath, PayloadExtractOptions{
		OutputDir: outDir, Partitions: []string{"misc"}, Jobs: -1,
	}); err == nil {
		t.Fatalf("expected negative jobs error")
	}
}

func TestPayload_LegacyDedupAndComputeHashTree(t *testing.T) {
	dir := t.TempDir()
	zipPath := writeTestOTAZip(t, dir, "legacy_dup.zip", nil, zip.Deflate, map[string][]byte{
		"system.transfer.list":     []byte("4\n1\n0\n0\nnew 2,0,1\ncompute_hash_tree 2,1,2\n"),
		"system.new.dat":           bytes.Repeat([]byte{0x44}, 4096),
		"sub/SYSTEM.transfer.list": []byte("4\n1\n0\n0\nnew 2,0,1\n"),
		"sub/system.new.dat":       bytes.Repeat([]byte{0x55}, 4096),
	})
	info, err := InspectPayload(zipPath)
	if err != nil {
		t.Fatalf("InspectPayload failed: %v", err)
	}
	if len(info.Partitions) != 1 || !info.Partitions[0].Extractable {
		t.Fatalf("expected 1 deduplicated extractable partition with compute_hash_tree, got %+v", info.Partitions)
	}

	res, err := ExtractPayload(context.Background(), zipPath, PayloadExtractOptions{
		OutputDir: filepath.Join(dir, "out"), Partitions: []string{"system", "SYSTEM"}, Verify: true,
	})
	if err != nil || len(res.Extracted) != 1 {
		t.Fatalf("case-insensitive deduplication failed: err=%v res=%+v", err, res)
	}
}
