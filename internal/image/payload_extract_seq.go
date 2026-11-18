package image

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"math"
	"os"
)

type decodedOpChunk struct {
	idx    int
	buf    *[]byte
	n      uint64
	isZero bool
	err    error
}

func extractSequential(
	ctx context.Context,
	pool *sharedOpPool,
	part partitionUpdate,
	outFile *os.File,
	verify bool,
	codecs *payloadCodecPool,
) (string, uint64, error) {
	bw := codecs.getBufWriter(outFile)
	defer codecs.putBufWriter(bw)

	var hasher hash.Hash
	var sink io.Writer = bw
	if verify {
		hasher = sha256.New()
		sink = io.MultiWriter(bw, hasher)
	}

	remaining := part.NewInfo.Size
	boundedSize := remaining > 0
	sparseZeroSkip := false
	if !verify && boundedSize && part.NewInfo.Size <= uint64(math.MaxInt64) {
		if markFileSparse(outFile) {
			if err := outFile.Truncate(int64(part.NewInfo.Size)); err == nil {
				sparseZeroSkip = true
			}
		}
	}

	numOps := len(part.Operations)
	if numOps == 0 {
		if err := ctx.Err(); err != nil {
			return "", 0, err
		}
		if part.NewInfo.Size > 0 && !sparseZeroSkip {
			if err := writeZeroStream64(sink, part.NewInfo.Size); err != nil {
				return "", 0, err
			}
		}
		if err := bw.Flush(); err != nil {
			return "", 0, fmt.Errorf("flush output file: %w", err)
		}
		if err := outFile.Close(); err != nil {
			return "", 0, fmt.Errorf("close output file: %w", err)
		}
		if !verify {
			return "", part.NewInfo.Size, nil
		}
		return hex.EncodeToString(hasher.Sum(nil)), part.NewInfo.Size, nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	resultsCh := make(chan decodedOpChunk, pool.maxInFlight())
	submittedCh := submitOpTasks(runCtx, pool, part.Operations, verify, resultsCh)

	pending := make(map[int]decodedOpChunk, pool.maxInFlight())
	nextIdx := 0
	received := 0
	var writtenTotal uint64
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
			if firstErr == nil {
				firstErr = res.err
				cancel()
			}
			goto done
		}
		pending[res.idx] = res
		for {
			chunk, ok := pending[nextIdx]
			if !ok {
				break
			}
			delete(pending, nextIdx)
			writeLen := clampWriteLen64(chunk.n, boundedSize, &remaining)
			if writeLen > 0 {
				var wErr error
				if chunk.isZero && sparseZeroSkip {
					if err := bw.Flush(); err != nil {
						wErr = err
					} else if _, err := outFile.Seek(int64(writeLen), io.SeekCurrent); err != nil {
						wErr = err
					}
				} else if chunk.isZero {
					wErr = writeZeroStream64(sink, writeLen)
				} else {
					_, wErr = sink.Write((*chunk.buf)[:int(writeLen)])
				}
				if wErr != nil {
					firstErr = fmt.Errorf("write output: %w", wErr)
					cancel()
					codecs.putOut(chunk.buf)
					pool.releaseSlot()
					goto done
				}
				writtenTotal += writeLen
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

	if firstErr != nil {
		return "", 0, firstErr
	}
	if err := ctx.Err(); err != nil {
		return "", 0, err
	}
	if boundedSize && remaining > 0 {
		if !sparseZeroSkip {
			if err := writeZeroStream64(sink, remaining); err != nil {
				return "", 0, fmt.Errorf("pad trailing zeros: %w", err)
			}
		}
		writtenTotal += remaining
	}
	if err := bw.Flush(); err != nil {
		return "", 0, fmt.Errorf("flush output file: %w", err)
	}
	if err := outFile.Close(); err != nil {
		return "", 0, fmt.Errorf("close output file: %w", err)
	}
	if !verify {
		return "", writtenTotal, nil
	}
	return hex.EncodeToString(hasher.Sum(nil)), writtenTotal, nil
}

func submitOpTasks(ctx context.Context, pool *sharedOpPool, ops []installOp, verify bool, resultsCh chan<- decodedOpChunk) <-chan int {
	submittedCh := make(chan int, 1)
	go func() {
		submitted := 0
		for idx, op := range ops {
			if !pool.acquireSlot(ctx) {
				break
			}
			if !pool.submit(opDecodeTask{idx: idx, op: op, verify: verify, resultCh: resultsCh, ctx: ctx}) {
				break
			}
			submitted++
		}
		submittedCh <- submitted
	}()
	return submittedCh
}

func drainResults(ch <-chan decodedOpChunk, submittedCh <-chan int, received int, codecs *payloadCodecPool, pool *sharedOpPool) {
	submitted := <-submittedCh
	for received < submitted {
		res := <-ch
		codecs.putOut(res.buf)
		pool.releaseSlot()
		received++
	}
}

func clampWriteLen64(chunkLen uint64, bounded bool, remaining *uint64) uint64 {
	if !bounded {
		return chunkLen
	}
	if *remaining == 0 {
		return 0
	}
	if chunkLen > *remaining {
		chunkLen = *remaining
	}
	*remaining -= chunkLen
	return chunkLen
}

func writeZeroStream64(w io.Writer, total uint64) error {
	for remaining := total; remaining > 0; {
		chunk := uint64(len(staticZeroBlock))
		if remaining < chunk {
			chunk = remaining
		}
		if _, err := w.Write(staticZeroBlock[:int(chunk)]); err != nil {
			return err
		}
		remaining -= chunk
	}
	return nil
}
