package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractZip(t *testing.T) {
	// Create a synthetic zip in memory
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

	// Verify extracted file
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

	// Create a malicious relative path entry
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
