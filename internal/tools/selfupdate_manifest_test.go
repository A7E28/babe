package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestValidateAssetURL(t *testing.T) {
	valid := []string{
		"https://github.com/A7E28/babe/releases/download/v0.4.0/babe-v0.4.0-windows-amd64.exe",
		"https://github.com/A7E28/babe/releases/latest/download/latest.json",
		"https://objects.githubusercontent.com/github-production-release-asset-2e65be/abc",
		"https://release-assets.githubusercontent.com/github-production-release-asset/xyz",
	}
	for _, u := range valid {
		if err := ValidateAssetURL(u); err != nil {
			t.Errorf("ValidateAssetURL(%q) returned unexpected error: %v", u, err)
		}
	}

	invalid := []string{
		"",
		"http://github.com/A7E28/babe/releases/download/v0.4.0/babe.exe",
		"https://github.com/other/repo/releases/download/v0.4.0/babe.exe",
		"https://evil.example.com/babe.exe",
		"file:///tmp/babe",
	}
	for _, u := range invalid {
		if err := ValidateAssetURL(u); err == nil {
			t.Errorf("ValidateAssetURL(%q) expected error, got nil", u)
		}
	}
}

func TestFetchAndResolveManifest_WithHTTPTestServer(t *testing.T) {
	payload := []byte("fake-binary-payload")
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])

	var gotUserAgent, gotAccept, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ReleaseManifest{
			TagName:     "v0.4.0",
			PublishedAt: "2026-11-17T10:00:00Z",
			Assets: map[string]ManifestAsset{
				"linux-amd64": {
					Name:   "babe-v0.4.0-linux-amd64",
					URL:    "https://github.com/A7E28/babe/releases/download/v0.4.0/babe-v0.4.0-linux-amd64",
					SHA256: hexSum,
					Size:   int64(len(payload)),
				},
			},
		})
	}))
	defer srv.Close()

	manifest, err := fetchManifestWithClient(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fetchManifestWithClient failed: %v", err)
	}
	if gotUserAgent != "babe-cli" {
		t.Errorf("expected User-Agent babe-cli, got %q", gotUserAgent)
	}
	if gotAccept != "application/json" {
		t.Errorf("expected Accept application/json, got %q", gotAccept)
	}
	if gotAuth != "" {
		t.Errorf("expected no Authorization header on manifest request, got %q", gotAuth)
	}

	asset, err := ResolveManifestAsset(manifest, "linux", "amd64")
	if err != nil {
		t.Fatalf("ResolveManifestAsset failed: %v", err)
	}
	if asset.SHA256 != hexSum || asset.Size != int64(len(payload)) {
		t.Errorf("unexpected asset metadata: %+v", asset)
	}

	if _, err := ResolveManifestAsset(manifest, "windows", "arm64"); err == nil {
		t.Errorf("expected error for missing platform key windows/arm64")
	}
}

func TestResolveManifestAsset_ValidationRejections(t *testing.T) {
	validSHA := strings.Repeat("a", 64)
	cases := []struct {
		name     string
		manifest *ReleaseManifest
	}{
		{
			name: "missing tag_name",
			manifest: &ReleaseManifest{
				Assets: map[string]ManifestAsset{
					"linux-amd64": {Name: "babe", URL: "https://github.com/A7E28/babe/releases/download/v0.4.0/babe", SHA256: validSHA},
				},
			},
		},
		{
			name: "malformed sha256",
			manifest: &ReleaseManifest{
				TagName: "v0.4.0",
				Assets: map[string]ManifestAsset{
					"linux-amd64": {Name: "babe", URL: "https://github.com/A7E28/babe/releases/download/v0.4.0/babe", SHA256: "not-hex"},
				},
			},
		},
		{
			name: "untrusted http url",
			manifest: &ReleaseManifest{
				TagName: "v0.4.0",
				Assets: map[string]ManifestAsset{
					"linux-amd64": {Name: "babe", URL: "http://example.com/babe", SHA256: validSHA},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ResolveManifestAsset(tc.manifest, "linux", "amd64"); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestManifestAndAPIErrors_404AndRateLimit403(t *testing.T) {
	srv404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv404.Close()

	_, err := fetchManifestWithClient(context.Background(), srv404.Client(), srv404.URL)
	if !errors.Is(err, errManifestNotFound) {
		t.Errorf("expected errManifestNotFound on 404, got %v", err)
	}

	srv403 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1794900000")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv403.Close()

	_, err = fetchLatestFromGitHubAPI(context.Background(), srv403.URL, "", srv403.Client())
	if err == nil || !strings.Contains(err.Error(), "rate limit exceeded") || !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Errorf("expected clear rate-limit error with GITHUB_TOKEN hint, got %v", err)
	}
}

func TestCheckForUpdateCached(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_ = json.NewEncoder(w).Encode(ReleaseManifest{TagName: "v0.5.0"})
	}))
	defer srv.Close()

	cachePath := filepath.Join(t.TempDir(), "babe", "update-check.json")

	// 1. Missing cache -> fetches remote and populates cache
	latest, avail, err := checkForUpdateCachedWithClient(context.Background(), "v0.4.0", cachePath, srv.URL, 24*time.Hour, srv.Client())
	if err != nil || !avail || latest != "v0.5.0" {
		t.Fatalf("unexpected first check result: latest=%q avail=%v err=%v", latest, avail, err)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected 1 HTTP hit, got %d", hits.Load())
	}

	// 2. Fresh cache -> zero HTTP calls
	latest, avail, err = checkForUpdateCachedWithClient(context.Background(), "v0.4.0", cachePath, srv.URL, 24*time.Hour, srv.Client())
	if err != nil || !avail || latest != "v0.5.0" {
		t.Fatalf("unexpected cached check result: latest=%q avail=%v err=%v", latest, avail, err)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected 0 extra HTTP hits on fresh cache, got %d total", hits.Load())
	}

	// 3. Corrupt cache JSON -> silently ignored and refetched
	if err := os.WriteFile(cachePath, []byte("{corrupt"), 0644); err != nil {
		t.Fatalf("write corrupt cache: %v", err)
	}
	latest, avail, err = checkForUpdateCachedWithClient(context.Background(), "v0.5.0", cachePath, srv.URL, 24*time.Hour, srv.Client())
	if err != nil || avail || latest != "v0.5.0" {
		t.Fatalf("unexpected post-corrupt result: latest=%q avail=%v err=%v", latest, avail, err)
	}
	if hits.Load() != 2 {
		t.Fatalf("expected refetch after corrupt cache, got %d hits", hits.Load())
	}

	// 4. dev version -> skipped with zero HTTP hits
	_, avail, err = checkForUpdateCachedWithClient(context.Background(), "dev", cachePath, srv.URL, 24*time.Hour, srv.Client())
	if err != nil || avail || hits.Load() != 2 {
		t.Fatalf("expected dev version to skip update check, got avail=%v err=%v hits=%d", avail, err, hits.Load())
	}
}

func TestValidatedHTTPClient_BlocksUntrustedRedirectAndRejectsApplyURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://evil.example.com/payload.exe", http.StatusFound)
	}))
	defer srv.Close()

	client := newValidatedHTTPClient(5*time.Second, ValidateAssetURL)
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("create req: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret-token")

	if _, err := client.Do(req); err == nil || !strings.Contains(err.Error(), "blocked redirect target") {
		t.Fatalf("expected blocked redirect target error, got %v", err)
	}

	_, err = ApplyBabeUpdate(context.Background(), &GitHubAsset{
		Name:               "babe-v0.4.0-linux-amd64",
		BrowserDownloadURL: "http://evil.example.com/babe",
	}, strings.Repeat("a", 64), nil)
	if err == nil || !strings.Contains(err.Error(), "untrusted") {
		t.Fatalf("expected untrusted URL rejection, got %v", err)
	}
}
