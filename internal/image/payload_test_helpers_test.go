package image

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

type testOpSpec struct {
	opType     PayloadOpType
	rawData    []byte
	bzBlob     []byte
	dstExtents []extent
	corruptOp  bool
}

type testPartSpec struct {
	name       string
	size       uint64
	ops        []testOpSpec
	corruptImg bool
}

func buildTestPayloadBytes(t *testing.T, blockSize uint32, minorVer uint32, secPatch string, parts []testPartSpec) []byte {
	t.Helper()
	var dataBlobs bytes.Buffer
	var partMsgs [][]byte

	for _, ps := range parts {
		var fullImg bytes.Buffer
		var opMsgs [][]byte

		for _, ospec := range ps.ops {
			var blob []byte
			switch ospec.opType {
			case OpReplace:
				blob = ospec.rawData
				fullImg.Write(ospec.rawData)
			case OpReplaceXZ:
				var xzBuf bytes.Buffer
				xw, err := xz.NewWriter(&xzBuf)
				if err != nil {
					t.Fatalf("xz.NewWriter: %v", err)
				}
				if _, err := xw.Write(ospec.rawData); err != nil {
					t.Fatalf("xz write: %v", err)
				}
				if err := xw.Close(); err != nil {
					t.Fatalf("xz close: %v", err)
				}
				blob = xzBuf.Bytes()
				fullImg.Write(ospec.rawData)
			case OpReplaceZstd:
				zw, err := zstd.NewWriter(nil)
				if err != nil {
					t.Fatalf("zstd.NewWriter: %v", err)
				}
				blob = zw.EncodeAll(ospec.rawData, nil)
				_ = zw.Close()
				fullImg.Write(ospec.rawData)
			case OpReplaceBZ:
				// ospec.rawData holds uncompressed expected bytes; ospec.bzBlob holds compressed bytes.
				blob = ospec.bzBlob
				fullImg.Write(ospec.rawData)
			case OpZero, OpDiscard:
				var extBlocks uint64
				for _, e := range ospec.dstExtents {
					extBlocks += e.NumBlocks
				}
				fullImg.Write(make([]byte, extBlocks*uint64(blockSize)))
			default:
				blob = []byte("delta-blob")
			}

			opOffset := uint64(dataBlobs.Len())
			opLen := uint64(len(blob))
			var opHash [32]byte
			if len(blob) > 0 {
				opHash = sha256.Sum256(blob)
				if ospec.corruptOp {
					opHash[0] ^= 0xFF
				}
				dataBlobs.Write(blob)
			}

			var opBuf bytes.Buffer
			writeProtoVarintField(&opBuf, 1, uint64(ospec.opType))
			if len(blob) > 0 {
				writeProtoVarintField(&opBuf, 2, opOffset)
				writeProtoVarintField(&opBuf, 3, opLen)
				writeProtoBytesField(&opBuf, 8, opHash[:])
			}
			for _, ext := range ospec.dstExtents {
				var extBuf bytes.Buffer
				writeProtoVarintField(&extBuf, 1, ext.StartBlock)
				writeProtoVarintField(&extBuf, 2, ext.NumBlocks)
				writeProtoBytesField(&opBuf, 6, extBuf.Bytes())
			}
			opMsgs = append(opMsgs, opBuf.Bytes())
		}

		imgBytes := fullImg.Bytes()
		if ps.size > 0 && uint64(len(imgBytes)) > ps.size {
			imgBytes = imgBytes[:ps.size]
		}
		imgHash := sha256.Sum256(imgBytes)
		if ps.corruptImg {
			imgHash[0] ^= 0xAA
		}

		var infoBuf bytes.Buffer
		writeProtoVarintField(&infoBuf, 1, ps.size)
		writeProtoBytesField(&infoBuf, 2, imgHash[:])

		var partBuf bytes.Buffer
		writeProtoBytesField(&partBuf, 1, []byte(ps.name))
		writeProtoBytesField(&partBuf, 7, infoBuf.Bytes())
		for _, om := range opMsgs {
			writeProtoBytesField(&partBuf, 8, om)
		}
		partMsgs = append(partMsgs, partBuf.Bytes())
	}

	var manifest bytes.Buffer
	writeProtoVarintField(&manifest, 3, uint64(blockSize))
	writeProtoVarintField(&manifest, 12, uint64(minorVer))
	for _, pm := range partMsgs {
		writeProtoBytesField(&manifest, 13, pm)
	}
	if secPatch != "" {
		writeProtoBytesField(&manifest, 18, []byte(secPatch))
	}

	var out bytes.Buffer
	out.WriteString(PayloadMagic)
	_ = binary.Write(&out, binary.BigEndian, uint64(PayloadFormatVersion))
	_ = binary.Write(&out, binary.BigEndian, uint64(manifest.Len()))
	_ = binary.Write(&out, binary.BigEndian, uint32(0))
	out.Write(manifest.Bytes())
	out.Write(dataBlobs.Bytes())
	return out.Bytes()
}

func writeTestOTAZip(t *testing.T, dir, name string, payload []byte, method uint16, extra map[string][]byte) string {
	t.Helper()
	zipPath := filepath.Join(dir, name)
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(f)
	if payload != nil {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: "payload.bin", Method: method})
		if err != nil {
			t.Fatalf("zip create payload.bin: %v", err)
		}
		if _, err := w.Write(payload); err != nil {
			t.Fatalf("zip write payload.bin: %v", err)
		}
	}
	for k, v := range extra {
		w, err := zw.Create(k)
		if err != nil {
			t.Fatalf("zip create %s: %v", k, err)
		}
		if _, err := w.Write(v); err != nil {
			t.Fatalf("zip write %s: %v", k, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("file close: %v", err)
	}
	return zipPath
}

func writeProtoVarintField(buf *bytes.Buffer, fieldNum int, val uint64) {
	var tmp [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(tmp[:], uint64(fieldNum<<3)|wireVarint)
	buf.Write(tmp[:n])
	n = binary.PutUvarint(tmp[:], val)
	buf.Write(tmp[:n])
}

func writeProtoBytesField(buf *bytes.Buffer, fieldNum int, data []byte) {
	var tmp [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(tmp[:], uint64(fieldNum<<3)|wireBytes)
	buf.Write(tmp[:n])
	n = binary.PutUvarint(tmp[:], uint64(len(data)))
	buf.Write(tmp[:n])
	buf.Write(data)
}
