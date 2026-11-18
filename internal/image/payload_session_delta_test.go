package image

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type nopZipCloser struct{ io.Writer }

func (nopZipCloser) Close() error { return nil }

func TestPayload_SessionCloseDeltaAndVerify(t *testing.T) {
	dir := t.TempDir()

	var pUpd, opBuf, pInfo bytes.Buffer
	writeProtoVarintField(&opBuf, 1, uint64(OpZero))
	writeProtoVarintField(&pInfo, 1, 4096)
	writeProtoBytesField(&pUpd, 1, []byte("boot"))
	writeProtoBytesField(&pUpd, 7, pInfo.Bytes())
	writeProtoBytesField(&pUpd, 8, opBuf.Bytes())
	summaryPart, err := decodePartitionUpdate(pUpd.Bytes(), true)
	if err != nil || !summaryPart.SummaryOnly || summaryPart.SummaryOpCnt != 1 || len(summaryPart.Operations) != 0 {
		t.Fatalf("unexpected summaryOnly partition: err=%v part=%+v", err, summaryPart)
	}

	var dynBuf, manBuf bytes.Buffer
	var grpBuf bytes.Buffer
	writeProtoBytesField(&grpBuf, 1, []byte("g"))
	for i := 0; i <= maxManifestPartitions; i++ {
		writeProtoBytesField(&dynBuf, 1, grpBuf.Bytes())
	}
	writeProtoVarintField(&manBuf, 3, 4096)
	writeProtoBytesField(&manBuf, 15, dynBuf.Bytes())
	if _, err := decodeManifest(manBuf.Bytes(), true); err == nil || !strings.Contains(err.Error(), "dynamic group") {
		t.Fatalf("expected >256 dynamic groups error, got: %v", err)
	}

	mixedPath := filepath.Join(dir, "mixed.bin")
	_ = os.WriteFile(mixedPath, buildTestPayloadBytes(t, 4096, 0, "", []testPartSpec{
		{name: "boot", size: 4096, ops: []testOpSpec{{opType: OpZero, dstExtents: []extent{{StartBlock: 0, NumBlocks: 1}}}}},
		{name: "system", size: 4096, ops: []testOpSpec{{opType: OpSourceCopy, dstExtents: []extent{{StartBlock: 0, NumBlocks: 1}}}}},
	}), 0644)
	sess, err := OpenPayload(mixedPath)
	if err != nil {
		t.Fatalf("OpenPayload mixed failed: %v", err)
	}
	mixRes, err := sess.Extract(context.Background(), PayloadExtractOptions{
		OutputDir: filepath.Join(dir, "mix_out"), All: true, Verify: false,
	})
	if err != nil || len(mixRes.Extracted) != 1 || mixRes.Extracted[0].Partition != "boot" {
		t.Fatalf("mixed --all failed: err=%v res=%+v", err, mixRes)
	}
	gotZero, err := os.ReadFile(mixRes.Extracted[0].OutputPath)
	if err != nil || len(gotZero) != 4096 || !bytes.Equal(gotZero, make([]byte, 4096)) {
		t.Fatalf("sequential Verify=false OpZero output mismatch: err=%v len=%d", err, len(gotZero))
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
	if _, err := sess.Extract(context.Background(), PayloadExtractOptions{OutputDir: dir, All: true}); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("expected closed session error, got: %v", err)
	}

	deltaPath := filepath.Join(dir, "delta.bin")
	_ = os.WriteFile(deltaPath, buildTestPayloadBytes(t, 4096, 0, "", []testPartSpec{
		{name: "system", size: 4096, ops: []testOpSpec{{opType: OpSourceCopy, dstExtents: []extent{{StartBlock: 0, NumBlocks: 1}}}}},
	}), 0644)
	if _, err := ExtractPayload(context.Background(), deltaPath, PayloadExtractOptions{OutputDir: dir, All: true}); err == nil || !strings.Contains(err.Error(), "no extractable partitions") {
		t.Fatalf("expected pure delta --all error, got: %v", err)
	}

	rawData := bytes.Repeat([]byte{0x5A}, 4096)
	rawSum := sha256.Sum256(rawData)
	writeOneOpPayload := func(name string, opRaw []byte, blob []byte, partHash []byte) string {
		var ext, opM, pI, pU, man, out bytes.Buffer
		writeProtoVarintField(&ext, 1, 0)
		writeProtoVarintField(&ext, 2, 1)
		opM.Write(opRaw)
		writeProtoBytesField(&opM, 6, ext.Bytes())
		writeProtoVarintField(&pI, 1, 4096)
		if len(partHash) > 0 {
			writeProtoBytesField(&pI, 2, partHash)
		}
		writeProtoBytesField(&pU, 1, []byte("boot"))
		writeProtoBytesField(&pU, 7, pI.Bytes())
		writeProtoBytesField(&pU, 8, opM.Bytes())
		writeProtoVarintField(&man, 3, 4096)
		writeProtoBytesField(&man, 13, pU.Bytes())
		out.WriteString(PayloadMagic)
		_ = binary.Write(&out, binary.BigEndian, uint64(PayloadFormatVersion))
		_ = binary.Write(&out, binary.BigEndian, uint64(man.Len()))
		_ = binary.Write(&out, binary.BigEndian, uint32(0))
		out.Write(man.Bytes())
		out.Write(blob)
		p := filepath.Join(dir, name)
		_ = os.WriteFile(p, out.Bytes(), 0644)
		return p
	}
	var noOpHashOp bytes.Buffer
	writeProtoVarintField(&noOpHashOp, 1, uint64(OpReplace))
	writeProtoVarintField(&noOpHashOp, 2, 0)
	writeProtoVarintField(&noOpHashOp, 3, uint64(len(rawData)))
	noOpHashBin := writeOneOpPayload("no_op_hash.bin", noOpHashOp.Bytes(), rawData, rawSum[:])
	if _, err := ExtractPayload(context.Background(), noOpHashBin, PayloadExtractOptions{OutputDir: filepath.Join(dir, "noh"), All: true, Verify: true}); err == nil || !strings.Contains(err.Error(), "missing required SHA-256") {
		t.Fatalf("expected missing op SHA-256 error, got: %v", err)
	}
	var zeroOp bytes.Buffer
	writeProtoVarintField(&zeroOp, 1, uint64(OpZero))
	noPartHashBin := writeOneOpPayload("no_part_hash.bin", zeroOp.Bytes(), nil, nil)
	if _, err := ExtractPayload(context.Background(), noPartHashBin, PayloadExtractOptions{OutputDir: filepath.Join(dir, "nph"), All: true, Verify: true}); err == nil || !strings.Contains(err.Error(), "missing expected partition SHA-256") {
		t.Fatalf("expected missing partition SHA-256 error, got: %v", err)
	}

	if isSequentialPartition([]installOp{{DstExtents: []extent{{StartBlock: 0, NumBlocks: 1}}}, {DstExtents: []extent{{StartBlock: ^uint64(0), NumBlocks: 2}}}}) {
		t.Fatalf("expected wraparound extent to return false in isSequentialPartition")
	}
}

func TestPayload_LegacyTransferListAndMethods(t *testing.T) {
	dir := t.TempDir()

	longErase := "erase 2200000," + strings.Repeat("0,1,", 1_099_999) + "0,1\n"
	longTL := "4\n1\n0\n0\n" + longErase + "new\t2,0,1\n"
	longZip := writeTestOTAZip(t, dir, "long_tl.zip", nil, zip.Deflate, map[string][]byte{
		"boot.transfer.list": []byte(longTL),
		"boot.new.dat":       bytes.Repeat([]byte{0x77}, 4096),
	})
	if _, err := ExtractPayload(context.Background(), longZip, PayloadExtractOptions{
		OutputDir: filepath.Join(dir, "long_out"), All: true, Verify: true,
	}); err != nil {
		t.Fatalf("long/tab transfer.list extract failed: %v", err)
	}

	hugeTL := make([]byte, maxTransferListBytes+10)
	copy(hugeTL, "4\n1\n0\n0\n")
	hugeZip := writeTestOTAZip(t, dir, "huge_tl.zip", nil, zip.Deflate, map[string][]byte{
		"boot.transfer.list": hugeTL,
		"boot.new.dat":       make([]byte, 4096),
	})
	if _, err := InspectPayload(hugeZip); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected >32 MiB transfer.list error, got: %v", err)
	}

	methodZipPath := filepath.Join(dir, "method99.zip")
	f, err := os.Create(methodZipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	zw.RegisterCompressor(99, func(w io.Writer) (io.WriteCloser, error) { return nopZipCloser{w}, nil })
	w1, _ := zw.CreateHeader(&zip.FileHeader{Name: "dtbo.img", Method: zip.Store})
	_, _ = w1.Write(bytes.Repeat([]byte{0x12}, 4096))
	w2, _ := zw.CreateHeader(&zip.FileHeader{Name: "super_empty.img", Method: zip.Store})
	_, _ = w2.Write(make([]byte, 4096))
	w3, _ := zw.CreateHeader(&zip.FileHeader{Name: "images/vendor.img", Method: 99})
	_, _ = w3.Write(make([]byte, 4096))
	_ = zw.Close()
	_ = f.Close()

	mInfo, err := InspectPayload(methodZipPath)
	if err != nil || len(mInfo.Partitions) != 2 {
		t.Fatalf("expected 2 partitions (dtbo + vendor), got err=%v info=%+v", err, mInfo)
	}
	var dtboOK, vendorBlocked bool
	for _, p := range mInfo.Partitions {
		if p.Name == "dtbo" && p.Extractable {
			dtboOK = true
		}
		if p.Name == "vendor" && !p.Extractable {
			vendorBlocked = true
		}
	}
	if !dtboOK || !vendorBlocked {
		t.Fatalf("unexpected method99 partition extractability: %+v", mInfo.Partitions)
	}

	shortZipPath := writeTestOTAZip(t, dir, "short.zip", nil, zip.Store, map[string][]byte{
		"dtbo.img": make([]byte, 128),
	})
	zr, err := zip.OpenReader(shortZipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	outF, _ := os.CreateTemp(dir, "raw.*.tmp")
	defer outF.Close()
	if _, _, err := extractLegacyRawImg(context.Background(), zr.File[0], 4096, outF, false, make([]byte, 4096)); err == nil || !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("expected size mismatch error, got: %v", err)
	}
}

func TestPayload_Bzip2AndCancel(t *testing.T) {
	dir := t.TempDir()
	outSub := filepath.Join(dir, "cancel_out")
	bzRaw := make([]byte, 4096)
	copy(bzRaw, "BABE")
	bzBlob, _ := hex.DecodeString("425a68393141592653599f6e33c10000234400c04032000008200020aa0683008130a54e99a1e2ee48a70a1213edc67820")
	bzBin := filepath.Join(dir, "bz.bin")
	_ = os.WriteFile(bzBin, buildTestPayloadBytes(t, 4096, 0, "", []testPartSpec{{
		name: "dtbo", size: 4096,
		ops: []testOpSpec{{opType: OpReplaceBZ, rawData: bzRaw, bzBlob: bzBlob, dstExtents: []extent{{StartBlock: 0, NumBlocks: 1}}}},
	}}), 0644)

	res, err := ExtractPayload(context.Background(), bzBin, PayloadExtractOptions{OutputDir: filepath.Join(dir, "bz_out"), Partitions: []string{"dtbo"}, Verify: true})
	if err != nil || !res.Extracted[0].Verified {
		t.Fatalf("bzip2 extraction failed: %v", err)
	}
	gotBZ, _ := os.ReadFile(res.Extracted[0].OutputPath)
	if !bytes.Equal(gotBZ, bzRaw) {
		t.Errorf("bzip2 output mismatch")
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ExtractPayload(cancelCtx, bzBin, PayloadExtractOptions{OutputDir: outSub, Partitions: []string{"dtbo"}, Verify: true}); err == nil {
		t.Errorf("expected error on cancelled context")
	}
}
