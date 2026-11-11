package tools

import (
	"testing"
)

func TestFindAssetForPlatform(t *testing.T) {
	rel := &GitHubRelease{
		TagName: "v0.2.0",
		Assets: []GitHubAsset{
			{Name: "babe-v0.2.0-windows-amd64.exe", BrowserDownloadURL: "https://example.com/win64"},
			{Name: "babe-v0.2.0-windows-arm64.exe", BrowserDownloadURL: "https://example.com/winarm64"},
			{Name: "babe-v0.2.0-linux-amd64", BrowserDownloadURL: "https://example.com/linux64"},
			{Name: "babe-v0.2.0-linux-arm64", BrowserDownloadURL: "https://example.com/linuxarm64"},
			{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/sums"},
		},
	}

	tests := []struct {
		goos     string
		goarch   string
		expected string
	}{
		{"windows", "amd64", "babe-v0.2.0-windows-amd64.exe"},
		{"windows", "arm64", "babe-v0.2.0-windows-arm64.exe"},
		{"linux", "amd64", "babe-v0.2.0-linux-amd64"},
		{"linux", "arm64", "babe-v0.2.0-linux-arm64"},
	}

	for _, tc := range tests {
		asset, err := FindAssetForPlatform(rel, tc.goos, tc.goarch)
		if err != nil {
			t.Fatalf("unexpected error for %s/%s: %v", tc.goos, tc.goarch, err)
		}
		if asset.Name != tc.expected {
			t.Errorf("for %s/%s expected %s, got %s", tc.goos, tc.goarch, tc.expected, asset.Name)
		}
	}

	_, err := FindAssetForPlatform(rel, "darwin", "arm64")
	if err == nil {
		t.Errorf("expected error for unsupported platform darwin/arm64, got nil")
	}
}

func TestParseChecksums(t *testing.T) {
	content := `
# Checksums for babe v0.2.0
a6b752eb596c319b7587c6948f6d47868d2187acb0ccd6e5c8a441c3e67d5d39  babe-v0.2.0-linux-amd64
226d7a84ac11fffc7ebb7518eb2ba5be40fa4981646760cdae78d1a3f61184cd  babe-v0.2.0-linux-arm64
ebedfb34a4fabaf8a3857e1a77360cc81a0c8f698c4b191b5e20bab7dd278ad8 *babe-v0.2.0-windows-amd64.exe
ce8c47af65151d083ad6ac4b919631e522610274cf7765bccc315eb4ebdbf9a9  babe-v0.2.0-windows-arm64.exe
`

	sums := ParseChecksums(content)
	if len(sums) != 4 {
		t.Fatalf("expected 4 checksum entries, got %d", len(sums))
	}

	expected := "ebedfb34a4fabaf8a3857e1a77360cc81a0c8f698c4b191b5e20bab7dd278ad8"
	if got := sums["babe-v0.2.0-windows-amd64.exe"]; got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}
