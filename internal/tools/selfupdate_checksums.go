package tools

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchReleaseChecksums retrieves and parses the checksums.txt file from the release.
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
	if err := ValidateAssetURL(checksumAsset.BrowserDownloadURL); err != nil {
		return nil, fmt.Errorf("untrusted checksums URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumAsset.BrowserDownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create checksum request: %w", err)
	}
	req.Header.Set("User-Agent", "babe-cli")

	client := newValidatedHTTPClient(15*time.Second, ValidateAssetURL)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch checksums returned HTTP %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxManifestBytes))
	if err != nil {
		return nil, fmt.Errorf("read checksums: %w", err)
	}

	return ParseChecksums(string(body)), nil
}

// ParseChecksums parses standard sha256sum format lines: "<hash>  <filename>".
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
			fileName = strings.TrimPrefix(fileName, "./")
			results[fileName] = hashVal
		}
	}
	return results
}

// LookupChecksum resolves an asset checksum, normalizing leading "./" and falling back to case-insensitive matching.
func LookupChecksum(checksums map[string]string, assetName string) (string, bool) {
	target := strings.TrimPrefix(strings.TrimSpace(assetName), "./")
	if target == "" {
		return "", false
	}
	if sum, ok := checksums[target]; ok {
		return sum, true
	}
	for k, v := range checksums {
		if strings.EqualFold(strings.TrimPrefix(k, "./"), target) {
			return v, true
		}
	}
	return "", false
}

func isValidSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
