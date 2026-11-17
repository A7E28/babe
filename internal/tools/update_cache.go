package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultUpdateCheckTTL = 12 * time.Hour

type UpdateCheckCache struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

func UpdateCheckCachePath() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache dir: %w", err)
	}
	return filepath.Join(base, "babe", "update-check.json"), nil
}

func ReadUpdateCache(path string) (*UpdateCheckCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c UpdateCheckCache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	c.LatestVersion = strings.TrimSpace(c.LatestVersion)
	if c.LatestVersion == "" || c.CheckedAt.IsZero() {
		return nil, fmt.Errorf("incomplete update cache")
	}
	return &c, nil
}

func WriteUpdateCache(path string, c UpdateCheckCache) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".update-check-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = tmp.Close()
		}
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		closed = true
		return err
	}
	closed = true
	return os.Rename(tmpPath, path)
}

// CheckForUpdateCached checks for a newer release using a local TTL cache, querying latest.json on miss/expiry with no api.github.com fallback.
func CheckForUpdateCached(ctx context.Context, currentVersion, cachePath string, ttl time.Duration) (string, bool, error) {
	client := newValidatedHTTPClient(2*time.Second, ValidateAssetURL)
	return checkForUpdateCachedWithClient(ctx, currentVersion, cachePath, babeLatestManifestURL, ttl, client)
}

func checkForUpdateCachedWithClient(ctx context.Context, currentVersion, cachePath, manifestURL string, ttl time.Duration, client *http.Client) (string, bool, error) {
	if strings.TrimSpace(currentVersion) == "" || currentVersion == "dev" {
		return "", false, nil
	}

	if cachePath != "" {
		if cached, err := ReadUpdateCache(cachePath); err == nil {
			if time.Since(cached.CheckedAt) < ttl {
				return cached.LatestVersion, CompareVersions(currentVersion, cached.LatestVersion) < 0, nil
			}
		}
	}

	manifest, err := fetchManifestWithClient(ctx, client, manifestURL)
	if err != nil {
		return "", false, err
	}

	if cachePath != "" && manifest.TagName != "" {
		_ = WriteUpdateCache(cachePath, UpdateCheckCache{
			CheckedAt:     time.Now().UTC(),
			LatestVersion: manifest.TagName,
		})
	}

	return manifest.TagName, CompareVersions(currentVersion, manifest.TagName) < 0, nil
}

func RecordLatestVersionCache(latestVersion string) {
	latestVersion = strings.TrimSpace(latestVersion)
	if latestVersion == "" {
		return
	}
	cachePath, err := UpdateCheckCachePath()
	if err != nil {
		return
	}
	_ = WriteUpdateCache(cachePath, UpdateCheckCache{
		CheckedAt:     time.Now().UTC(),
		LatestVersion: latestVersion,
	})
}
