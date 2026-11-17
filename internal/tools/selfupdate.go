package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/A7E28/babe/internal/hash"
)

const babeReleasesLatestAPI = "https://api.github.com/repos/A7E28/babe/releases/latest"

// GitHubRelease represents release metadata for the latest babe release.
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	HTMLURL     string        `json:"html_url"`
	Assets      []GitHubAsset `json:"assets"`
	PublishedAt string        `json:"published_at"`
}

// GitHubAsset represents a downloadable binary or asset in a release.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	SHA256             string `json:"sha256,omitempty"`
}

// FetchLatestBabeRelease queries latest.json on GitHub Releases without using the rate-limited REST API.
// If latest.json returns 404 and GH_TOKEN/GITHUB_TOKEN is set, it falls back once to the GitHub API.
func FetchLatestBabeRelease(ctx context.Context) (*GitHubRelease, error) {
	manifest, err := FetchLatestManifest(ctx)
	if err == nil {
		return manifestToGitHubRelease(manifest), nil
	}
	if errors.Is(err, errManifestNotFound) {
		if token := resolveGitHubToken(); token != "" {
			return fetchLatestFromGitHubAPI(ctx, babeReleasesLatestAPI, token, nil)
		}
		return nil, fmt.Errorf("fetch release metadata: %w (release may still be publishing or missing latest.json)", err)
	}
	return nil, err
}

func resolveGitHubToken() string {
	if tok := strings.TrimSpace(os.Getenv("GH_TOKEN")); tok != "" {
		return tok
	}
	return strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
}

func fetchLatestFromGitHubAPI(ctx context.Context, apiURL, token string, client *http.Client) (*GitHubRelease, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "babe-cli")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, formatHTTPError("github api", resp)
	}

	var rel GitHubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxManifestBytes)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("parse release JSON: %w", err)
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return nil, fmt.Errorf("github api response is missing tag_name")
	}
	return &rel, nil
}

// FindAssetForPlatform locates the matching binary asset for the specified OS and architecture.
func FindAssetForPlatform(release *GitHubRelease, goos, goarch string) (*GitHubAsset, error) {
	if release == nil {
		return nil, fmt.Errorf("release metadata is nil")
	}
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

	// Fallback if naming omits version tag prefix
	genericName := fmt.Sprintf("babe-%s-%s%s", goos, goarch, ext)
	for i := range release.Assets {
		if strings.EqualFold(release.Assets[i].Name, genericName) {
			return &release.Assets[i], nil
		}
	}

	return nil, fmt.Errorf("no release asset found for platform %s/%s (expected %s)", goos, goarch, expectedName)
}

// ApplyBabeUpdate validates the asset URL and SHA-256 checksum, downloads the binary,
// verifies size and hash, and atomically replaces the running executable.
func ApplyBabeUpdate(ctx context.Context, asset *GitHubAsset, expectedSHA256 string, onProgress ProgressFunc) (string, error) {
	if asset == nil {
		return "", fmt.Errorf("release asset is nil")
	}
	if err := ValidateAssetURL(asset.BrowserDownloadURL); err != nil {
		return "", fmt.Errorf("reject untrusted download URL: %w", err)
	}
	client := newValidatedHTTPClient(30*time.Minute, ValidateAssetURL)
	return applyBabeUpdateWithClient(ctx, client, asset, expectedSHA256, onProgress)
}

func applyBabeUpdateWithClient(ctx context.Context, client *http.Client, asset *GitHubAsset, expectedSHA256 string, onProgress ProgressFunc) (string, error) {
	expectedSHA256 = strings.TrimSpace(expectedSHA256)
	if !isValidSHA256(expectedSHA256) {
		return "", fmt.Errorf("valid 64-character SHA-256 checksum is required for self-update")
	}

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
		if runtime.GOOS == "linux" && errors.Is(err, os.ErrPermission) {
			return "", fmt.Errorf("permission denied creating staging file in %s (re-run with sudo or install to ~/.local/bin via 'babe setup'): %w", dir, err)
		}
		return "", fmt.Errorf("create staging file in %s: %w", dir, err)
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := DownloadFileWithClient(ctx, client, asset.BrowserDownloadURL, tempPath, onProgress); err != nil {
		return "", fmt.Errorf("download binary: %w", err)
	}

	if asset.Size > 0 {
		fi, statErr := os.Stat(tempPath)
		if statErr != nil {
			return "", fmt.Errorf("inspect downloaded binary: %w", statErr)
		}
		if fi.Size() != asset.Size {
			return "", fmt.Errorf("downloaded size mismatch: expected %d bytes, got %d", asset.Size, fi.Size())
		}
	}

	actualSHA, err := hash.CalculateFileHash(tempPath, hash.SHA256)
	if err != nil {
		return "", fmt.Errorf("calculate sha256 of downloaded binary: %w", err)
	}
	if !strings.EqualFold(actualSHA, expectedSHA256) {
		return "", fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, actualSHA)
	}

	if err := replaceExecutable(exePath, tempPath); err != nil {
		return "", err
	}

	return exePath, nil
}
