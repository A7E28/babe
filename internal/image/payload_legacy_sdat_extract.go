package image

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/andybalholm/brotli"
)

const maxLegacyRawImgBytes = uint64(maxLegacyImageBlocks) * DefaultBlockSize

func extractLegacyRawImg(ctx context.Context, zf *zip.File, expectedSize uint64, out *os.File, verify bool, copyBuf []byte) (string, uint64, error) {
	if zf == nil {
		return "", 0, fmt.Errorf("missing zip entry")
	}
	if expectedSize > maxLegacyRawImgBytes || expectedSize > uint64(math.MaxInt64-1) {
		return "", 0, fmt.Errorf("zip entry %s size %d exceeds %d-byte limit", zf.Name, expectedSize, maxLegacyRawImgBytes)
	}
	rc, err := zf.Open()
	if err != nil {
		return "", 0, err
	}
	defer rc.Close()

	bw := bufio.NewWriterSize(out, 256*1024)
	var dst io.Writer = bw
	var h io.Writer
	var shaHex string
	if verify {
		hasher := sha256.New()
		h = hasher
		dst = io.MultiWriter(bw, h)
	}

	r := io.LimitReader(ctxReader{ctx: ctx, r: rc}, int64(expectedSize)+1)
	n, err := copyWithBuf(dst, r, copyBuf)
	if err != nil {
		return "", 0, err
	}
	if uint64(n) != expectedSize {
		return "", 0, fmt.Errorf("zip entry %s size mismatch (got %d, want %d bytes)", zf.Name, n, expectedSize)
	}
	if err := bw.Flush(); err != nil {
		return "", 0, err
	}
	if err := out.Close(); err != nil {
		return "", 0, err
	}
	if verify {
		if sum, ok := h.(interface{ Sum([]byte) []byte }); ok {
			shaHex = hex.EncodeToString(sum.Sum(nil))
		}
	}
	return shaHex, uint64(n), nil
}

func extractLegacySdat(ctx context.Context, zf *zip.File, sp legacyPartitionSpec, out *os.File, tmpPath string, verify bool, copyBuf []byte) (string, uint64, error) {
	rc, err := zf.Open()
	if err != nil {
		return "", 0, err
	}
	defer rc.Close()

	var datReader io.Reader = ctxReader{ctx: ctx, r: rc}
	if sp.isBrotli {
		datReader = brotli.NewReader(datReader)
	}

	if sp.size > 0 {
		markFileSparse(out)
		if err := out.Truncate(int64(sp.size)); err != nil {
			return "", 0, fmt.Errorf("allocate output image: %w", err)
		}
	}

	bw := bufio.NewWriterSize(out, 256*1024)
	var curBlock uint64

	for _, rng := range sp.newRanges {
		if rng.startBlock != curBlock {
			if err := bw.Flush(); err != nil {
				return "", 0, err
			}
			if _, err := out.Seek(int64(rng.startBlock)*DefaultBlockSize, io.SeekStart); err != nil {
				return "", 0, err
			}
		}
		need := int64(rng.endBlock-rng.startBlock) * DefaultBlockSize
		copied, err := copyWithBuf(bw, io.LimitReader(datReader, need), copyBuf)
		if err != nil {
			return "", 0, err
		}
		if copied != need {
			return "", 0, fmt.Errorf("unexpected EOF in %s data stream (got %d/%d bytes)", sp.name, copied, need)
		}
		curBlock = rng.endBlock
	}
	if err := bw.Flush(); err != nil {
		return "", 0, err
	}

	var tail [1]byte
	if en, rErr := datReader.Read(tail[:]); en > 0 {
		return "", 0, fmt.Errorf("unexpected trailing bytes in %s data stream", sp.name)
	} else if rErr != nil && rErr != io.EOF {
		return "", 0, fmt.Errorf("verify %s stream integrity: %w", sp.name, rErr)
	}

	if err := out.Close(); err != nil {
		return "", 0, err
	}
	if !verify {
		return "", sp.size, nil
	}
	sum, err := calculateFileSHA256Fast(tmpPath)
	if err != nil {
		return "", 0, err
	}
	return sum, sp.size, nil
}

func copyWithBuf(dst io.Writer, src io.Reader, buf []byte) (int64, error) {
	var total int64
	for {
		nr, rErr := src.Read(buf)
		if nr > 0 {
			nw, wErr := dst.Write(buf[:nr])
			total += int64(nw)
			if wErr != nil {
				return total, wErr
			}
			if nw != nr {
				return total, io.ErrShortWrite
			}
		}
		if rErr != nil {
			if rErr == io.EOF {
				return total, nil
			}
			return total, rErr
		}
	}
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
