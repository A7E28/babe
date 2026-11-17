package image

import (
	"fmt"
	"math"
	"sort"
)

const (
	wireVarint  = 0
	wireFixed64 = 1
	wireBytes   = 2
	wireFixed32 = 5

	maxManifestPartitions = 256
	maxPartitionOps       = 1_000_000
	maxOpExtents          = 65_536
	maxProtoHashBytes     = 64
)

// decodeManifest parses a serialized AOSP DeltaArchiveManifest protobuf without reflection.
// When summaryOnly is true, per-op structs and byte slices are skipped to avoid OOM amplification.
func decodeManifest(data []byte, summaryOnly bool) (*payloadManifest, error) {
	m := &payloadManifest{
		BlockSize:     DefaultBlockSize,
		DynamicGroups: []DynamicGroupInfo{},
	}

	err := walkProtoFields(data, func(fieldNum, wireType int, val uint64, raw []byte) error {
		switch {
		case fieldNum == 3 && wireType == wireVarint:
			if val == 0 || val > math.MaxUint32 {
				return fmt.Errorf("invalid payload block_size: %d", val)
			}
			m.BlockSize = uint32(val)
		case fieldNum == 12 && wireType == wireVarint:
			if val > math.MaxUint32 {
				return fmt.Errorf("invalid payload minor_version: %d", val)
			}
			m.MinorVersion = uint32(val)
		case fieldNum == 13 && wireType == wireBytes:
			if len(m.Partitions) >= maxManifestPartitions {
				return fmt.Errorf("payload partition count exceeds limit (%d)", maxManifestPartitions)
			}
			p, err := decodePartitionUpdate(raw, summaryOnly)
			if err != nil {
				return err
			}
			m.Partitions = append(m.Partitions, p)
		case fieldNum == 15 && wireType == wireBytes:
			return decodeDynamicMetadata(raw, m)
		case fieldNum == 16 && wireType == wireVarint:
			m.PartialUpdate = val != 0
		case fieldNum == 18 && wireType == wireBytes:
			m.SecurityPatchLevel = string(raw)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}

func decodePartitionUpdate(data []byte, summaryOnly bool) (partitionUpdate, error) {
	p := partitionUpdate{SummaryOnly: summaryOnly}
	var seenCodecs map[string]struct{}
	if summaryOnly {
		seenCodecs = make(map[string]struct{}, 4)
	}

	err := walkProtoFields(data, func(fieldNum, wireType int, _ uint64, raw []byte) error {
		if wireType != wireBytes {
			return nil
		}
		switch fieldNum {
		case 1:
			p.Name = string(raw)
		case 7:
			return decodePartitionInfoInto(raw, &p.NewInfo)
		case 8:
			if summaryOnly {
				if p.SummaryOpCnt >= maxPartitionOps {
					return fmt.Errorf("partition operation count exceeds limit (%d)", maxPartitionOps)
				}
				p.SummaryOpCnt++
				op, err := decodeInstallOp(raw, true)
				if err != nil {
					return err
				}
				seenCodecs[op.Type.CodecName()] = struct{}{}
				if !op.Type.IsExtractable() && p.SummaryDelta == "" {
					p.SummaryDelta = fmt.Sprintf("requires source blocks from device (%s)", op.Type.String())
				}
				return nil
			}
			if len(p.Operations) >= maxPartitionOps {
				return fmt.Errorf("partition operation count exceeds limit (%d)", maxPartitionOps)
			}
			op, err := decodeInstallOp(raw, false)
			if err != nil {
				return err
			}
			p.Operations = append(p.Operations, op)
		}
		return nil
	})
	if summaryOnly && len(seenCodecs) > 0 {
		p.SummaryCodecs = make([]string, 0, len(seenCodecs))
		for c := range seenCodecs {
			p.SummaryCodecs = append(p.SummaryCodecs, c)
		}
		sort.Strings(p.SummaryCodecs)
	}
	return p, err
}

func decodePartitionInfoInto(data []byte, info *partitionInfo) error {
	return walkProtoFields(data, func(fieldNum, wireType int, val uint64, raw []byte) error {
		if fieldNum == 1 && wireType == wireVarint {
			info.Size = val
		} else if fieldNum == 2 && wireType == wireBytes {
			if len(raw) > maxProtoHashBytes {
				return fmt.Errorf("partition hash length %d exceeds %d bytes", len(raw), maxProtoHashBytes)
			}
			info.SHA256 = append([]byte(nil), raw...)
		}
		return nil
	})
}

func decodeInstallOp(data []byte, summaryOnly bool) (installOp, error) {
	var op installOp
	err := walkProtoFields(data, func(fieldNum, wireType int, val uint64, raw []byte) error {
		if summaryOnly && fieldNum != 1 {
			return nil
		}
		switch {
		case fieldNum == 1 && wireType == wireVarint:
			if val > math.MaxInt32 {
				return fmt.Errorf("invalid operation type: %d", val)
			}
			op.Type = PayloadOpType(val)
		case fieldNum == 2 && wireType == wireVarint:
			op.DataOffset = val
		case fieldNum == 3 && wireType == wireVarint:
			op.DataLength = val
		case fieldNum == 6 && wireType == wireBytes:
			if len(op.DstExtents) >= maxOpExtents {
				return fmt.Errorf("operation extent count exceeds limit (%d)", maxOpExtents)
			}
			ext, err := decodeExtent(raw)
			if err != nil {
				return err
			}
			op.DstExtents = append(op.DstExtents, ext)
		case fieldNum == 7 && wireType == wireVarint:
			op.DstLength = val
		case fieldNum == 8 && wireType == wireBytes:
			if len(raw) > maxProtoHashBytes {
				return fmt.Errorf("operation hash length %d exceeds %d bytes", len(raw), maxProtoHashBytes)
			}
			op.DataSHA256Hash = append([]byte(nil), raw...)
		}
		return nil
	})
	return op, err
}

func decodeExtent(data []byte) (extent, error) {
	var ext extent
	err := walkProtoFields(data, func(fieldNum, wireType int, val uint64, _ []byte) error {
		if wireType != wireVarint {
			return nil
		}
		if fieldNum == 1 {
			ext.StartBlock = val
		} else if fieldNum == 2 {
			ext.NumBlocks = val
		}
		return nil
	})
	return ext, err
}

func decodeDynamicMetadata(data []byte, m *payloadManifest) error {
	return walkProtoFields(data, func(fieldNum, wireType int, val uint64, raw []byte) error {
		switch {
		case fieldNum == 1 && wireType == wireBytes:
			if len(m.DynamicGroups) >= maxManifestPartitions {
				return fmt.Errorf("dynamic group count exceeds limit (%d)", maxManifestPartitions)
			}
			g, err := decodeDynamicGroup(raw)
			if err != nil {
				return err
			}
			m.DynamicGroups = append(m.DynamicGroups, g)
		case fieldNum == 2 && wireType == wireVarint:
			m.SnapshotEnabled = val != 0
		case fieldNum == 3 && wireType == wireVarint:
			m.VABCEnabled = val != 0
		}
		return nil
	})
}

func decodeDynamicGroup(data []byte) (DynamicGroupInfo, error) {
	g := DynamicGroupInfo{Partitions: []string{}}
	err := walkProtoFields(data, func(fieldNum, wireType int, val uint64, raw []byte) error {
		switch {
		case fieldNum == 1 && wireType == wireBytes:
			g.Name = string(raw)
		case fieldNum == 2 && wireType == wireVarint:
			g.Size = val
		case fieldNum == 3 && wireType == wireBytes:
			if len(g.Partitions) >= maxManifestPartitions {
				return fmt.Errorf("dynamic group partition count exceeds limit (%d)", maxManifestPartitions)
			}
			g.Partitions = append(g.Partitions, string(raw))
		}
		return nil
	})
	return g, err
}
