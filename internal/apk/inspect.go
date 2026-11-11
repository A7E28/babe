package apk

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
)

type ArchiveInfo struct {
	FilePath       string
	FileSize       int64
	IsBundle       bool
	SplitAPKs      []string
	PackageName    string
	VersionName    string
	VersionCode    uint32
	MinSDK         uint32
	TargetSDK      uint32
	ABIs           []string
	DexCount       int
	Permissions    []string
	SignatureFiles []string
}

// offline static analysis on an .apk, .apks, .xapk, or .apkm file.
func InspectArchive(filePath string) (*ArchiveInfo, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat archive: %w", err)
	}

	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}
	defer zr.Close()

	info := &ArchiveInfo{
		FilePath: filePath,
		FileSize: fi.Size(),
	}

	abiSet := make(map[string]bool)
	var manifestEntry *zip.File
	var baseAPKEntry *zip.File

	for _, f := range zr.File {
		name := f.Name
		lower := strings.ToLower(name)

		if name == "AndroidManifest.xml" {
			manifestEntry = f
		} else if strings.HasSuffix(lower, ".apk") && !f.FileInfo().IsDir() {
			info.SplitAPKs = append(info.SplitAPKs, name)
			if baseAPKEntry == nil || strings.Contains(lower, "base") {
				baseAPKEntry = f
			}
		} else if strings.HasPrefix(name, "lib/") && strings.HasSuffix(lower, ".so") {
			parts := strings.Split(name, "/")
			if len(parts) >= 3 && parts[1] != "" {
				abiSet[parts[1]] = true
			}
		} else if path.Dir(name) == "." && strings.HasPrefix(name, "classes") && strings.HasSuffix(name, ".dex") {
			info.DexCount++
		} else if strings.HasPrefix(name, "META-INF/") &&
			(strings.HasSuffix(lower, ".rsa") || strings.HasSuffix(lower, ".dsa") || strings.HasSuffix(lower, ".ec")) {
			info.SignatureFiles = append(info.SignatureFiles, path.Base(name))
		}
	}

	for abi := range abiSet {
		info.ABIs = append(info.ABIs, abi)
	}
	sort.Strings(info.ABIs)

	if manifestEntry != nil {
		rc, err := manifestEntry.Open()
		if err == nil {
			raw, readErr := io.ReadAll(rc)
			_ = rc.Close()
			if readErr == nil {
				ParseAXMLManifest(raw, info)
			}
		}
		return info, nil
	}

	if len(info.SplitAPKs) > 0 {
		info.IsBundle = true
		if baseAPKEntry != nil {
			inspectNestedAPK(baseAPKEntry, info)
		}
		return info, nil
	}

	return nil, fmt.Errorf("file is neither a valid Android APK nor a Split APK bundle")
}

func inspectNestedAPK(entry *zip.File, info *ArchiveInfo) {
	rc, err := entry.Open()
	if err != nil {
		return
	}
	defer rc.Close()

	rawAPK, err := io.ReadAll(rc)
	if err != nil {
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(rawAPK), int64(len(rawAPK)))
	if err != nil {
		return
	}
	for _, f := range zr.File {
		if f.Name == "AndroidManifest.xml" {
			mc, err := f.Open()
			if err == nil {
				data, _ := io.ReadAll(mc)
				_ = mc.Close()
				ParseAXMLManifest(data, info)
			}
			break
		}
	}
}
