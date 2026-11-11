package tools

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const repositoryXMLURL = "https://dl.google.com/android/repository/repository2-1.xml"

// details the latest stable platform-tools release from Google.
type RemoteRelease struct {
	Version  string
	ZipURL   string
	Checksum string
}

type repoXML struct {
	XMLName        xml.Name        `xml:"sdk-repository"`
	RemotePackages []remotePackage `xml:"remotePackage"`
}

type remotePackage struct {
	Path       string       `xml:"path,attr"`
	Revision   repoRevision `xml:"revision"`
	ChannelRef channelRef   `xml:"channelRef"`
	Archives   repoArchives `xml:"archives"`
}

type channelRef struct {
	Ref string `xml:"ref,attr"`
}

type repoRevision struct {
	Major int  `xml:"major"`
	Minor int  `xml:"minor"`
	Micro *int `xml:"micro"`
}

type repoArchives struct {
	Archives []repoArchive `xml:"archive"`
}

type repoArchive struct {
	Complete archiveComplete `xml:"complete"`
	HostOS   string          `xml:"host-os"`
}

type archiveComplete struct {
	Checksum string `xml:"checksum"`
	URL      string `xml:"url"`
	Size     int64  `xml:"size"`
}

// queries the Google Android SDK repository for the latest stable platform-tools release.
func FetchLatestRelease(ctx context.Context) (*RemoteRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, repositoryXMLURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create repository request: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch repository index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("repository returned HTTP %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read repository index: %w", err)
	}

	var repo repoXML
	if err := xml.Unmarshal(data, &repo); err != nil {
		return nil, fmt.Errorf("parse repository XML: %w", err)
	}

	targetOS := resolveTargetHostOS()
	var bestRelease *RemoteRelease

	for _, pkg := range repo.RemotePackages {
		if pkg.Path != "platform-tools" || pkg.ChannelRef.Ref != "channel-0" {
			continue
		}

		micro := 0
		if pkg.Revision.Micro != nil {
			micro = *pkg.Revision.Micro
		}
		verStr := fmt.Sprintf("%d.%d.%d", pkg.Revision.Major, pkg.Revision.Minor, micro)

		for _, arch := range pkg.Archives.Archives {
			if arch.HostOS == targetOS {
				downloadURL := arch.Complete.URL
				if !strings.HasPrefix(downloadURL, "http://") && !strings.HasPrefix(downloadURL, "https://") {
					downloadURL = "https://dl.google.com/android/repository/" + downloadURL
				}

				candidate := &RemoteRelease{
					Version:  verStr,
					ZipURL:   downloadURL,
					Checksum: arch.Complete.Checksum,
				}

				if bestRelease == nil || CompareVersions(candidate.Version, bestRelease.Version) > 0 {
					bestRelease = candidate
				}
			}
		}
	}

	if bestRelease == nil {
		return nil, fmt.Errorf("no matching platform-tools package found for OS %s", runtime.GOOS)
	}

	return bestRelease, nil
}

func resolveTargetHostOS() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "macosx"
	default:
		return "linux"
	}
}

// compares two semantic version strings.
// returns -1 if v1 < v2, 0 if v1 == v2, and 1 if v1 > v2.
func CompareVersions(v1, v2 string) int {
	clean1 := cleanVersionString(v1)
	clean2 := cleanVersionString(v2)

	parts1 := strings.Split(clean1, ".")
	parts2 := strings.Split(clean2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}

	return 0
}

func cleanVersionString(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "r")
	if idx := strings.Index(v, "-"); idx != -1 {
		v = v[:idx]
	}
	return v
}
