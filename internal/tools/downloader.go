package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ProgressFunc receives the current bytes downloaded and the total content length.
type ProgressFunc func(current, total int64)

type progressWriter struct {
	total      int64
	current    int64
	onProgress ProgressFunc
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.current += int64(n)
	if pw.onProgress != nil {
		pw.onProgress(pw.current, pw.total)
	}
	return n, nil
}

// DownloadFile downloads a URL to destPath
func DownloadFile(ctx context.Context, url, destPath string, onProgress ProgressFunc) error {
	client := &http.Client{
		Timeout: 30 * time.Minute,
	}
	return DownloadFileWithClient(ctx, client, url, destPath, onProgress)
}

// DownloadFileWithClient downloads a URL to destPath using the supplied HTTP client.
func DownloadFileWithClient(ctx context.Context, client *http.Client, rawURL, destPath string, onProgress ProgressFunc) error {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Minute}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("User-Agent", "babe-cli")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("execute download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create destination file %s: %w", destPath, err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = out.Close()
		}
	}()

	pw := &progressWriter{
		total:      resp.ContentLength,
		onProgress: onProgress,
	}

	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(out, io.TeeReader(resp.Body, pw), buf); err != nil {
		return fmt.Errorf("write stream to %s: %w", destPath, err)
	}
	if err := out.Sync(); err != nil {
		return fmt.Errorf("sync destination file %s: %w", destPath, err)
	}
	if err := out.Close(); err != nil {
		closed = true
		return fmt.Errorf("close destination file %s: %w", destPath, err)
	}
	closed = true

	return nil
}
