package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	files := []struct {
		name    string
		content string
	}{
		{"testdir/file1.txt", "hello file 1"},
		{"testdir/adb", "#!/bin/sh\necho adb"},
	}

	for _, file := range files {
		w, err := zw.Create(file.name)
		if err != nil {
			t.Fatalf("failed to create zip entry: %v", err)
		}
		if _, err := w.Write([]byte(file.content)); err != nil {
			t.Fatalf("failed to write zip entry content: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write test zip: %v", err)
	}

	destDir := filepath.Join(tmpDir, "extracted")
	if err := ExtractZip(zipPath, destDir); err != nil {
		t.Fatalf("ExtractZip failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(destDir, "testdir", "file1.txt"))
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if string(content) != "hello file 1" {
		t.Errorf("got content %q, want %q", string(content), "hello file 1")
	}
}

func TestExtractZip_ZipSlipProtection(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	w, err := zw.Create("../malicious.txt")
	if err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}
	_, _ = w.Write([]byte("danger"))
	_ = zw.Close()

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "slip.zip")
	_ = os.WriteFile(zipPath, buf.Bytes(), 0644)

	destDir := filepath.Join(tmpDir, "extracted")
	err = ExtractZip(zipPath, destDir)
	if err == nil || !errors.Is(err, ErrZipSlip) {
		t.Errorf("expected ErrZipSlip, got %v", err)
	}
}

func TestDownloadFile(t *testing.T) {
	expectedData := []byte("download payload test data")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "26")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(expectedData)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	destFile := filepath.Join(tmpDir, "downloaded.bin")

	var lastCurrent, lastTotal int64
	err := DownloadFile(context.Background(), ts.URL, destFile, func(current, total int64) {
		lastCurrent = current
		lastTotal = total
	})

	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}

	data, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if !bytes.Equal(data, expectedData) {
		t.Errorf("got data %q, want %q", data, expectedData)
	}
	if lastCurrent != int64(len(expectedData)) || lastTotal != int64(len(expectedData)) {
		t.Errorf("progress values mismatch: current=%d, total=%d", lastCurrent, lastTotal)
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"37.0.1", "37.0.1", 0},
		{"37.0.1-15733141", "37.0.1", 0},
		{"v37.0.1", "37.0.1", 0},
		{"r37.0.1", "37.0.1", 0},
		{"35.0.0", "37.0.1", -1},
		{"37.0.2", "37.0.1", 1},
		{"37.1.0", "37.0.9", 1},
		{"37.0", "37.0.0", 0},
		{"37.0", "37.0.1", -1},
		{"38.0.0", "37.9.9", 1},
	}

	for _, tt := range tests {
		got := CompareVersions(tt.v1, tt.v2)
		if got != tt.expected {
			t.Errorf("CompareVersions(%q, %q) = %d; want %d", tt.v1, tt.v2, got, tt.expected)
		}
	}
}

func TestVerifyArchiveChecksum(t *testing.T) {
	payload := []byte("platform-tools-zip-payload")
	tmpFile := filepath.Join(t.TempDir(), "archive.zip")
	if err := os.WriteFile(tmpFile, payload, 0644); err != nil {
		t.Fatalf("write temp archive: %v", err)
	}

	sum1 := sha1.Sum(payload)
	sum256 := sha256.Sum256(payload)

	if err := verifyArchiveChecksum(tmpFile, hex.EncodeToString(sum1[:])); err != nil {
		t.Errorf("valid SHA-1 failed verification: %v", err)
	}
	if err := verifyArchiveChecksum(tmpFile, hex.EncodeToString(sum256[:])); err != nil {
		t.Errorf("valid SHA-256 failed verification: %v", err)
	}

	for _, bad := range []string{"", "short", "0123456789abcdef0123456789abcdef"} {
		if err := verifyArchiveChecksum(tmpFile, bad); err == nil {
			t.Errorf("expected error for malformed checksum %q, got nil", bad)
		}
	}
	if err := verifyArchiveChecksum(tmpFile, "0000000000000000000000000000000000000000"); err == nil {
		t.Errorf("expected error for mismatched SHA-1, got nil")
	}
}
