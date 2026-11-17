package image

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	kflate "github.com/klauspost/compress/flate"
)

const maxManifestBytes = 128 * 1024 * 1024

type payloadSource struct {
	mu          sync.Mutex
	closed      bool
	sourcePath  string
	initialInfo os.FileInfo
	fileSize    int64
	zipOffset   int64
	reader      io.ReaderAt
	size        int64
	dataStart   int64
	manifest    *payloadManifest
	info        *PayloadInfo
	legacyZip   *zip.Reader
	legacySpec  map[string]legacyPartitionSpec
	closer      io.Closer
}

func openPayloadSource(filePath string, summaryOnly bool) (*payloadSource, error) {
	lower := strings.ToLower(filePath)
	if strings.HasSuffix(lower, ".tar.md5") || strings.HasSuffix(lower, ".tar") {
		return nil, fmt.Errorf("Samsung Odin archive (%s) does not use Android A/B payload.bin", filepath.Base(filePath))
	}
	if strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar.gz") {
		return nil, fmt.Errorf("compressed tar archive (%s) is not a random-access zip/payload.bin; unpack fastboot images with tar", filepath.Base(filePath))
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open payload source: %w", err)
	}

	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("stat payload source: %w", err)
	}

	var hdr [12]byte
	n, rErr := f.ReadAt(hdr[:], 0)
	if rErr != nil && rErr != io.EOF {
		_ = f.Close()
		return nil, fmt.Errorf("read file signature: %w", rErr)
	}
	if n < 4 {
		_ = f.Close()
		return nil, fmt.Errorf("file too small (%d bytes)", fi.Size())
	}
	if n == 12 && string(hdr[:]) == "OPPOENCRYPT!" {
		_ = f.Close()
		return nil, fmt.Errorf("encrypted OPPO/Realme .ozip (%s) is not supported without decryption key", filepath.Base(filePath))
	}

	if string(hdr[:4]) == PayloadMagic || hdr[0] != 'P' || hdr[1] != 'K' {
		if string(hdr[:4]) != PayloadMagic {
			_ = f.Close()
			return nil, fmt.Errorf("unsupported file format %q (expected CrAU payload.bin or OTA .zip)", filePath)
		}
		src, err := parsePayloadHeaderAndManifest(f, fi.Size(), filePath, "bin", nil, summaryOnly)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		src.sourcePath = filePath
		src.initialInfo = fi
		src.fileSize = fi.Size()
		src.closer = f
		return src, nil
	}

	zr, err := openZipWithFastFlate(f, fi.Size())
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("open OTA zip archive: %w", err)
	}
	pf, findErr := findPayloadZipEntry(zr)
	if findErr != nil {
		if isLegacyOTAZip(zr) {
			src, lErr := openLegacyZipSource(f, fi, zr, filePath)
			if lErr != nil {
				_ = f.Close()
				return nil, lErr
			}
			return src, nil
		}
		_ = f.Close()
		return nil, findErr
	}
	if pf.Method != zip.Store {
		_ = f.Close()
		return nil, fmt.Errorf("payload.bin inside %q uses compression method %d (expected Store/uncompressed)", filePath, pf.Method)
	}
	dataOffset, err := pf.DataOffset()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("resolve payload.bin zip offset: %w", err)
	}
	if pf.UncompressedSize64 > uint64(fi.Size()) {
		_ = f.Close()
		return nil, fmt.Errorf("invalid payload.bin bounds inside zip archive")
	}
	payloadSize := int64(pf.UncompressedSize64)
	if dataOffset < 0 || payloadSize < PayloadHeaderSize || dataOffset > fi.Size() || payloadSize > fi.Size()-dataOffset {
		_ = f.Close()
		return nil, fmt.Errorf("invalid payload.bin bounds inside zip archive")
	}
	src, err := parsePayloadHeaderAndManifest(io.NewSectionReader(f, dataOffset, payloadSize), payloadSize, filePath, "zip", readOTAZipMetadata(zr), summaryOnly)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	src.sourcePath = filePath
	src.initialInfo = fi
	src.fileSize = fi.Size()
	src.zipOffset = dataOffset
	src.closer = f
	return src, nil
}

func openZipWithFastFlate(r io.ReaderAt, size int64) (*zip.Reader, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	zr.RegisterDecompressor(zip.Deflate, func(in io.Reader) io.ReadCloser {
		return kflate.NewReader(in)
	})
	return zr, nil
}

func parsePayloadHeaderAndManifest(r io.ReaderAt, size int64, filePath, sourceType string, otaMeta map[string]string, summaryOnly bool) (*payloadSource, error) {
	if size < PayloadHeaderSize {
		return nil, fmt.Errorf("payload too small (%d bytes)", size)
	}
	var hdr [PayloadHeaderSize]byte
	if _, err := r.ReadAt(hdr[:], 0); err != nil {
		return nil, fmt.Errorf("read payload header: %w", err)
	}
	if string(hdr[:4]) != PayloadMagic {
		return nil, fmt.Errorf("invalid payload magic (expected %q)", PayloadMagic)
	}

	formatVer := binary.BigEndian.Uint64(hdr[4:12])
	if formatVer != PayloadFormatVersion {
		return nil, fmt.Errorf("unsupported payload format version %d (expected %d)", formatVer, PayloadFormatVersion)
	}
	manifestSize := binary.BigEndian.Uint64(hdr[12:20])
	metaSigSize := binary.BigEndian.Uint32(hdr[20:24])
	if manifestSize == 0 || manifestSize > maxManifestBytes {
		return nil, fmt.Errorf("invalid payload manifest size: %d bytes", manifestSize)
	}
	availAfterHdr := uint64(size - PayloadHeaderSize)
	if manifestSize > availAfterHdr || uint64(metaSigSize) > availAfterHdr-manifestSize {
		return nil, fmt.Errorf("payload manifest/signature exceeds file size (%d)", size)
	}
	dataStart := int64(PayloadHeaderSize) + int64(manifestSize) + int64(metaSigSize)

	manifestBuf := make([]byte, manifestSize)
	if _, err := io.ReadFull(io.NewSectionReader(r, PayloadHeaderSize, int64(manifestSize)), manifestBuf); err != nil {
		return nil, fmt.Errorf("read payload manifest: %w", err)
	}
	manifest, err := decodeManifest(manifestBuf, summaryOnly)
	if err != nil {
		return nil, fmt.Errorf("decode payload manifest: %w", err)
	}

	return &payloadSource{
		reader:    r,
		size:      size,
		dataStart: dataStart,
		manifest:  manifest,
		info:      buildPayloadInfo(filePath, sourceType, formatVer, manifest, otaMeta),
	}, nil
}

func findPayloadZipEntry(zr *zip.Reader) (*zip.File, error) {
	var fallback *zip.File
	var hasOdin bool

	for _, f := range zr.File {
		if f.Mode()&os.ModeSymlink != 0 || f.FileInfo().IsDir() {
			continue
		}
		name := stripLeadingDotSlash(f.Name)
		lower := strings.ToLower(name)
		if lower == "payload.bin" {
			return f, nil
		}
		if fallback == nil && path.Base(lower) == "payload.bin" {
			fallback = f
		}
		hasOdin = hasOdin || strings.HasSuffix(lower, ".tar.md5")
	}
	if fallback != nil {
		return fallback, nil
	}
	if hasOdin {
		return nil, fmt.Errorf("archive contains Samsung Odin .tar.md5 firmware, not an A/B payload.bin")
	}
	return nil, fmt.Errorf("payload.bin not found inside zip archive")
}
