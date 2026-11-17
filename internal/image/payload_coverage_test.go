package image

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPayload_CoverageGapsAndBackslashes(t *testing.T) {
	dir := t.TempDir()

	rootSysZip := writeTestOTAZip(t, dir, "root_super.zip", nil, zip.Deflate, map[string][]byte{
		"super.img": bytes.Repeat([]byte{0x11}, 4096),
	})
	if info, err := InspectPayload(rootSysZip); err != nil || len(info.Partitions) != 1 || info.Partitions[0].Name != "super" {
		t.Fatalf("expected root super.img detection, got info=%+v err=%v", info, err)
	}

	if res, err := ExtractPayload(context.Background(), rootSysZip, PayloadExtractOptions{
		OutputDir: filepath.Join(dir, "raw_nv"), Partitions: []string{"super"}, Verify: false,
	}); err != nil || res.Extracted[0].SHA256 != "" {
		t.Fatalf("expected empty SHA256 on raw-img with Verify=false, got err=%v res=%+v", err, res)
	}

	bsZip := writeTestOTAZip(t, dir, "backslash.zip", nil, zip.Deflate, map[string][]byte{
		`META-INF\com\android\metadata`: []byte("ota-type=BLOCK\npre-device=lynx\n"),
		`images\boot.img`:               bytes.Repeat([]byte{0x22}, 4096),
		`vendor.transfer.list`:          []byte("4\n1\n0\n0\nnew 2,0,1\ncompute_fec 2,1,2\n"),
		`vendor.new.dat`:                bytes.Repeat([]byte{0x33}, 4096),
	})
	bsInfo, err := InspectPayload(bsZip)
	if err != nil || len(bsInfo.Partitions) != 2 || bsInfo.OTAMetadata["pre-device"] != "lynx" {
		t.Fatalf("backslash zip inspect failed: err=%v info=%+v", err, bsInfo)
	}
	if _, err := ExtractPayload(context.Background(), bsZip, PayloadExtractOptions{
		OutputDir: filepath.Join(dir, "bs_out"), All: true, Verify: true,
	}); err != nil {
		t.Fatalf("backslash + compute_fec extract failed: %v", err)
	}

	incOnlyZip := writeTestOTAZip(t, dir, "inc_only.zip", nil, zip.Deflate, map[string][]byte{
		"system.transfer.list": []byte("4\n1\n0\n0\nmove 2,0,1\n"),
		"system.patch.dat":     nil,
	})
	incInfo, err := InspectPayload(incOnlyZip)
	if err != nil || incInfo.OTAType != "delta" || len(incInfo.Partitions) != 1 || incInfo.Partitions[0].Extractable {
		t.Fatalf("expected incremental transfer.list to appear as delta, got err=%v info=%+v", err, incInfo)
	}

	badCases := map[string][]byte{
		"bad_total":    []byte("4\nnope\n0\n0\n"),
		"bad_v2_hdr":   []byte("4\n1\nnope\n0\n"),
		"trunc_v2_hdr": []byte("4\n1\n0\n"),
		"overlap_new":  []byte("4\n1\n0\n0\nnew 2,0,10000000\nnew 2,0,10000000\n"),
	}
	for name, tl := range badCases {
		zp := writeTestOTAZip(t, dir, name+".zip", nil, zip.Deflate, map[string][]byte{
			"system.transfer.list": tl,
			"system.new.dat":       make([]byte, 4096),
		})
		if _, err := InspectPayload(zp); err == nil {
			t.Errorf("expected error for %s transfer.list, got nil", name)
		}
	}

	if _, _, err := extractLegacyRawImg(context.Background(), &zip.File{FileHeader: zip.FileHeader{Name: "huge.img"}}, math.MaxInt64, nil, true, nil); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected raw-img size limit error, got: %v", err)
	}
}

func TestPayload_ProtoHashCapMergeAndCaseCollision(t *testing.T) {
	dir := t.TempDir()

	var infoBuf bytes.Buffer
	writeProtoBytesField(&infoBuf, 2, make([]byte, 128))
	if err := decodePartitionInfoInto(infoBuf.Bytes(), &partitionInfo{}); err == nil {
		t.Fatalf("expected hash length > 64 rejection")
	}

	// Repeated tag 7 must merge fields rather than overwriting earlier size/hash.
	var pUpd, p7a, p7b bytes.Buffer
	writeProtoVarintField(&p7a, 1, 4096)
	writeProtoBytesField(&p7b, 2, bytes.Repeat([]byte{0xAA}, 32))
	writeProtoBytesField(&pUpd, 1, []byte("boot"))
	writeProtoBytesField(&pUpd, 7, p7a.Bytes())
	writeProtoBytesField(&pUpd, 7, p7b.Bytes())
	pu, err := decodePartitionUpdate(pUpd.Bytes(), false)
	if err != nil || pu.NewInfo.Size != 4096 || len(pu.NewInfo.SHA256) != 32 {
		t.Fatalf("expected merged NewInfo, got err=%v info=%+v", err, pu.NewInfo)
	}

	colBin := filepath.Join(dir, "collide.bin")
	_ = os.WriteFile(colBin, buildTestPayloadBytes(t, 4096, 0, "", []testPartSpec{
		{name: "boot", size: 4096, ops: []testOpSpec{{opType: OpZero, dstExtents: []extent{{StartBlock: 0, NumBlocks: 1}}}}},
		{name: "BOOT", size: 4096, ops: []testOpSpec{{opType: OpZero, dstExtents: []extent{{StartBlock: 0, NumBlocks: 1}}}}},
	}), 0644)
	if _, err := ExtractPayload(context.Background(), colBin, PayloadExtractOptions{OutputDir: dir, All: true}); err == nil || !strings.Contains(err.Error(), "case-colliding") {
		t.Fatalf("expected case-colliding error, got: %v", err)
	}

	ovlBin := filepath.Join(dir, "overlap.bin")
	zeroBlock := make([]byte, 4096)
	zeroSum := sha256.Sum256(zeroBlock)
	rawBlock := bytes.Repeat([]byte{0xAA}, 4096)
	rawHash := sha256.Sum256(rawBlock)

	var ext0, op0, op1, pI, pU, man bytes.Buffer
	writeProtoVarintField(&ext0, 1, 0)
	writeProtoVarintField(&ext0, 2, 1)
	writeProtoVarintField(&op0, 1, uint64(OpReplace))
	writeProtoVarintField(&op0, 2, 0)
	writeProtoVarintField(&op0, 3, 4096)
	writeProtoBytesField(&op0, 6, ext0.Bytes())
	writeProtoBytesField(&op0, 8, rawHash[:])
	writeProtoVarintField(&op1, 1, uint64(OpZero))
	writeProtoBytesField(&op1, 6, ext0.Bytes())
	writeProtoVarintField(&pI, 1, 4096)
	writeProtoBytesField(&pI, 2, zeroSum[:])
	writeProtoBytesField(&pU, 1, []byte("ovl"))
	writeProtoBytesField(&pU, 7, pI.Bytes())
	writeProtoBytesField(&pU, 8, op0.Bytes())
	writeProtoBytesField(&pU, 8, op1.Bytes())
	writeProtoVarintField(&man, 3, 4096)
	writeProtoBytesField(&man, 13, pU.Bytes())

	var ovlBuf bytes.Buffer
	ovlBuf.WriteString(PayloadMagic)
	_ = binary.Write(&ovlBuf, binary.BigEndian, uint64(PayloadFormatVersion))
	_ = binary.Write(&ovlBuf, binary.BigEndian, uint64(man.Len()))
	_ = binary.Write(&ovlBuf, binary.BigEndian, uint32(0))
	ovlBuf.Write(man.Bytes())
	ovlBuf.Write(rawBlock)
	_ = os.WriteFile(ovlBin, ovlBuf.Bytes(), 0644)

	sess, err := OpenPayload(ovlBin)
	if err != nil {
		t.Fatalf("OpenPayload failed: %v", err)
	}
	defer sess.Close()
	if len(sess.Info().Partitions) != 1 {
		t.Fatalf("unexpected session info: %+v", sess.Info())
	}
	ovlRes, err := sess.Extract(context.Background(), PayloadExtractOptions{
		OutputDir: filepath.Join(dir, "ovl_out"), Partitions: []string{"ovl"}, Verify: true,
	})
	if err != nil || !ovlRes.Extracted[0].Verified {
		t.Fatalf("overlapping ordered extract failed: err=%v res=%+v", err, ovlRes)
	}

	tmp := filepath.Join(dir, "a.tmp")
	dest := filepath.Join(dir, "b.img")
	_ = os.WriteFile(tmp, []byte("new"), 0644)
	_ = os.WriteFile(dest, []byte("old"), 0644)
	if err := commitTempFile(tmp, dest, true); err == nil {
		t.Fatalf("expected atomic commitTempFile no-clobber error")
	}

	if !hasOverlappingExtents([]installOp{{
		DstExtents: []extent{{StartBlock: ^uint64(0) - 5, NumBlocks: 10}},
	}}) {
		t.Errorf("expected uint64 wraparound extent to trigger ordered/safe path")
	}
}
