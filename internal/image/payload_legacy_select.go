package image

import (
	"fmt"
	"strings"
)

func selectLegacyTargets(src *payloadSource, opts PayloadExtractOptions) ([]legacyPartitionSpec, error) {
	var chosen []legacyPartitionSpec
	extractAll := opts.All
	if !extractAll && len(opts.Partitions) == 1 && strings.EqualFold(strings.TrimSpace(opts.Partitions[0]), "all") {
		if _, hasLiteralAll := src.legacySpec[strings.TrimSpace(opts.Partitions[0])]; !hasLiteralAll {
			extractAll = true
		}
	}
	if extractAll {
		for _, p := range src.info.Partitions {
			if p.Extractable {
				chosen = append(chosen, src.legacySpec[p.Name])
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
			sp, ok := src.legacySpec[name]
			if !ok {
				for k, candidate := range src.legacySpec {
					if strings.EqualFold(k, name) {
						sp = candidate
						ok = true
						break
					}
				}
			}
			if !ok {
				avail := make([]string, 0, len(src.info.Partitions))
				for _, p := range src.info.Partitions {
					avail = append(avail, p.Name)
				}
				return nil, fmt.Errorf("partition %q not found in legacy OTA (available: %s)", name, strings.Join(avail, ", "))
			}
			canonKey := strings.ToLower(sp.name)
			if _, dup := seen[canonKey]; dup {
				continue
			}
			seen[canonKey] = struct{}{}
			chosen = append(chosen, sp)
		}
		if len(chosen) == 0 {
			return nil, fmt.Errorf("at least one partition (-p) or --all is required")
		}
	}

	for _, sp := range chosen {
		if !isSafePartitionName(sp.name) {
			return nil, fmt.Errorf("unsafe partition name %q in archive", sp.name)
		}
		if !sp.extractable {
			return nil, fmt.Errorf("cannot extract partition %q standalone: %s", sp.name, sp.unavailReason)
		}
	}
	return chosen, nil
}
