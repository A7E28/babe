package image

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
)

type PayloadSession struct {
	mu     sync.Mutex
	src    *payloadSource
	closed bool
}

func OpenPayload(filePath string) (*PayloadSession, error) {
	src, err := openPayloadSource(filePath, false)
	if err != nil {
		return nil, err
	}
	return &PayloadSession{src: src}, nil
}

func (s *PayloadSession) Info() *PayloadInfo {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.src == nil {
		return nil
	}
	return s.src.info
}

func (s *PayloadSession) Extract(ctx context.Context, opts PayloadExtractOptions) (*PayloadExtractResult, error) {
	if s == nil {
		return nil, fmt.Errorf("payload session is closed")
	}
	s.mu.Lock()
	if s.closed || s.src == nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("payload session is closed")
	}
	src := s.src
	s.mu.Unlock()
	return extractFromSource(ctx, src, opts)
}

func (s *PayloadSession) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.src == nil {
		return nil
	}
	s.closed = true
	err := s.src.Close()
	s.src = nil
	return err
}

func InspectPayload(filePath string) (*PayloadInfo, error) {
	src, err := openPayloadSource(filePath, true)
	if err != nil {
		return nil, err
	}
	defer src.Close()
	return src.info, nil
}

func (s *payloadSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.closer != nil {
		err := s.closer.Close()
		s.closer = nil
		return err
	}
	return nil
}

// openWorkerReader opens a dedicated read-only handle and verifies file identity against TOCTOU swaps.
func (s *payloadSource) openWorkerReader() (io.ReaderAt, func(), error) {
	s.mu.Lock()
	closed := s.closed
	srcPath := s.sourcePath
	initInfo := s.initialInfo
	expSize := s.fileSize
	zipOff := s.zipOffset
	sz := s.size
	fallback := s.reader
	s.mu.Unlock()

	if closed {
		return nil, func() {}, fmt.Errorf("payload source is closed")
	}
	if srcPath == "" {
		return fallback, func() {}, nil
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open worker payload handle: %w", err)
	}
	if initInfo != nil {
		fi, sErr := f.Stat()
		if sErr != nil {
			_ = f.Close()
			return nil, func() {}, fmt.Errorf("stat worker payload handle: %w", sErr)
		}
		if !os.SameFile(initInfo, fi) || fi.Size() != expSize {
			_ = f.Close()
			return nil, func() {}, fmt.Errorf("payload source file %q changed on disk during extraction", srcPath)
		}
	}
	if zipOff > 0 {
		return io.NewSectionReader(f, zipOff, sz), func() { _ = f.Close() }, nil
	}
	return f, func() { _ = f.Close() }, nil
}
