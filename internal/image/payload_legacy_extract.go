package image

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

func extractLegacyZip(ctx context.Context, src *payloadSource, opts PayloadExtractOptions) (*PayloadExtractResult, error) {
	start := time.Now()
	targets, err := selectLegacyTargets(src, opts)
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
		for _, sp := range targets {
			dest := filepath.Join(outDir, sp.name+".img")
			if _, statErr := os.Lstat(dest); statErr == nil {
				return nil, fmt.Errorf("output file %q already exists (--no-clobber)", dest)
			}
		}
	}

	// Largest-first prevents tail latency when big .new.dat.br/.new.dat partitions start last.
	sort.SliceStable(targets, func(i, j int) bool { return targets[i].size > targets[j].size })

	jobs := opts.Jobs
	if jobs <= 0 {
		jobs = runtime.NumCPU()
	}
	workers := min(jobs, len(targets))

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]PayloadExtractItem, len(targets))
	jobsCh := make(chan int, len(targets))
	for i := range targets {
		jobsCh <- i
	}
	close(jobsCh)

	var errOnce sync.Once
	var firstErr error
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wzr, closeZip, zErr := openWorkerLegacyZip(src)
			if zErr != nil {
				errOnce.Do(func() {
					firstErr = zErr
					cancel()
				})
				return
			}
			defer closeZip()

			buf := make([]byte, 256*1024)
			for idx := range jobsCh {
				if runCtx.Err() != nil {
					return
				}
				item, exErr := extractOneLegacyPartition(runCtx, wzr, targets[idx], outDir, opts.Verify, opts.InspectBoot, opts.NoClobber, buf)
				if exErr != nil {
					errOnce.Do(func() {
						firstErr = exErr
						cancel()
					})
					return
				}
				results[idx] = item
			}
		}()
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	if err := runCtx.Err(); err != nil {
		return nil, err
	}

	var totalBytes uint64
	for _, it := range results {
		totalBytes += it.Size
	}
	return &PayloadExtractResult{
		FilePath:   src.info.FilePath,
		SourceType: src.info.SourceType,
		OTAType:    src.info.OTAType,
		Jobs:       workers,
		Extracted:  results,
		TotalBytes: totalBytes,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

func openWorkerLegacyZip(src *payloadSource) (*zip.Reader, func(), error) {
	src.mu.Lock()
	closed := src.closed
	srcPath := src.sourcePath
	initInfo := src.initialInfo
	expSize := src.fileSize
	sz := src.size
	fallback := src.legacyZip
	src.mu.Unlock()

	if closed {
		return nil, func() {}, fmt.Errorf("payload source is closed")
	}
	if srcPath == "" {
		return fallback, func() {}, nil
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open worker zip handle: %w", err)
	}
	if initInfo != nil {
		fi, sErr := f.Stat()
		if sErr != nil {
			_ = f.Close()
			return nil, func() {}, fmt.Errorf("stat worker zip handle: %w", sErr)
		}
		if !os.SameFile(initInfo, fi) || fi.Size() != expSize {
			_ = f.Close()
			return nil, func() {}, fmt.Errorf("zip source file %q changed on disk during extraction", srcPath)
		}
	}
	zr, err := openZipWithFastFlate(f, sz)
	if err != nil {
		_ = f.Close()
		return nil, func() {}, fmt.Errorf("open worker zip reader: %w", err)
	}
	return zr, func() { _ = f.Close() }, nil
}

func lookupZipEntry(zr *zip.Reader, name string, fallback *zip.File) *zip.File {
	if zr == nil || name == "" {
		return fallback
	}
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}
	return fallback
}

func extractOneLegacyPartition(ctx context.Context, zr *zip.Reader, sp legacyPartitionSpec, outDir string, verify, inspectBoot, noClobber bool, copyBuf []byte) (PayloadExtractItem, error) {
	start := time.Now()
	finalPath := filepath.Join(outDir, sp.name+".img")
	tmpFile, err := os.CreateTemp(outDir, sp.name+".*.tmp")
	if err != nil {
		return PayloadExtractItem{}, fmt.Errorf("create temp file for %s: %w", sp.name, err)
	}
	tmpPath := tmpFile.Name()

	committed := false
	defer func() {
		if !committed {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	var shaHex string
	var outSize uint64
	if sp.kind == "raw-img" {
		zf := lookupZipEntry(zr, sp.entryPath, sp.imgFile)
		shaHex, outSize, err = extractLegacyRawImg(ctx, zf, sp.size, tmpFile, verify, copyBuf)
	} else {
		zf := lookupZipEntry(zr, sp.datEntryPath, sp.datFile)
		shaHex, outSize, err = extractLegacySdat(ctx, zf, sp, tmpFile, tmpPath, verify, copyBuf)
	}
	if err != nil {
		return PayloadExtractItem{}, fmt.Errorf("extract %s: %w", sp.name, err)
	}

	if err := commitTempFile(tmpPath, finalPath, noClobber); err != nil {
		return PayloadExtractItem{}, fmt.Errorf("finalize %s: %w", finalPath, err)
	}
	committed = true

	var bootInfo *BootImageInfo
	if inspectBoot {
		if bi, bErr := tryInspectBootMagic(finalPath); bErr == nil {
			bootInfo = bi
		}
	}

	return PayloadExtractItem{
		Partition:  sp.name,
		OutputPath: finalPath,
		Size:       outSize,
		SHA256:     shaHex,
		Verified:   false,
		DurationMs: time.Since(start).Milliseconds(),
		Codecs:     sp.codecs,
		BootInfo:   bootInfo,
	}, nil
}
