package hash

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
)

// represents supported cryptographic hash algorithms.
type Algorithm string

const (
	MD5    Algorithm = "MD5"
	SHA1   Algorithm = "SHA1"
	SHA256 Algorithm = "SHA256"
)

// holds MD5, SHA1, and SHA256 hexadecimal digests computed in a single pass.
type MultiDigest struct {
	MD5    string
	SHA1   string
	SHA256 string
}

// computes MD5, SHA1, and SHA256 digests of a file in a single streaming read.
func CalculateAllFileHashes(filePath string) (MultiDigest, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return MultiDigest{}, fmt.Errorf("open file for hashing: %w", err)
	}
	defer file.Close()

	return CalculateAllReaderHashes(file)
}

// computes MD5, SHA1, and SHA256 digests from an io.Reader in one pass.
func CalculateAllReaderHashes(r io.Reader) (MultiDigest, error) {
	hMD5 := md5.New()
	hSHA1 := sha1.New()
	hSHA256 := sha256.New()

	multi := io.MultiWriter(hMD5, hSHA1, hSHA256)
	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(multi, r, buf); err != nil {
		return MultiDigest{}, fmt.Errorf("stream multi-hash calculation: %w", err)
	}

	return MultiDigest{
		MD5:    hex.EncodeToString(hMD5.Sum(nil)),
		SHA1:   hex.EncodeToString(hSHA1.Sum(nil)),
		SHA256: hex.EncodeToString(hSHA256.Sum(nil)),
	}, nil
}

// computes the hexadecimal hash digest of a file using streaming I/O.
func CalculateFileHash(filePath string, algo Algorithm) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file for hashing: %w", err)
	}
	defer file.Close()

	return CalculateReaderHash(file, algo)
}

// computes the hash digest from an arbitrary io.Reader stream.
func CalculateReaderHash(r io.Reader, algo Algorithm) (string, error) {
	var h hash.Hash

	switch algo {
	case MD5:
		h = md5.New()
	case SHA1:
		h = sha1.New()
	case SHA256:
		h = sha256.New()
	default:
		return "", fmt.Errorf("unsupported hash algorithm: %s", algo)
	}

	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(h, r, buf); err != nil {
		return "", fmt.Errorf("stream hash calculation: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
