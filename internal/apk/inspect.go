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

const (
	maxManifestBytes  = 16 * 1024 * 1024
	maxNestedAPKBytes = 128 * 1024 * 1024
)

type ArchiveInfo struct {
	FilePath          string   `json:"file_path"`
	FileSize          int64    `json:"file_size"`
	IsBundle          bool     `json:"is_bundle"`
	SplitAPKs         []string `json:"split_apks,omitempty"`
	PackageName       string   `json:"package_name,omitempty"`
	VersionName       string   `json:"version_name,omitempty"`
	VersionCode       uint32   `json:"version_code,omitempty"`
	MinSDK            uint32   `json:"min_sdk,omitempty"`
	MinSDKCodename    string   `json:"min_sdk_codename,omitempty"`
	TargetSDK         uint32   `json:"target_sdk,omitempty"`
	TargetSDKCodename string   `json:"target_sdk_codename,omitempty"`
	ABIs              []string `json:"abis"`
	DexCount          int      `json:"dex_count"`
	Permissions       []string `json:"permissions"`
	SignatureFiles    []string `json:"signature_files"`
}

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
		FilePath:       filePath,
		FileSize:       fi.Size(),
		ABIs:           []string{},
		Permissions:    []string{},
		SignatureFiles: []string{},
	}

	abiSet := make(map[string]bool)
	sigSet := make(map[string]bool)
	var manifestEntry *zip.File
	var splitEntries []*zip.File

	for _, f := range zr.File {
		name := f.Name
		lower := strings.ToLower(name)

		if name == "AndroidManifest.xml" {
			manifestEntry = f
		} else if strings.HasSuffix(lower, ".apk") && !f.FileInfo().IsDir() {
			info.SplitAPKs = append(info.SplitAPKs, name)
			splitEntries = append(splitEntries, f)
		} else {
			recordArchiveEntry(name, lower, abiSet, sigSet, &info.DexCount)
		}
	}

	if manifestEntry != nil {
		parseManifestEntry(manifestEntry, info)
		finalizeSets(info, abiSet, sigSet)
		return info, nil
	}

	if len(splitEntries) > 0 {
		info.IsBundle = true
		for _, entry := range splitEntries {
			isBase := strings.Contains(strings.ToLower(entry.Name), "base") || len(splitEntries) == 1
			inspectNestedAPK(entry, isBase, abiSet, sigSet, info)
		}
		finalizeSets(info, abiSet, sigSet)
		return info, nil
	}

	return nil, fmt.Errorf("file is neither a valid Android APK nor a Split APK bundle")
}

func recordArchiveEntry(name, lower string, abiSet, sigSet map[string]bool, dexCount *int) {
	if strings.HasPrefix(name, "lib/") && strings.HasSuffix(lower, ".so") {
		parts := strings.Split(name, "/")
		if len(parts) >= 3 && parts[1] != "" {
			abiSet[parts[1]] = true
		}
	} else if path.Dir(name) == "." && strings.HasPrefix(name, "classes") && strings.HasSuffix(lower, ".dex") {
		*dexCount++
	} else if strings.HasPrefix(name, "META-INF/") &&
		(strings.HasSuffix(lower, ".rsa") || strings.HasSuffix(lower, ".dsa") || strings.HasSuffix(lower, ".ec")) {
		sigSet[path.Base(name)] = true
	}
}

func finalizeSets(info *ArchiveInfo, abiSet, sigSet map[string]bool) {
	for abi := range abiSet {
		info.ABIs = append(info.ABIs, abi)
	}
	sort.Strings(info.ABIs)

	for sig := range sigSet {
		info.SignatureFiles = append(info.SignatureFiles, sig)
	}
	sort.Strings(info.SignatureFiles)
}

func parseManifestEntry(entry *zip.File, info *ArchiveInfo) {
	rc, err := entry.Open()
	if err != nil {
		return
	}
	defer rc.Close()

	raw, err := io.ReadAll(io.LimitReader(rc, maxManifestBytes))
	if err == nil {
		ParseAXMLManifest(raw, info)
	}
}

func inspectNestedAPK(entry *zip.File, parseManifest bool, abiSet, sigSet map[string]bool, info *ArchiveInfo) {
	if entry.UncompressedSize64 > maxNestedAPKBytes {
		return
	}
	rc, err := entry.Open()
	if err != nil {
		return
	}
	defer rc.Close()

	rawAPK, err := io.ReadAll(io.LimitReader(rc, maxNestedAPKBytes))
	if err != nil {
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(rawAPK), int64(len(rawAPK)))
	if err != nil {
		return
	}
	for _, f := range zr.File {
		if f.Name == "AndroidManifest.xml" && (parseManifest || info.PackageName == "") {
			parseManifestEntry(f, info)
		} else {
			recordArchiveEntry(f.Name, strings.ToLower(f.Name), abiSet, sigSet, &info.DexCount)
		}
	}
}
