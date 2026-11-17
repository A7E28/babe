package image

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
)

func extractRandomAccess(
	ctx context.Context,
	pool *sharedOpPool,
	part partitionUpdate,
	outFile *os.File,
	tmpPath string,
	verify bool,
	codecs *payloadCodecPool,
) (string, uint64, error) {
	if part.NewInfo.Size > uint64(math.MaxInt64) {
		return "", 0, fmt.Errorf("partition size %d overflows int64", part.NewInfo.Size)
	}
	sparseMarked := false
	if part.NewInfo.Size > 0 {
		sparseMarked = markFileSparse(outFile)
		if err := outFile.Truncate(int64(part.NewInfo.Size)); err != nil {
			return "", 0, fmt.Errorf("allocate output file: %w", err)
		}
	}

	if hasOverlappingExtents(part.Operations) {
		return extractRandomOrdered(ctx, pool, part, outFile, tmpPath, verify, codecs)
	}
	return extractRandomDisjoint(ctx, pool, part, outFile, tmpPath, verify, codecs, sparseMarked)
}

func extractRandomDisjoint(
	ctx context.Context,
	pool *sharedOpPool,
	part partitionUpdate,
	outFile *os.File,
	tmpPath string,
	verify bool,
	codecs *payloadCodecPool,
	skipZero bool,
) (string, uint64, error) {
	numOps := len(part.Operations)
	if numOps > 0 {
		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		resultsCh := make(chan decodedOpChunk, pool.maxInFlight())
		submittedCh := submitOpTasks(runCtx, pool, part.Operations, verify, resultsCh)

		received := 0
		var firstErr error
		for done := 0; done < numOps; done++ {
			var res decodedOpChunk
			select {
			case res = <-resultsCh:
				received++
			case <-runCtx.Done():
				firstErr = runCtx.Err()
				goto cleanup
			}
			if res.err != nil {
				codecs.putOut(res.buf)
				pool.releaseSlot()
				firstErr = res.err
				cancel()
				break
			}
			op := part.Operations[res.idx]
			if wErr := writeOpExtentsAt(outFile, res.buf, res.n, res.isZero, op.DstExtents, pool.src.manifest.BlockSize, part.NewInfo.Size, skipZero); wErr != nil {
				codecs.putOut(res.buf)
				pool.releaseSlot()
				firstErr = wErr
				cancel()
				break
			}
			codecs.putOut(res.buf)
			pool.releaseSlot()
		}

	cleanup:
		cancel()
		drainResults(resultsCh, submittedCh, received, codecs, pool)
		if firstErr != nil {
			_ = outFile.Close()
			return "", 0, firstErr
		}
		if err := ctx.Err(); err != nil {
			_ = outFile.Close()
			return "", 0, err
		}
	}

	var fileSize uint64
	if fi, sErr := outFile.Stat(); sErr == nil {
		fileSize = uint64(fi.Size())
	}
	if err := outFile.Close(); err != nil {
		return "", 0, fmt.Errorf("close output file: %w", err)
	}
	if !verify {
		return "", fileSize, nil
	}
	sum, err := calculateFileSHA256Fast(tmpPath)
	return sum, fileSize, err
}

func writeOpExtentsAt(f *os.File, buf *[]byte, n uint64, isZero bool, exts []extent, blockSize uint32, maxFileSize uint64, skipZeroOnTruncate bool) error {
	if isZero && skipZeroOnTruncate && maxFileSize > 0 {
		return nil
	}
	bs := uint64(blockSize)
	if bs == 0 {
		return fmt.Errorf("invalid zero block size")
	}
	var cursor uint64
	for i := 0; i < len(exts); {
		ext := exts[i]
		if ext.StartBlock > uint64(math.MaxInt64)/bs || ext.NumBlocks > uint64(math.MaxInt64)/bs {
			return fmt.Errorf("extent block offset overflow")
		}
		runBlocks := ext.NumBlocks
		j := i + 1
		for j < len(exts) && exts[j].StartBlock == ext.StartBlock+runBlocks {
			if exts[j].NumBlocks > uint64(math.MaxInt64)/bs-runBlocks {
				return fmt.Errorf("extent block offset overflow")
			}
			runBlocks += exts[j].NumBlocks
			j++
		}
		i = j

		offsetU := ext.StartBlock * bs
		extBytesU := runBlocks * bs
		writeLenU := extBytesU
		if maxFileSize > 0 {
			if offsetU >= maxFileSize {
				cursor += extBytesU
				continue
			}
			if writeLenU > maxFileSize-offsetU {
				writeLenU = maxFileSize - offsetU
			}
		}
		if offsetU > uint64(math.MaxInt64)-writeLenU {
			return fmt.Errorf("extent write range overflows int64")
		}
		offset := int64(offsetU)
		if isZero {
			for written := uint64(0); written < writeLenU; {
				chunk := min(uint64(len(staticZeroBlock)), writeLenU-written)
				if _, err := f.WriteAt(staticZeroBlock[:int(chunk)], offset+int64(written)); err != nil {
					return err
				}
				written += chunk
			}
		} else {
			if writeLenU > uint64(math.MaxInt) || cursor+writeLenU > n {
				return fmt.Errorf("decoded operation buffer underflow (%d < %d)", n, cursor+writeLenU)
			}
			if _, err := f.WriteAt((*buf)[int(cursor):int(cursor+writeLenU)], offset); err != nil {
				return err
			}
		}
		cursor += extBytesU
	}
	return nil
}

func calculateFileSHA256Fast(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file for sha256: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, 1024*1024)
	if _, err := io.CopyBuffer(h, f, buf); err != nil {
		return "", fmt.Errorf("compute sha256: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
