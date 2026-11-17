package image

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
)

type legacyPartitionSpec struct {
	name          string
	kind          string // "raw-img", "sdat", or "delta-only"
	entryPath     string
	imgFile       *zip.File
	datFile       *zip.File
	datEntryPath  string
	transferFile  *zip.File
	isBrotli      bool
	newRanges     []transferRange
	maxBlock      uint64
	extractable   bool
	unavailReason string
	codecs        []string
	size          uint64
}

func isLegacyOTAZip(zr *zip.Reader) bool {
	for _, f := range zr.File {
		if f.Mode()&os.ModeSymlink != 0 || f.FileInfo().IsDir() {
			continue
		}
		lower := strings.ToLower(stripLeadingDotSlash(f.Name))
		if strings.HasSuffix(lower, ".new.dat.br") || strings.HasSuffix(lower, ".new.dat") || strings.HasSuffix(lower, ".transfer.list") {
			return true
		}
		if lower == "meta-inf/com/google/android/updater-script" || lower == "android-info.txt" {
			return true
		}
		if strings.HasSuffix(lower, ".img") && !strings.Contains(lower, "/") {
			base := path.Base(lower)
			if base != ".img" && base != "super_empty.img" && base != "unsparse_super_empty.img" {
				return true
			}
		}
		if (strings.HasPrefix(lower, "images/") || strings.HasPrefix(lower, "firmware-update/")) && strings.HasSuffix(lower, ".img") {
			return true
		}
	}
	return false
}

func openLegacyZipSource(f interface {
	io.ReaderAt
	io.Closer
}, fi os.FileInfo, zr *zip.Reader, filePath string) (*payloadSource, error) {
	specs, err := scanLegacyZipPartitions(zr)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("no extractable partition images or .new.dat entries found in archive %q", filePath)
	}

	otaMeta := readOTAZipMetadata(zr)
	hasDelta := false
	partitions := make([]PayloadPartition, 0, len(specs))
	var totalSize uint64
	legacyMap := make(map[string]legacyPartitionSpec, len(specs))

	for _, sp := range specs {
		if !sp.extractable {
			hasDelta = true
		}
		if totalSize+sp.size >= totalSize {
			totalSize += sp.size
		} else {
			totalSize = ^uint64(0)
		}
		legacyMap[sp.name] = sp
		partitions = append(partitions, PayloadPartition{
			Name:              sp.name,
			Size:              sp.size,
			OpCount:           max(1, len(sp.newRanges)),
			Codecs:            sp.codecs,
			Extractable:       sp.extractable,
			UnsupportedReason: sp.unavailReason,
		})
	}

	otaType := "full"
	if hasDelta {
		otaType = "delta"
	}

	info := &PayloadInfo{
		FilePath:           filePath,
		SourceType:         "legacy-zip",
		FormatVersion:      0,
		OTAType:            otaType,
		BlockSize:          DefaultBlockSize,
		SecurityPatchLevel: otaMeta["post-security-patch-level"],
		DynamicGroups:      []DynamicGroupInfo{},
		OTAMetadata:        otaMeta,
		Partitions:         partitions,
		TotalSize:          totalSize,
	}

	return &payloadSource{
		sourcePath:  filePath,
		initialInfo: fi,
		fileSize:    fi.Size(),
		reader:      f,
		size:        fi.Size(),
		info:        info,
		legacyZip:   zr,
		legacySpec:  legacyMap,
		closer:      f,
	}, nil
}

func scanLegacyZipPartitions(zr *zip.Reader) ([]legacyPartitionSpec, error) {
	byLower := make(map[string]*zip.File, len(zr.File))
	for _, zf := range zr.File {
		if zf.Mode()&os.ModeSymlink != 0 || zf.FileInfo().IsDir() {
			continue
		}
		key := strings.ToLower(stripLeadingDotSlash(zf.Name))
		if _, exists := byLower[key]; !exists {
			byLower[key] = zf
		}
	}

	seen := make(map[string]struct{})
	var specs []legacyPartitionSpec

	for _, zf := range zr.File {
		if zf.Mode()&os.ModeSymlink != 0 || zf.FileInfo().IsDir() {
			continue
		}
		clean := stripLeadingDotSlash(zf.Name)
		lower := strings.ToLower(clean)
		if !strings.HasSuffix(lower, ".transfer.list") {
			continue
		}
		base := path.Base(clean)
		part := base[:len(base)-len(".transfer.list")]
		if part == "" {
			continue
		}
		partLower := strings.ToLower(part)
		if _, dup := seen[partLower]; dup {
			continue
		}
		dirLower := strings.ToLower(path.Dir(clean))
		datBr := joinZipPath(dirLower, partLower+".new.dat.br")
		datRaw := joinZipPath(dirLower, partLower+".new.dat")
		patchDat := joinZipPath(dirLower, partLower+".patch.dat")

		var datFile *zip.File
		var isBrotli bool
		if f, ok := byLower[datBr]; ok {
			datFile = f
			isBrotli = true
		} else if f, ok := byLower[datRaw]; ok {
			datFile = f
		}

		pf, ok := byLower[patchDat]
		patchHasData := ok && pf.UncompressedSize64 > 0

		sp, err := parseLegacySdatSpec(part, zf, datFile, isBrotli, patchHasData)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", clean, err)
		}
		seen[partLower] = struct{}{}
		specs = append(specs, sp)
	}

	for _, zf := range zr.File {
		if zf.Mode()&os.ModeSymlink != 0 || zf.FileInfo().IsDir() {
			continue
		}
		clean := stripLeadingDotSlash(zf.Name)
		lower := strings.ToLower(clean)
		if strings.Contains(clean, "/") && !strings.HasPrefix(lower, "images/") && !strings.HasPrefix(lower, "firmware-update/") {
			continue
		}
		base := path.Base(clean)
		if !strings.HasSuffix(strings.ToLower(base), ".img") || strings.EqualFold(base, "unsparse_super_empty.img") || strings.EqualFold(base, "super_empty.img") {
			continue
		}
		part := base[:len(base)-len(".img")]
		if part == "" {
			continue
		}
		if _, dup := seen[strings.ToLower(part)]; dup {
			continue
		}
		seen[strings.ToLower(part)] = struct{}{}
		codec, extractable, reason := legacyZipCodec(zf.Method)
		specs = append(specs, legacyPartitionSpec{
			name:          part,
			kind:          "raw-img",
			entryPath:     zf.Name,
			imgFile:       zf,
			extractable:   extractable,
			unavailReason: reason,
			codecs:        []string{codec},
			size:          zf.UncompressedSize64,
		})
	}

	sort.Slice(specs, func(i, j int) bool { return specs[i].name < specs[j].name })
	return specs, nil
}

func legacyZipCodec(method uint16) (string, bool, string) {
	switch method {
	case zip.Store:
		return "zip-store", true, ""
	case zip.Deflate:
		return "zip-deflate", true, ""
	default:
		return fmt.Sprintf("zip-method-%d", method), false, fmt.Sprintf("unsupported zip compression method %d (expected Store or Deflate)", method)
	}
}

func joinZipPath(dir, name string) string {
	if dir == "." || dir == "" {
		return name
	}
	return dir + "/" + name
}
