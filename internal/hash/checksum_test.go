package hash

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateReaderHash(t *testing.T) {
	input := []byte("hello android world")

	tests := []struct {
		name     string
		algo     Algorithm
		expected string
		wantErr  bool
	}{
		{
			name:     "MD5",
			algo:     MD5,
			expected: "bdb3121d0de8ed02af763054e6cafa72",
			wantErr:  false,
		},
		{
			name:     "SHA1",
			algo:     SHA1,
			expected: "c345154ef18cd7062738d56e76c4c01df745e822",
			wantErr:  false,
		},
		{
			name:     "SHA256",
			algo:     SHA256,
			expected: "983702e3ff358efb23e633fdbec44fd97eb43d025518c0d571844638ae0b1253",
			wantErr:  false,
		},
		{
			name:     "Invalid Algorithm",
			algo:     "SHA512",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(input)
			actual, err := CalculateReaderHash(r, tt.algo)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CalculateReaderHash() error = %v, wantErr %v", err, tt.wantErr)
			}
			if actual != tt.expected {
				t.Errorf("got %s, want %s", actual, tt.expected)
			}
		})
	}
}

func TestCalculateFileHash(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("hello android world")

	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	hash, err := CalculateFileHash(testFile, SHA256)
	if err != nil {
		t.Fatalf("CalculateFileHash failed: %v", err)
	}

	expected := "983702e3ff358efb23e633fdbec44fd97eb43d025518c0d571844638ae0b1253"
	if hash != expected {
		t.Errorf("got %s, want %s", hash, expected)
	}

	multi, err := CalculateAllFileHashes(testFile)
	if err != nil {
		t.Fatalf("CalculateAllFileHashes failed: %v", err)
	}
	if multi.MD5 != "bdb3121d0de8ed02af763054e6cafa72" {
		t.Errorf("MD5 got %s, want bdb3121d0de8ed02af763054e6cafa72", multi.MD5)
	}
	if multi.SHA1 != "c345154ef18cd7062738d56e76c4c01df745e822" {
		t.Errorf("SHA1 got %s, want c345154ef18cd7062738d56e76c4c01df745e822", multi.SHA1)
	}
	if multi.SHA256 != expected {
		t.Errorf("SHA256 got %s, want %s", multi.SHA256, expected)
	}
}
