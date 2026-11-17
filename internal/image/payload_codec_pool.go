package image

import (
	"bufio"
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

const (
	defaultChunkPoolCap = 2 * 1024 * 1024   // 2 MiB standard AOSP chunk
	maxPoolRetainBytes  = 8 * 1024 * 1024   // Cap sync.Pool retention to avoid pinning huge buffers
	maxOpOutputBytes    = 256 * 1024 * 1024 // Covers 200 MiB AOSP chunks & whole-partition boot ops
)

type payloadCodecPool struct {
	bufPool sync.Pool
	bwPool  sync.Pool
	xzCfg   xz.ReaderConfig

	zstdOnce sync.Once
	zstdDec  *zstd.Decoder
	zstdErr  error
}

func newPayloadCodecPool() *payloadCodecPool {
	return &payloadCodecPool{
		bufPool: sync.Pool{
			New: func() any {
				b := make([]byte, 0, defaultChunkPoolCap)
				return &b
			},
		},
		bwPool: sync.Pool{
			New: func() any {
				return bufio.NewWriterSize(io.Discard, 256*1024)
			},
		},
		xzCfg: xz.ReaderConfig{
			// In ulikunitz/xz, DictCap is a minimum buffer size; MinDictCap lets the stream header size the dict.
			DictCap: lzma.MinDictCap,
		},
	}
}

func (p *payloadCodecPool) Close() {
	if p.zstdDec != nil {
		p.zstdDec.Close()
	}
}

func (p *payloadCodecPool) getBuf(size int) *[]byte {
	if size > maxPoolRetainBytes {
		b := make([]byte, size)
		return &b
	}
	bp := p.bufPool.Get().(*[]byte)
	if cap(*bp) < size {
		b := make([]byte, size)
		*bp = b
	} else {
		*bp = (*bp)[:size]
	}
	return bp
}

func (p *payloadCodecPool) putBuf(bp *[]byte) {
	if bp == nil || cap(*bp) > maxPoolRetainBytes {
		return
	}
	*bp = (*bp)[:0]
	p.bufPool.Put(bp)
}

func (p *payloadCodecPool) getComp(size int) *[]byte  { return p.getBuf(size) }
func (p *payloadCodecPool) putComp(bp *[]byte)        { p.putBuf(bp) }
func (p *payloadCodecPool) getOut(size int) *[]byte   { return p.getBuf(size) }
func (p *payloadCodecPool) putOut(bp *[]byte)         { p.putBuf(bp) }

func (p *payloadCodecPool) getBufWriter(w io.Writer) *bufio.Writer {
	bw := p.bwPool.Get().(*bufio.Writer)
	bw.Reset(w)
	return bw
}

func (p *payloadCodecPool) putBufWriter(bw *bufio.Writer) {
	if bw == nil {
		return
	}
	bw.Reset(io.Discard)
	p.bwPool.Put(bw)
}

func (p *payloadCodecPool) getZstdDecoder() (*zstd.Decoder, error) {
	p.zstdOnce.Do(func() {
		p.zstdDec, p.zstdErr = zstd.NewReader(
			nil,
			zstd.WithDecoderConcurrency(0),
			zstd.WithDecoderMaxMemory(maxOpOutputBytes),
			zstd.WithDecoderMaxWindow(maxOpOutputBytes),
		)
	})
	return p.zstdDec, p.zstdErr
}
