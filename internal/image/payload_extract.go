package image

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

var staticZeroBlock [64 * 1024]byte

func ExtractPayload(ctx context.Context, filePath string, opts PayloadExtractOptions) (*PayloadExtractResult, error) {
	src, err := openPayloadSource(filePath, false)
	if err != nil {
		return nil, err
	}
	defer src.Close()
	return extractFromSource(ctx, src, opts)
}

func extractFromSource(ctx context.Context, src *payloadSource, opts PayloadExtractOptions) (*PayloadExtractResult, error) {
	if opts.Jobs < 0 {
		return nil, fmt.Errorf("worker jobs count (%d) must be >= 0", opts.Jobs)
	}
	if opts.All && len(opts.Partitions) > 0 {
		return nil, fmt.Errorf("cannot combine --all with -p / --partition")
	}
	if src.legacyZip != nil {
		return extractLegacyZip(ctx, src, opts)
	}

	start := time.Now()
	targets, err := selectTargetPartitions(src.manifest, src.info, opts)
	if err != nil {
		return nil, err
	}

	outDir := opts.OutputDir
	if strings.TrimSpace(outDir) == "" {
		outDir = "."
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}
	if opts.NoClobber {
		for _, t := range targets {
			dest := filepath.Join(outDir, t.update.Name+".img")
			if _, statErr := os.Lstat(dest); statErr == nil {
				return nil, fmt.Errorf("output file %q already exists (--no-clobber)", dest)
			}
		}
	}

	jobs := opts.Jobs
	if jobs <= 0 {
		jobs = runtime.NumCPU()
	}

	codecs := newPayloadCodecPool()
	defer codecs.Close()

	items, totalBytes, err := runTargetExtractions(ctx, src, targets, outDir, opts, codecs, jobs)
	if err != nil {
		return nil, err
	}

	return &PayloadExtractResult{
		FilePath:   src.info.FilePath,
		SourceType: src.info.SourceType,
		OTAType:    src.info.OTAType,
		Jobs:       jobs,
		Extracted:  items,
		TotalBytes: totalBytes,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

func runTargetExtractions(
	ctx context.Context,
	src *payloadSource,
	targets []selectedPartition,
	outDir string,
	opts PayloadExtractOptions,
	codecs *payloadCodecPool,
	jobs int,
) ([]PayloadExtractItem, uint64, error) {
	pool, err := newSharedOpPool(ctx, src, codecs, jobs)
	if err != nil {
		return nil, 0, err
	}
	defer pool.Close()

	type indexedTarget struct {
		origIdx int
		target  selectedPartition
	}
	ordered := make([]indexedTarget, len(targets))
	for i, t := range targets {
		ordered[i] = indexedTarget{origIdx: i, target: t}
	}
	// Largest-first overlaps small/medium partitions across tails without long tail stalls.
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].target.meta.Size > ordered[j].target.meta.Size
	})

	items := make([]PayloadExtractItem, len(targets))
	concParts := min(len(ordered), min(jobs, 8))
	partSem := make(chan struct{}, max(1, concParts))

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var errOnce sync.Once
	var firstErr error
	var wg sync.WaitGroup

	for _, entry := range ordered {
		select {
		case partSem <- struct{}{}:
		case <-runCtx.Done():
			break
		}
		if runCtx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(e indexedTarget) {
			defer wg.Done()
			defer func() { <-partSem }()
			item, exErr := extractOnePartition(runCtx, pool, e.target.update, e.target.meta, outDir, opts, codecs)
			if exErr != nil {
				errOnce.Do(func() {
					firstErr = exErr
					cancel()
				})
				return
			}
			items[e.origIdx] = item
		}(entry)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, 0, firstErr
	}
	if err := runCtx.Err(); err != nil {
		return nil, 0, err
	}

	var totalBytes uint64
	for _, it := range items {
		totalBytes += it.Size
	}
	return items, totalBytes, nil
}

func extractOnePartition(
	ctx context.Context,
	pool *sharedOpPool,
	part partitionUpdate,
	meta PayloadPartition,
	outDir string,
	opts PayloadExtractOptions,
	codecs *payloadCodecPool,
) (PayloadExtractItem, error) {
	start := time.Now()
	finalPath := filepath.Join(outDir, part.Name+".img")
	tmpFile, err := os.CreateTemp(outDir, part.Name+".*.tmp")
	if err != nil {
		return PayloadExtractItem{}, fmt.Errorf("create temp file for %s: %w", part.Name, err)
	}
	tmpPath := tmpFile.Name()

	committed := false
	defer func() {
		if !committed {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	var actualSHA string
	writtenSize := part.NewInfo.Size
	if isSequentialPartition(part.Operations) {
		var seqBytes uint64
		actualSHA, seqBytes, err = extractSequential(ctx, pool, part, tmpFile, opts.Verify, codecs)
		if writtenSize == 0 {
			writtenSize = seqBytes
		}
	} else {
		var randBytes uint64
		actualSHA, randBytes, err = extractRandomAccess(ctx, pool, part, tmpFile, tmpPath, opts.Verify, codecs)
		if writtenSize == 0 {
			writtenSize = randBytes
		}
	}
	if err != nil {
		return PayloadExtractItem{}, fmt.Errorf("extract %s: %w", part.Name, err)
	}

	expectedSHA := hex.EncodeToString(part.NewInfo.SHA256)
	verified := false
	if opts.Verify {
		if expectedSHA == "" {
			return PayloadExtractItem{}, fmt.Errorf("extract %s: manifest missing expected partition SHA-256 hash", part.Name)
		}
		if !strings.EqualFold(actualSHA, expectedSHA) {
			return PayloadExtractItem{}, fmt.Errorf("extract %s: SHA-256 mismatch (got %s, want %s)", part.Name, actualSHA, expectedSHA)
		}
		verified = true
	}

	if err := commitTempFile(tmpPath, finalPath, opts.NoClobber); err != nil {
		return PayloadExtractItem{}, fmt.Errorf("finalize %s: %w", finalPath, err)
	}
	committed = true

	var bootInfo *BootImageInfo
	if opts.InspectBoot {
		if bi, bErr := tryInspectBootMagic(finalPath); bErr == nil {
			bootInfo = bi
		}
	}

	return PayloadExtractItem{
		Partition:  part.Name,
		OutputPath: finalPath,
		Size:       writtenSize,
		SHA256:     actualSHA,
		Verified:   verified,
		DurationMs: time.Since(start).Milliseconds(),
		Codecs:     meta.Codecs,
		BootInfo:   bootInfo,
	}, nil
}
