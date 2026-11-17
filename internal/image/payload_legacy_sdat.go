package image

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	maxTransferListBytes = 32 * 1024 * 1024
	maxTransferRanges    = 1_000_000
	maxLegacyImageBlocks = (64 * 1024 * 1024 * 1024) / DefaultBlockSize // 64 GiB sparse safety cap
)

type transferRange struct {
	startBlock uint64
	endBlock   uint64
}

func parseLegacySdatSpec(part string, tf, df *zip.File, isBrotli, patchHasData bool) (legacyPartitionSpec, error) {
	rc, err := tf.Open()
	if err != nil {
		return legacyPartitionSpec{}, err
	}
	defer rc.Close()

	codec, extractable, reason := classifySdatSpec(df, isBrotli, patchHasData)
	sp := legacyPartitionSpec{
		name:          part,
		kind:          "sdat",
		datFile:       df,
		transferFile:  tf,
		isBrotli:      isBrotli,
		extractable:   extractable,
		unavailReason: reason,
		codecs:        []string{codec},
	}
	if df == nil {
		sp.kind = "delta-only"
	} else {
		sp.datEntryPath = df.Name
	}

	lr := io.LimitReader(rc, maxTransferListBytes+1)
	br := bufio.NewReaderSize(lr, 64*1024)
	var bytesRead int64
	lineIdx := 0
	var version uint64
	var totalNewBlocks uint64

	for {
		rawLine, rErr := br.ReadString('\n')
		bytesRead += int64(len(rawLine))
		if bytesRead > maxTransferListBytes {
			return sp, fmt.Errorf("transfer.list exceeds %d-byte limit", maxTransferListBytes)
		}
		if rErr != nil && rErr != io.EOF {
			return sp, rErr
		}

		line := strings.TrimSpace(rawLine)
		if line != "" {
			lineIdx++
			if err := applyTransferListLine(&sp, lineIdx, &version, &totalNewBlocks, line); err != nil {
				return sp, err
			}
		}
		if rErr == io.EOF {
			break
		}
	}

	if lineIdx < 2 || (version >= 2 && lineIdx < 4) {
		return sp, fmt.Errorf("truncated transfer.list header (only %d lines)", lineIdx)
	}
	if sp.maxBlock > maxLegacyImageBlocks {
		return sp, fmt.Errorf("transfer.list block count %d exceeds %d-block limit", sp.maxBlock, maxLegacyImageBlocks)
	}
	sp.size = sp.maxBlock * DefaultBlockSize
	return sp, nil
}

func classifySdatSpec(df *zip.File, isBrotli, patchHasData bool) (string, bool, string) {
	if df == nil {
		return "delta", false, "incremental block OTA (requires existing partition blocks)"
	}
	if df.Method != zip.Store && df.Method != zip.Deflate {
		return fmt.Sprintf("sdat-zip-method-%d", df.Method), false, fmt.Sprintf("unsupported zip compression method %d on %s", df.Method, df.Name)
	}
	codec := "sdat-deflate"
	if isBrotli {
		codec = "sdat-brotli"
	} else if df.Method == zip.Store {
		codec = "sdat-raw"
	}
	if patchHasData {
		return codec, false, "incremental block OTA (requires existing partition blocks)"
	}
	return codec, true, ""
}

func applyTransferListLine(sp *legacyPartitionSpec, lineIdx int, version, totalNewBlocks *uint64, line string) error {
	switch lineIdx {
	case 1:
		v, err := strconv.ParseUint(line, 10, 64)
		if err != nil || v == 0 || v > 4 {
			return fmt.Errorf("invalid transfer.list version %q", line)
		}
		*version = v
		return nil
	case 2:
		tb, err := strconv.ParseUint(line, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid transfer.list total_blocks %q", line)
		}
		if tb > sp.maxBlock {
			sp.maxBlock = tb
		}
		return nil
	case 3, 4:
		if *version >= 2 {
			if _, err := strconv.ParseUint(line, 10, 64); err != nil {
				return fmt.Errorf("invalid transfer.list v%d header line %q", *version, line)
			}
			return nil
		}
	}

	cmd, rest := splitTransferCmd(line)
	switch cmd {
	case "new":
		ranges, maxB, rangeBlocks, err := parseTransferRanges(rest)
		if err != nil {
			return err
		}
		if len(sp.newRanges)+len(ranges) > maxTransferRanges {
			return fmt.Errorf("transfer.list range count exceeds limit (%d)", maxTransferRanges)
		}
		if *totalNewBlocks+rangeBlocks < *totalNewBlocks || *totalNewBlocks+rangeBlocks > maxLegacyImageBlocks {
			return fmt.Errorf("transfer.list cumulative new blocks exceed %d-block limit", maxLegacyImageBlocks)
		}
		*totalNewBlocks += rangeBlocks
		sp.newRanges = append(sp.newRanges, ranges...)
		if maxB > sp.maxBlock {
			sp.maxBlock = maxB
		}
	case "zero", "erase":
		_, maxB, _, err := parseTransferRanges(rest)
		if err != nil {
			return err
		}
		if maxB > sp.maxBlock {
			sp.maxBlock = maxB
		}
	case "compute_hash_tree", "compute_fec":
		// Post-write verity/FEC metadata; does not consume .new.dat bytes.
	default:
		if sp.extractable {
			sp.extractable = false
			sp.unavailReason = fmt.Sprintf("requires source blocks from device (transfer.list op %q)", cmd)
		}
	}
	return nil
}

func splitTransferCmd(line string) (string, string) {
	idx := strings.IndexAny(line, " \t")
	if idx < 0 {
		return line, ""
	}
	return line[:idx], strings.TrimSpace(line[idx+1:])
}

func parseTransferRanges(arg string) ([]transferRange, uint64, uint64, error) {
	parts := strings.Split(strings.TrimSpace(arg), ",")
	if len(parts) < 1 {
		return nil, 0, 0, fmt.Errorf("invalid transfer.list range %q", arg)
	}
	count, err := strconv.Atoi(parts[0])
	if err != nil || count < 0 || count%2 != 0 || len(parts)-1 != count {
		return nil, 0, 0, fmt.Errorf("invalid transfer.list range count in %q", arg)
	}
	if count == 0 {
		return nil, 0, 0, nil
	}
	ranges := make([]transferRange, 0, count/2)
	var maxBlock uint64
	var sumBlocks uint64
	for i := 1; i < len(parts); i += 2 {
		s, err1 := strconv.ParseUint(parts[i], 10, 64)
		e, err2 := strconv.ParseUint(parts[i+1], 10, 64)
		if err1 != nil || err2 != nil || e < s {
			return nil, 0, 0, fmt.Errorf("invalid block bounds in %q", arg)
		}
		span := e - s
		if sumBlocks+span < sumBlocks {
			return nil, 0, 0, fmt.Errorf("transfer.list range block sum overflow")
		}
		sumBlocks += span
		ranges = append(ranges, transferRange{startBlock: s, endBlock: e})
		if e > maxBlock {
			maxBlock = e
		}
	}
	return ranges, maxBlock, sumBlocks, nil
}
