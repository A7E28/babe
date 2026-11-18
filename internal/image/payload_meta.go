package image

import (
	"archive/zip"
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"
)

func buildPayloadInfo(filePath, sourceType string, formatVer uint64, m *payloadManifest, otaMeta map[string]string) *PayloadInfo {
	hasDeltaOp := false
	partitions := make([]PayloadPartition, 0, len(m.Partitions))
	var totalSize uint64

	for _, p := range m.Partitions {
		var codecs []string
		var extractable bool
		var reason string
		var opCount int
		if p.SummaryOnly {
			codecs = p.SummaryCodecs
			if codecs == nil {
				codecs = []string{}
			}
			extractable = p.SummaryDelta == ""
			reason = p.SummaryDelta
			opCount = p.SummaryOpCnt
		} else {
			codecs, extractable, reason = summarizePartitionOps(p.Operations)
			opCount = len(p.Operations)
		}
		if !extractable {
			hasDeltaOp = true
		}
		if totalSize+p.NewInfo.Size >= totalSize {
			totalSize += p.NewInfo.Size
		} else {
			totalSize = ^uint64(0)
		}
		partitions = append(partitions, PayloadPartition{
			Name:              p.Name,
			Size:              p.NewInfo.Size,
			SHA256:            hex.EncodeToString(p.NewInfo.SHA256),
			OpCount:           opCount,
			Codecs:            codecs,
			Extractable:       extractable,
			UnsupportedReason: reason,
		})
	}

	otaType := "full"
	if hasDeltaOp {
		otaType = "delta"
	}
	secPatch := m.SecurityPatchLevel
	if secPatch == "" && otaMeta != nil {
		secPatch = otaMeta["post-security-patch-level"]
	}

	return &PayloadInfo{
		FilePath:           filePath,
		SourceType:         sourceType,
		FormatVersion:      formatVer,
		MinorVersion:       m.MinorVersion,
		OTAType:            otaType,
		BlockSize:          m.BlockSize,
		SecurityPatchLevel: secPatch,
		PartialUpdate:      m.PartialUpdate,
		SnapshotEnabled:    m.SnapshotEnabled,
		VABCEnabled:        m.VABCEnabled,
		DynamicGroups:      m.DynamicGroups,
		OTAMetadata:        otaMeta,
		Partitions:         partitions,
		TotalSize:          totalSize,
	}
}

func summarizePartitionOps(ops []installOp) ([]string, bool, string) {
	seen := make(map[string]struct{}, 4)
	extractable := true
	var reason string

	for _, op := range ops {
		seen[op.Type.CodecName()] = struct{}{}
		if !op.Type.IsExtractable() && extractable {
			extractable = false
			reason = fmt.Sprintf("requires source blocks from device (%s)", op.Type.String())
		}
	}

	codecs := make([]string, 0, len(seen))
	for c := range seen {
		codecs = append(codecs, c)
	}
	sort.Strings(codecs)
	return codecs, extractable, reason
}

func readOTAZipMetadata(zr *zip.Reader) map[string]string {
	meta := make(map[string]string)
	for _, f := range zr.File {
		switch strings.ToLower(stripLeadingDotSlash(f.Name)) {
		case "meta-inf/com/android/metadata":
			readPlainOTAMetadata(f, meta)
		case "meta-inf/com/android/metadata.pb":
			readProtoOTAMetadata(f, meta)
		}
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

func readPlainOTAMetadata(f *zip.File, meta map[string]string) {
	rc, err := f.Open()
	if err != nil {
		return
	}
	defer rc.Close()

	parsed := make(map[string]string)
	scanner := bufio.NewScanner(io.LimitReader(rc, 64*1024))
	for scanner.Scan() {
		k, v, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			continue
		}
		switch k {
		case "ota-type", "pre-device", "post-build", "post-build-incremental", "post-security-patch-level":
			if val := strings.TrimSpace(v); val != "" {
				parsed[k] = val
			}
		}
	}
	if scanner.Err() != nil {
		return
	}
	for k, v := range parsed {
		meta[k] = v
	}
}

func readProtoOTAMetadata(f *zip.File, meta map[string]string) {
	rc, err := f.Open()
	if err != nil {
		return
	}
	defer rc.Close()

	raw, err := io.ReadAll(io.LimitReader(rc, 256*1024))
	if err != nil {
		return
	}
	parsed := make(map[string]string)
	if err := walkProtoFields(raw, func(fieldNum, wireType int, val uint64, b []byte) error {
		switch {
		case fieldNum == 1 && wireType == wireVarint:
			switch val {
			case 1:
				parsed["ota-type"] = "AB"
			case 2:
				parsed["ota-type"] = "BLOCK"
			case 3:
				parsed["ota-type"] = "BRICK"
			}
		case fieldNum == 5 && wireType == wireBytes: // precondition DeviceState
			return parseDeviceStateProto(b, parsed, false)
		case fieldNum == 6 && wireType == wireBytes: // postcondition DeviceState
			return parseDeviceStateProto(b, parsed, true)
		}
		return nil
	}); err != nil {
		return
	}
	for k, v := range parsed {
		if meta[k] == "" {
			meta[k] = v
		}
	}
}

func parseDeviceStateProto(data []byte, meta map[string]string, isPost bool) error {
	var devices []string
	var builds []string
	if err := walkProtoFields(data, func(fieldNum, wireType int, _ uint64, b []byte) error {
		if wireType != wireBytes {
			return nil
		}
		switch fieldNum {
		case 1: // repeated string device
			devices = append(devices, string(b))
		case 2: // repeated string build
			if isPost {
				builds = append(builds, string(b))
			}
		case 3: // string build_incremental
			if isPost && meta["post-build-incremental"] == "" {
				meta["post-build-incremental"] = string(b)
			}
		case 6: // string security_patch_level
			if isPost && meta["post-security-patch-level"] == "" {
				meta["post-security-patch-level"] = string(b)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if len(devices) > 0 {
		key := "pre-device"
		if isPost {
			key = "post-device"
		}
		if meta[key] == "" {
			meta[key] = strings.Join(devices, ",")
		}
	}
	if isPost && len(builds) > 0 && meta["post-build"] == "" {
		meta["post-build"] = strings.Join(builds, ",")
	}
	return nil
}

func stripLeadingDotSlash(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	for strings.HasPrefix(name, "./") {
		name = name[2:]
	}
	return name
}
