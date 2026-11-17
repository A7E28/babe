package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/A7E28/babe/internal/image"
)

func buildMinimalCLIPayloadZip(t *testing.T, dir string) string {
	t.Helper()
	raw := bytes.Repeat([]byte{0x5A}, 4096)
	rawHash := sha256.Sum256(raw)

	var extBuf bytes.Buffer
	appendProtoVarint(&extBuf, 1, 0)
	appendProtoVarint(&extBuf, 2, 1)

	var opBuf bytes.Buffer
	appendProtoVarint(&opBuf, 1, 0) // OpReplace
	appendProtoVarint(&opBuf, 2, 0)
	appendProtoVarint(&opBuf, 3, uint64(len(raw)))
	appendProtoBytes(&opBuf, 6, extBuf.Bytes())
	appendProtoBytes(&opBuf, 8, rawHash[:])

	var infoBuf bytes.Buffer
	appendProtoVarint(&infoBuf, 1, uint64(len(raw)))
	appendProtoBytes(&infoBuf, 2, rawHash[:])

	var partBuf bytes.Buffer
	appendProtoBytes(&partBuf, 1, []byte("init_boot"))
	appendProtoBytes(&partBuf, 7, infoBuf.Bytes())
	appendProtoBytes(&partBuf, 8, opBuf.Bytes())

	var manifest bytes.Buffer
	appendProtoVarint(&manifest, 3, 4096)
	appendProtoBytes(&manifest, 13, partBuf.Bytes())

	var payload bytes.Buffer
	payload.WriteString("CrAU")
	_ = binary.Write(&payload, binary.BigEndian, uint64(2))
	_ = binary.Write(&payload, binary.BigEndian, uint64(manifest.Len()))
	_ = binary.Write(&payload, binary.BigEndian, uint32(0))
	payload.Write(manifest.Bytes())
	payload.Write(raw)

	zipPath := filepath.Join(dir, "ota.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "payload.bin", Method: zip.Store})
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := w.Write(payload.Bytes()); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	_ = zw.Close()
	_ = f.Close()
	return zipPath
}

func appendProtoVarint(buf *bytes.Buffer, fieldNum int, val uint64) {
	var tmp [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(tmp[:], uint64(fieldNum<<3))
	buf.Write(tmp[:n])
	n = binary.PutUvarint(tmp[:], val)
	buf.Write(tmp[:n])
}

func appendProtoBytes(buf *bytes.Buffer, fieldNum int, data []byte) {
	var tmp [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(tmp[:], uint64((fieldNum<<3)|2))
	buf.Write(tmp[:n])
	n = binary.PutUvarint(tmp[:], uint64(len(data)))
	buf.Write(tmp[:n])
	buf.Write(data)
}

func TestCLI_ImagePayloadListAndExtractJSON(t *testing.T) {
	dir := t.TempDir()
	zipPath := buildMinimalCLIPayloadZip(t, dir)

	out, errOut, err := runCLIWithBuffers("image", "payload", "list", zipPath, "--json")
	if err != nil {
		t.Fatalf("payload list --json failed: %v (stderr: %s)", err, errOut)
	}
	if errOut != "" {
		t.Errorf("expected empty stderr, got %q", errOut)
	}
	var listDecoded map[string]any
	if err := json.Unmarshal([]byte(out), &listDecoded); err != nil {
		t.Fatalf("invalid list JSON: %v\n%s", err, out)
	}
	if listDecoded["ota_type"] != "full" {
		t.Errorf("expected ota_type=full, got %v", listDecoded["ota_type"])
	}
	if _, ok := listDecoded["dynamic_groups"].([]any); !ok {
		t.Errorf("expected dynamic_groups array, got %T", listDecoded["dynamic_groups"])
	}

	outDir := filepath.Join(dir, "extracted")
	out, errOut, err = runCLIWithBuffers("image", "payload", "extract", zipPath, "-p", "init_boot", "-o", outDir, "--json")
	if err != nil {
		t.Fatalf("payload extract --json failed: %v (stderr: %s)", err, errOut)
	}
	var extDecoded map[string]any
	if err := json.Unmarshal([]byte(out), &extDecoded); err != nil {
		t.Fatalf("invalid extract JSON: %v\n%s", err, out)
	}
	items, ok := extDecoded["extracted"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 extracted item, got %v", extDecoded["extracted"])
	}
	if _, statErr := os.Stat(filepath.Join(outDir, "init_boot.img")); statErr != nil {
		t.Errorf("expected extracted init_boot.img: %v", statErr)
	}

	for _, args := range [][]string{
		{"image", "inspect", zipPath, "--json"},
		{"image", "list", zipPath, "--json"},
		{"image", zipPath, "--json"},
	} {
		shortcutOut, shortcutErr, sErr := runCLIWithBuffers(args...)
		if sErr != nil || shortcutErr != "" || !bytes.Contains([]byte(shortcutOut), []byte(`"ota_type"`)) {
			t.Fatalf("shortcut %v failed: err=%v stderr=%q out=%q", args, sErr, shortcutErr, shortcutOut)
		}
	}

	allDir := filepath.Join(dir, "all_out")
	if _, eOut, eErr := runCLIWithBuffers("image", "extract", zipPath, "-p", "all", "-o", allDir, "--json"); eErr != nil || eOut != "" {
		t.Fatalf("-p all failed: err=%v stderr=%q", eErr, eOut)
	}
	if _, _, eErr := runCLIWithBuffers("image", "extract", zipPath, "-p", "-all", "--json"); eErr == nil {
		t.Fatalf("expected error for -p -all")
	}

	humanListOut, humanListErr, hErr := runCLIWithBuffers("image", "list", zipPath)
	if hErr != nil || humanListErr != "" || !bytes.Contains([]byte(humanListOut), []byte("init_boot")) {
		t.Fatalf("human list output failed: err=%v stderr=%q out=%q", hErr, humanListErr, humanListOut)
	}
	humanExtOut, humanExtErr, heErr := runCLIWithBuffers("image", "extract", zipPath, "-p", "init_boot", "-o", filepath.Join(dir, "human_out"))
	if heErr != nil || humanExtErr != "" || !bytes.Contains([]byte(humanExtOut), []byte("[VERIFIED]")) {
		t.Fatalf("human extract output failed: err=%v stderr=%q out=%q", heErr, humanExtErr, humanExtOut)
	}

	out, errOut, err = runCLIWithBuffers("image", "payload", "extract", zipPath, "-p", "nonexistent", "--json")
	if err == nil || out != "" {
		t.Fatalf("expected error with empty stdout, got err=%v out=%q", err, out)
	}
	var errGot errorReport
	if jErr := json.Unmarshal([]byte(errOut), &errGot); jErr != nil || errGot.Error == "" {
		t.Fatalf("expected JSON error on stderr, got %q (%v)", errOut, jErr)
	}
}

func TestCLI_ImageInspectMissingPayloadFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"missing.zip", "MISSING.BIN"} {
		target := filepath.Join(dir, name)
		out, errOut, err := runCLIWithBuffers("image", "inspect", target)
		if err == nil {
			t.Fatalf("expected error for missing %s", name)
		}
		if out != "" {
			t.Errorf("expected empty stdout on error, got %q", out)
		}
		if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "open payload source") {
			t.Errorf("expected payload open error for %s, got: %v (stderr: %q)", name, err, errOut)
		}
		if strings.Contains(strings.ToLower(err.Error()), "invalid boot image magic") {
			t.Errorf("unexpected boot image magic error for %s: %v", name, err)
		}
	}

	for name, data := range map[string][]byte{
		"short.zip":   []byte("ab"),
		"corrupt.bin": []byte("NOPE_NOT_BOOT_OR_CRAU"),
	} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0644); err != nil {
			t.Fatal(err)
		}
		_, _, err := runCLIWithBuffers("image", "inspect", p)
		if err == nil || strings.Contains(strings.ToLower(err.Error()), "invalid boot image magic") {
			t.Errorf("expected payload error for %s, got: %v", name, err)
		}
	}

	outDir := filepath.Join(dir, "out")
	_ = os.MkdirAll(outDir, 0755)
	_ = os.WriteFile(filepath.Join(outDir, "system.img"), []byte("keep"), 0644)
	info := &image.PayloadInfo{
		Partitions: []image.PayloadPartition{
			{Name: "boot", Extractable: true},
			{Name: "system", Extractable: false},
		},
	}
	if hasExistingTargetImg(outDir, info, image.PayloadExtractOptions{All: true}) {
		t.Errorf("expected non-extractable delta partition not to trigger overwrite prompt")
	}
	_ = os.WriteFile(filepath.Join(outDir, "boot.img"), []byte("keep"), 0644)
	if !hasExistingTargetImg(outDir, info, image.PayloadExtractOptions{All: true}) {
		t.Errorf("expected extractable partition collision to trigger overwrite prompt")
	}
}
