package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	babeLatestManifestURL = "https://github.com/A7E28/babe/releases/latest/download/latest.json"
	maxManifestBytes      = 1 << 20
)

var errManifestNotFound = errors.New("release manifest not found (HTTP 404)")

type ReleaseManifest struct {
	TagName     string                   `json:"tag_name"`
	PublishedAt string                   `json:"published_at"`
	Assets      map[string]ManifestAsset `json:"assets"`
}

type ManifestAsset struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func ValidateAssetURL(rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil {
		return fmt.Errorf("invalid download URL %q", rawURL)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("untrusted download URL scheme %q (https required)", u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "github.com":
		if !strings.HasPrefix(u.Path, "/A7E28/babe/releases/download/") &&
			!strings.HasPrefix(u.Path, "/A7E28/babe/releases/latest/download/") {
			return fmt.Errorf("untrusted GitHub release path %q", u.Path)
		}
		return nil
	case "objects.githubusercontent.com", "release-assets.githubusercontent.com":
		return nil
	default:
		return fmt.Errorf("untrusted download host %q", host)
	}
}

func newValidatedHTTPClient(timeout time.Duration, urlCheck func(string) error) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			req.Header.Del("Authorization")
			if urlCheck != nil {
				if err := urlCheck(req.URL.String()); err != nil {
					return fmt.Errorf("blocked redirect target: %w", err)
				}
			}
			return nil
		},
	}
}

func FetchLatestManifest(ctx context.Context) (*ReleaseManifest, error) {
	client := newValidatedHTTPClient(15*time.Second, ValidateAssetURL)
	return fetchManifestWithClient(ctx, client, babeLatestManifestURL)
}

func fetchManifestWithClient(ctx context.Context, client *http.Client, manifestURL string) (*ReleaseManifest, error) {
	if client == nil {
		client = newValidatedHTTPClient(15*time.Second, nil)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create manifest request: %w", err)
	}
	req.Header.Set("User-Agent", "babe-cli")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errManifestNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, formatHTTPError("release manifest", resp)
	}

	var manifest ReleaseManifest
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxManifestBytes))
	if err := dec.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("parse release manifest JSON: %w", err)
	}
	manifest.TagName = strings.TrimSpace(manifest.TagName)
	if manifest.TagName == "" {
		return nil, fmt.Errorf("release manifest is missing tag_name")
	}
	return &manifest, nil
}

func ResolveManifestAsset(manifest *ReleaseManifest, goos, goarch string) (*GitHubAsset, error) {
	return resolveManifestAssetWithValidator(manifest, goos, goarch, ValidateAssetURL)
}

func resolveManifestAssetWithValidator(manifest *ReleaseManifest, goos, goarch string, urlCheck func(string) error) (*GitHubAsset, error) {
	if manifest == nil {
		return nil, fmt.Errorf("release manifest is nil")
	}
	if strings.TrimSpace(manifest.TagName) == "" {
		return nil, fmt.Errorf("release manifest is missing tag_name")
	}
	key := fmt.Sprintf("%s-%s", goos, goarch)
	entry, ok := manifest.Assets[key]
	if !ok {
		return nil, fmt.Errorf("no release asset found in manifest for platform %s/%s", goos, goarch)
	}
	if strings.TrimSpace(entry.Name) == "" {
		return nil, fmt.Errorf("release manifest asset %s is missing name", key)
	}
	if urlCheck != nil {
		if err := urlCheck(entry.URL); err != nil {
			return nil, fmt.Errorf("invalid release asset URL for %s: %w", key, err)
		}
	}
	sha := strings.ToLower(strings.TrimSpace(entry.SHA256))
	if !isValidSHA256(sha) {
		return nil, fmt.Errorf("release manifest asset %s has invalid sha256 checksum", key)
	}
	return &GitHubAsset{
		Name:               entry.Name,
		BrowserDownloadURL: entry.URL,
		Size:               entry.Size,
		SHA256:             sha,
	}, nil
}

func manifestToGitHubRelease(m *ReleaseManifest) *GitHubRelease {
	rel := &GitHubRelease{
		TagName:     m.TagName,
		Name:        m.TagName,
		PublishedAt: m.PublishedAt,
		Assets:      make([]GitHubAsset, 0, len(m.Assets)),
	}
	for _, entry := range m.Assets {
		rel.Assets = append(rel.Assets, GitHubAsset{
			Name:               entry.Name,
			BrowserDownloadURL: entry.URL,
			Size:               entry.Size,
			SHA256:             strings.ToLower(strings.TrimSpace(entry.SHA256)),
		})
	}
	return rel
}

func formatHTTPError(source string, resp *http.Response) error {
	if (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests) &&
		resp.Header.Get("X-RateLimit-Remaining") == "0" {
		resetStr := "unknown"
		if raw := strings.TrimSpace(resp.Header.Get("X-RateLimit-Reset")); raw != "" {
			if ts, err := strconv.ParseInt(raw, 10, 64); err == nil && ts > 0 {
				resetStr = time.Unix(ts, 0).Local().Format(time.RFC1123)
			}
		}
		return fmt.Errorf("%s rate limit exceeded (HTTP %d, resets at %s). Set GITHUB_TOKEN or retry later",
			source, resp.StatusCode, resetStr)
	}
	return fmt.Errorf("%s returned HTTP %s", source, resp.Status)
}
