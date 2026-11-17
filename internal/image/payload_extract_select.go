package image

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type selectedPartition struct {
	update partitionUpdate
	meta   PayloadPartition
}

func selectTargetPartitions(m *payloadManifest, info *PayloadInfo, opts PayloadExtractOptions) ([]selectedPartition, error) {
	byName := make(map[string]selectedPartition, len(m.Partitions))
	for i, p := range m.Partitions {
		if _, dup := byName[p.Name]; dup {
			return nil, fmt.Errorf("duplicate partition %q in payload manifest", p.Name)
		}
		byName[p.Name] = selectedPartition{update: p, meta: info.Partitions[i]}
	}

	var chosen []selectedPartition
	extractAll := opts.All
	if !extractAll && len(opts.Partitions) == 1 && strings.EqualFold(strings.TrimSpace(opts.Partitions[0]), "all") {
		if _, hasLiteralAll := byName[strings.TrimSpace(opts.Partitions[0])]; !hasLiteralAll {
			extractAll = true
		}
	}
	if extractAll {
		for i, p := range m.Partitions {
			if info.Partitions[i].Extractable {
				chosen = append(chosen, selectedPartition{update: p, meta: info.Partitions[i]})
			}
		}
		if len(chosen) == 0 {
			return nil, fmt.Errorf("no extractable partitions found in delta OTA")
		}
	} else {
		if len(opts.Partitions) == 0 {
			return nil, fmt.Errorf("at least one partition (-p) or --all is required")
		}
		seen := make(map[string]struct{}, len(opts.Partitions))
		for _, raw := range opts.Partitions {
			name := strings.TrimSpace(raw)
			if name == "" {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			sp, ok := byName[name]
			if !ok {
				avail := make([]string, 0, len(info.Partitions))
				for _, p := range info.Partitions {
					avail = append(avail, p.Name)
				}
				return nil, fmt.Errorf("partition %q not found in payload (available: %s)", name, strings.Join(avail, ", "))
			}
			chosen = append(chosen, sp)
		}
		if len(chosen) == 0 {
			return nil, fmt.Errorf("at least one partition (-p) or --all is required")
		}
	}

	for _, sp := range chosen {
		if !isSafePartitionName(sp.update.Name) {
			return nil, fmt.Errorf("unsafe partition name %q in manifest", sp.update.Name)
		}
		if !sp.meta.Extractable {
			return nil, fmt.Errorf("cannot extract partition %q standalone: %s", sp.update.Name, sp.meta.UnsupportedReason)
		}
	}
	seenLower := make(map[string]string, len(chosen))
	for _, sp := range chosen {
		lk := strings.ToLower(sp.update.Name)
		if prev, dup := seenLower[lk]; dup {
			return nil, fmt.Errorf("case-colliding partition names %q and %q cannot be extracted together", prev, sp.update.Name)
		}
		seenLower[lk] = sp.update.Name
	}
	return chosen, nil
}

func isSafePartitionName(name string) bool {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return false
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c < 0x20 || strings.ContainsRune(`<>:"/\|?*`, rune(c)) {
			return false
		}
	}
	stem := strings.ToUpper(name)
	if dot := strings.IndexByte(stem, '.'); dot != -1 {
		stem = stem[:dot]
	}
	switch stem {
	case "CON", "PRN", "AUX", "NUL":
		return false
	}
	if len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9' {
		return false
	}
	return true
}

func isSequentialPartition(ops []installOp) bool {
	var nextBlock uint64
	for _, op := range ops {
		if len(op.DstExtents) != 1 {
			return false
		}
		ext := op.DstExtents[0]
		if ext.StartBlock != nextBlock || nextBlock+ext.NumBlocks < nextBlock {
			return false
		}
		nextBlock += ext.NumBlocks
	}
	return true
}

func tryInspectBootMagic(filePath string) (*BootImageInfo, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	var magic [8]byte
	_, err = io.ReadFull(f, magic[:])
	_ = f.Close()
	if err != nil || string(magic[:]) != BootMagic {
		return nil, fmt.Errorf("not a boot image")
	}
	return InspectBootImage(filePath)
}
