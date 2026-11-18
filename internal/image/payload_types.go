package image

import "fmt"

const (
	PayloadMagic         = "CrAU"
	PayloadHeaderSize    = 24
	PayloadFormatVersion = 2
	DefaultBlockSize     = 4096
)

// PayloadOpType mirrors UpdateEngine's InstallOperation.Type enum.
type PayloadOpType int32

const (
	OpReplace         PayloadOpType = 0
	OpReplaceBZ       PayloadOpType = 1
	OpMove            PayloadOpType = 2
	OpBsdiff          PayloadOpType = 3
	OpSourceCopy      PayloadOpType = 4
	OpSourceBsdiff    PayloadOpType = 5
	OpZero            PayloadOpType = 6
	OpDiscard         PayloadOpType = 7
	OpReplaceXZ       PayloadOpType = 8
	OpPuffdiff        PayloadOpType = 9
	OpBrotliBsdiff    PayloadOpType = 10
	OpZucchini        PayloadOpType = 11
	OpLz4diffBsdiff   PayloadOpType = 12
	OpLz4diffPuffdiff PayloadOpType = 13
	OpReplaceZstd     PayloadOpType = 14
)

func (op PayloadOpType) String() string {
	switch op {
	case OpReplace:
		return "REPLACE"
	case OpReplaceBZ:
		return "REPLACE_BZ"
	case OpMove:
		return "MOVE"
	case OpBsdiff:
		return "BSDIFF"
	case OpSourceCopy:
		return "SOURCE_COPY"
	case OpSourceBsdiff:
		return "SOURCE_BSDIFF"
	case OpZero:
		return "ZERO"
	case OpDiscard:
		return "DISCARD"
	case OpReplaceXZ:
		return "REPLACE_XZ"
	case OpPuffdiff:
		return "PUFFDIFF"
	case OpBrotliBsdiff:
		return "BROTLI_BSDIFF"
	case OpZucchini:
		return "ZUCCHINI"
	case OpLz4diffBsdiff:
		return "LZ4DIFF_BSDIFF"
	case OpLz4diffPuffdiff:
		return "LZ4DIFF_PUFFDIFF"
	case OpReplaceZstd:
		return "REPLACE_ZSTD"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int32(op))
	}
}

func (op PayloadOpType) CodecName() string {
	switch op {
	case OpReplace:
		return "raw"
	case OpReplaceBZ:
		return "bzip2"
	case OpReplaceXZ:
		return "xz"
	case OpReplaceZstd:
		return "zstd"
	case OpZero, OpDiscard:
		return "zero"
	default:
		return op.String()
	}
}

// IsExtractable reports whether the operation can be reconstructed offline without source blocks.
func (op PayloadOpType) IsExtractable() bool {
	switch op {
	case OpReplace, OpReplaceBZ, OpReplaceXZ, OpReplaceZstd, OpZero, OpDiscard:
		return true
	default:
		return false
	}
}

type DynamicGroupInfo struct {
	Name       string   `json:"name"`
	Size       uint64   `json:"size"`
	Partitions []string `json:"partitions"`
}

type PayloadPartition struct {
	Name              string   `json:"name"`
	Size              uint64   `json:"size"`
	SHA256            string   `json:"sha256,omitempty"`
	OpCount           int      `json:"op_count"`
	Codecs            []string `json:"codecs"`
	Extractable       bool     `json:"extractable"`
	UnsupportedReason string   `json:"unsupported_reason,omitempty"`
}

type PayloadInfo struct {
	FilePath           string             `json:"file_path"`
	SourceType         string             `json:"source_type"`
	FormatVersion      uint64             `json:"format_version"`
	MinorVersion       uint32             `json:"minor_version"`
	OTAType            string             `json:"ota_type"`
	BlockSize          uint32             `json:"block_size"`
	SecurityPatchLevel string             `json:"security_patch_level,omitempty"`
	PartialUpdate      bool               `json:"partial_update,omitempty"`
	SnapshotEnabled    bool               `json:"snapshot_enabled,omitempty"`
	VABCEnabled        bool               `json:"vabc_enabled,omitempty"`
	DynamicGroups      []DynamicGroupInfo `json:"dynamic_groups"`
	OTAMetadata        map[string]string  `json:"ota_metadata,omitempty"`
	Partitions         []PayloadPartition `json:"partitions"`
	TotalSize          uint64             `json:"total_size"`
}

type PayloadExtractOptions struct {
	OutputDir   string
	Partitions  []string
	All         bool
	Jobs        int
	Verify      bool
	InspectBoot bool
	NoClobber   bool
}

type PayloadExtractItem struct {
	Partition  string         `json:"partition"`
	OutputPath string         `json:"output_path"`
	Size       uint64         `json:"size"`
	SHA256     string         `json:"sha256"`
	Verified   bool           `json:"verified"`
	DurationMs int64          `json:"duration_ms"`
	Codecs     []string       `json:"codecs"`
	BootInfo   *BootImageInfo `json:"boot_info,omitempty"`
}

type PayloadExtractResult struct {
	FilePath   string               `json:"file_path"`
	SourceType string               `json:"source_type,omitempty"`
	OTAType    string               `json:"ota_type"`
	Jobs       int                  `json:"jobs"`
	Extracted  []PayloadExtractItem `json:"extracted"`
	TotalBytes uint64               `json:"total_bytes"`
	DurationMs int64                `json:"duration_ms"`
}

type payloadManifest struct {
	BlockSize          uint32
	MinorVersion       uint32
	Partitions         []partitionUpdate
	DynamicGroups      []DynamicGroupInfo
	SnapshotEnabled    bool
	VABCEnabled        bool
	PartialUpdate      bool
	SecurityPatchLevel string
}

type partitionUpdate struct {
	Name          string
	NewInfo       partitionInfo
	Operations    []installOp
	SummaryOnly   bool
	SummaryOpCnt  int
	SummaryCodecs []string
	SummaryDelta  string
}

type partitionInfo struct {
	Size   uint64
	SHA256 []byte
}

type installOp struct {
	Type           PayloadOpType
	DataOffset     uint64
	DataLength     uint64
	DstExtents     []extent
	DstLength      uint64
	DataSHA256Hash []byte
}

type extent struct {
	StartBlock uint64
	NumBlocks  uint64
}
