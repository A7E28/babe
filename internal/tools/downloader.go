package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// receives the current bytes downloaded and the total content length.
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

// downloads a URL to destPath with real-time progress reporting and context cancellation.
func DownloadFile(ctx context.Context, url, destPath string, onProgress ProgressFunc) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Minute, // Large file buffer timeout
	}

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
	defer out.Close()

	pw := &progressWriter{
		total:      resp.ContentLength,
		onProgress: onProgress,
	}

	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(out, io.TeeReader(resp.Body, pw), buf); err != nil {
		return fmt.Errorf("write stream to %s: %w", destPath, err)
	}

	return nil
}
