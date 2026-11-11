package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/A7E28/babe/internal/hash"
)

const babeReleasesLatestAPI = "https://api.github.com/repos/A7E28/babe/releases/latest"

// represents release metadata returned by the GitHub API.
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	HTMLURL     string        `json:"html_url"`
	Assets      []GitHubAsset `json:"assets"`
	PublishedAt string        `json:"published_at"`
}

// represents a downloadable binary or asset in a GitHub release.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// queries the GitHub API for the latest release of babe.
func FetchLatestBabeRelease(ctx context.Context) (*GitHubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, babeReleasesLatestAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("create release request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "babe-cli")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned HTTP %s", resp.Status)
	}

	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("parse release JSON: %w", err)
	}

	return &rel, nil
}

// locates the matching binary asset for the specified OS and architecture.
func FindAssetForPlatform(release *GitHubRelease, goos, goarch string) (*GitHubAsset, error) {
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}

	expectedName := fmt.Sprintf("babe-%s-%s-%s%s", release.TagName, goos, goarch, ext)
	for i := range release.Assets {
		if strings.EqualFold(release.Assets[i].Name, expectedName) {
			return &release.Assets[i], nil
		}
	}

	// without version tag prefix if naming differs
	genericName := fmt.Sprintf("babe-%s-%s%s", goos, goarch, ext)
	for i := range release.Assets {
		if strings.EqualFold(release.Assets[i].Name, genericName) {
			return &release.Assets[i], nil
		}
	}

	return nil, fmt.Errorf("no release asset found for platform %s/%s (expected %s)", goos, goarch, expectedName)
}

// retrieves and parses the checksums.txt file from the release.
func FetchReleaseChecksums(ctx context.Context, release *GitHubRelease) (map[string]string, error) {
	var checksumAsset *GitHubAsset
	for i := range release.Assets {
		if strings.EqualFold(release.Assets[i].Name, "checksums.txt") {
			checksumAsset = &release.Assets[i]
			break
		}
	}

	if checksumAsset == nil {
		return nil, fmt.Errorf("release %s is missing checksums.txt", release.TagName)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumAsset.BrowserDownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create checksum request: %w", err)
	}
	req.Header.Set("User-Agent", "babe-cli")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch checksums returned HTTP %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read checksums: %w", err)
	}

	return ParseChecksums(string(body)), nil
}

// parses standard sha256sum format lines: "<hash>  <filename>".
func ParseChecksums(content string) map[string]string {
	results := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			hashVal := strings.ToLower(parts[0])
			fileName := strings.TrimPrefix(parts[1], "*")
			results[fileName] = hashVal
		}
	}
	return results
}

// downloads the update, verifies its sha256 checksum, and replaces the running binary.
func ApplyBabeUpdate(ctx context.Context, asset *GitHubAsset, expectedSHA256 string, onProgress ProgressFunc) (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return "", fmt.Errorf("resolve symlink for %s: %w", exePath, err)
	}

	dir := filepath.Dir(exePath)
	tempFile, err := os.CreateTemp(dir, ".babe-update-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create staging file in %s: %w", dir, err)
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := DownloadFile(ctx, asset.BrowserDownloadURL, tempPath, onProgress); err != nil {
		return "", fmt.Errorf("download binary: %w", err)
	}

	if expectedSHA256 != "" {
		actualSHA, err := hash.CalculateFileHash(tempPath, hash.SHA256)
		if err != nil {
			return "", fmt.Errorf("calculate sha256 of downloaded binary: %w", err)
		}
		if !strings.EqualFold(actualSHA, expectedSHA256) {
			return "", fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, actualSHA)
		}
	}

	if err := replaceExecutable(exePath, tempPath); err != nil {
		return "", err
	}

	return exePath, nil
}
