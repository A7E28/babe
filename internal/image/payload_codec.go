package image

import (
	"bytes"
	"compress/bzip2"
	"crypto/sha256"
	"fmt"
	"io"
	"math"
)

func (p *payloadCodecPool) decodeOp(r io.ReaderAt, payloadSize, dataStart int64, blockSize uint32, op installOp, verify bool) (*[]byte, uint64, bool, error) {
	extentBytesU64, err := sumExtentBytes(op.DstExtents, blockSize)
	if err != nil {
		return nil, 0, false, err
	}

	if op.Type == OpZero || op.Type == OpDiscard {
		if extentBytesU64 > maxLegacyRawImgBytes {
			return nil, 0, false, fmt.Errorf("zero/discard operation %d bytes exceeds %d-byte limit", extentBytesU64, maxLegacyRawImgBytes)
		}
		return nil, extentBytesU64, true, nil
	}
	if !op.Type.IsExtractable() {
		return nil, 0, false, fmt.Errorf("unsupported delta operation %s", op.Type.String())
	}
	if extentBytesU64 > maxOpOutputBytes || extentBytesU64 > uint64(math.MaxInt) {
		return nil, 0, false, fmt.Errorf("operation output %d bytes exceeds safety limit", extentBytesU64)
	}
	extentBytes := int(extentBytesU64)

	if op.DataLength > uint64(maxOpOutputBytes) || op.DataLength > uint64(math.MaxInt) {
		return nil, 0, false, fmt.Errorf("operation blob %d bytes exceeds safety limit", op.DataLength)
	}
	if op.DataOffset > uint64(math.MaxInt64) {
		return nil, 0, false, fmt.Errorf("operation data offset overflow")
	}
	avail := payloadSize - dataStart
	if avail < 0 || int64(op.DataLength) > avail || int64(op.DataOffset) > avail-int64(op.DataLength) {
		return nil, 0, false, fmt.Errorf("operation data bounds exceed payload size %d", payloadSize)
	}
	absOffset := dataStart + int64(op.DataOffset)

	compLen := int(op.DataLength)
	compPtr := p.getComp(compLen)
	compBuf := *compPtr
	if _, err := io.ReadFull(io.NewSectionReader(r, absOffset, int64(compLen)), compBuf); err != nil {
		p.putComp(compPtr)
		return nil, 0, false, fmt.Errorf("read %s blob: %w", op.Type.String(), err)
	}

	if verify {
		if len(op.DataSHA256Hash) > 0 {
			sum := sha256.Sum256(compBuf)
			if !bytes.Equal(sum[:], op.DataSHA256Hash) {
				p.putComp(compPtr)
				return nil, 0, false, fmt.Errorf("operation %s blob SHA-256 mismatch", op.Type.String())
			}
		} else {
			p.putComp(compPtr)
			return nil, 0, false, fmt.Errorf("operation %s missing required SHA-256 hash", op.Type.String())
		}
	}

	if op.Type == OpReplace {
		return p.decodeRawOp(compPtr, extentBytes, op.DstLength)
	}

	outPtr := p.getOut(extentBytes)
	defer p.putComp(compPtr)

	if err := p.decompressOp(op, compBuf, outPtr, extentBytes); err != nil {
		p.putOut(outPtr)
		return nil, 0, false, err
	}
	return outPtr, extentBytesU64, false, nil
}

func (p *payloadCodecPool) decodeRawOp(compPtr *[]byte, extentBytes int, dstLength uint64) (*[]byte, uint64, bool, error) {
	n := len(*compPtr)
	if n == extentBytes {
		return compPtr, uint64(extentBytes), false, nil
	}
	if dstLength > 0 && uint64(n) == dstLength && n < extentBytes {
		outPtr := p.getOut(extentBytes)
		copy(*outPtr, *compPtr)
		clear((*outPtr)[n:extentBytes])
		p.putComp(compPtr)
		return outPtr, uint64(extentBytes), false, nil
	}
	p.putComp(compPtr)
	return nil, 0, false, fmt.Errorf("raw REPLACE length %d does not match expected %d bytes", n, extentBytes)
}

func (p *payloadCodecPool) decompressOp(op installOp, compBuf []byte, outPtr *[]byte, extentBytes int) error {
	outBuf := *outPtr
	switch op.Type {
	case OpReplaceBZ:
		return readExactOrDstLen(bzip2.NewReader(bytes.NewReader(compBuf)), outBuf[:extentBytes], op.DstLength, "bzip2")
	case OpReplaceXZ:
		xzr, err := p.xzCfg.NewReader(bytes.NewReader(compBuf))
		if err != nil {
			return fmt.Errorf("init xz reader: %w", err)
		}
		return readExactOrDstLen(xzr, outBuf[:extentBytes], op.DstLength, "xz")
	case OpReplaceZstd:
		dec, err := p.getZstdDecoder()
		if err != nil {
			return fmt.Errorf("init zstd decoder: %w", err)
		}
		decoded, err := dec.DecodeAll(compBuf, outBuf[:0])
		if err != nil {
			return fmt.Errorf("zstd decode: %w", err)
		}
		if len(decoded) == extentBytes {
			*outPtr = decoded
			return nil
		}
		if op.DstLength > 0 && uint64(len(decoded)) == op.DstLength && len(decoded) < extentBytes {
			if cap(decoded) < extentBytes {
				grown := make([]byte, extentBytes)
				copy(grown, decoded)
				*outPtr = grown
				return nil
			}
			decoded = decoded[:extentBytes]
			clear(decoded[op.DstLength:])
			*outPtr = decoded
			return nil
		}
		return fmt.Errorf("zstd decoded %d bytes, expected %d", len(decoded), extentBytes)
	default:
		return fmt.Errorf("unsupported codec %s", op.Type.String())
	}
}

func readExactOrDstLen(r io.Reader, dst []byte, dstLength uint64, codec string) error {
	n, err := io.ReadFull(r, dst)
	if err == nil {
		var extra [1]byte
		en, rErr := r.Read(extra[:])
		if en > 0 {
			return fmt.Errorf("%s stream exceeded expected %d bytes", codec, len(dst))
		}
		if rErr != nil && rErr != io.EOF {
			return fmt.Errorf("%s stream trailer error: %w", codec, rErr)
		}
		return nil
	}
	if err == io.ErrUnexpectedEOF && dstLength > 0 && uint64(n) == dstLength && n < len(dst) {
		clear(dst[n:])
		return nil
	}
	return fmt.Errorf("%s decode (got %d/%d bytes): %w", codec, n, len(dst), err)
}

func sumExtentBytes(exts []extent, blockSize uint32) (uint64, error) {
	if blockSize == 0 {
		return 0, fmt.Errorf("invalid zero block size")
	}
	var blocks uint64
	for _, ext := range exts {
		if blocks+ext.NumBlocks < blocks {
			return 0, fmt.Errorf("extent block count overflow")
		}
		blocks += ext.NumBlocks
	}
	total := blocks * uint64(blockSize)
	if blocks > 0 && total/uint64(blockSize) != blocks {
		return 0, fmt.Errorf("extent block size overflow")
	}
	return total, nil
}
