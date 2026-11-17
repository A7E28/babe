package image

import (
	"context"
	"fmt"
	"os"
	"sort"
)

func extractRandomOrdered(
	ctx context.Context,
	pool *sharedOpPool,
	part partitionUpdate,
	outFile *os.File,
	tmpPath string,
	verify bool,
	codecs *payloadCodecPool,
) (string, uint64, error) {
	numOps := len(part.Operations)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	resultsCh := make(chan decodedOpChunk, pool.maxInFlight())
	submittedCh := submitOpTasks(runCtx, pool, part.Operations, verify, resultsCh)

	pending := make(map[int]decodedOpChunk, pool.maxInFlight())
	nextIdx := 0
	received := 0
	var firstErr error
	for nextIdx < numOps {
		var res decodedOpChunk
		select {
		case res = <-resultsCh:
			received++
		case <-runCtx.Done():
			if firstErr == nil {
				firstErr = runCtx.Err()
			}
			goto done
		}
		if res.err != nil {
			codecs.putOut(res.buf)
			pool.releaseSlot()
			firstErr = res.err
			cancel()
			goto done
		}
		pending[res.idx] = res
		for {
			chunk, ok := pending[nextIdx]
			if !ok {
				break
			}
			delete(pending, nextIdx)
			if wErr := writeOpExtentsAt(outFile, chunk.buf, chunk.n, chunk.isZero, part.Operations[nextIdx].DstExtents, pool.src.manifest.BlockSize, part.NewInfo.Size, false); wErr != nil {
				codecs.putOut(chunk.buf)
				pool.releaseSlot()
				firstErr = wErr
				cancel()
				goto done
			}
			codecs.putOut(chunk.buf)
			pool.releaseSlot()
			nextIdx++
		}
	}

done:
	cancel()
	for _, leftover := range pending {
		codecs.putOut(leftover.buf)
		pool.releaseSlot()
	}
	drainResults(resultsCh, submittedCh, received, codecs, pool)

	var fileSize uint64
	if fi, sErr := outFile.Stat(); sErr == nil {
		fileSize = uint64(fi.Size())
	}
	if err := outFile.Close(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("close output file: %w", err)
	}
	if firstErr != nil {
		return "", 0, firstErr
	}
	if err := ctx.Err(); err != nil {
		return "", 0, err
	}
	if !verify {
		return "", fileSize, nil
	}
	sum, err := calculateFileSHA256Fast(tmpPath)
	return sum, fileSize, err
}

func hasOverlappingExtents(ops []installOp) bool {
	type span struct{ start, end uint64 }
	var spans []span
	for _, op := range ops {
		for _, ext := range op.DstExtents {
			if ext.NumBlocks == 0 {
				continue
			}
			if ext.StartBlock > ^uint64(0)-ext.NumBlocks {
				return true // Force deterministic path so writeOpExtentsAt reports overflow cleanly.
			}
			spans = append(spans, span{start: ext.StartBlock, end: ext.StartBlock + ext.NumBlocks})
		}
	}
	if len(spans) <= 1 {
		return false
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	for i := 1; i < len(spans); i++ {
		if spans[i].start < spans[i-1].end {
			return true
		}
	}
	return false
}
