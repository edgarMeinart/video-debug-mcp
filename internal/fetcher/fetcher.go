package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

// AssetType represents the type of a downloaded asset.
type AssetType string

const (
	AssetTypeHLS   AssetType = "hls"
	AssetTypeDASH  AssetType = "dash"
	AssetTypeMedia AssetType = "media"
)

// Fetcher downloads assets from URLs with concurrency control.
type Fetcher struct {
	client    *http.Client
	maxConc   int
	userAgent string
}

// NewFetcher creates a new Fetcher with the given concurrency limit, request timeout, and user agent.
func NewFetcher(maxConcurrent int, requestTimeout time.Duration, userAgent string) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: requestTimeout,
		},
		maxConc:   maxConcurrent,
		userAgent: userAgent,
	}
}

// DownloadFile downloads a single file from url and saves it to the workspace under filename.
// Optional headers are added to the request. On success the asset is recorded in the workspace.
func (f *Fetcher) DownloadFile(ctx context.Context, url string, ws *Workspace, filename string, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request for %s: %w", url, err)
	}
	if f.userAgent != "" {
		req.Header.Set("User-Agent", f.userAgent)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}

	destPath := ws.AssetPath(filename)
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", destPath, err)
	}
	defer file.Close()

	written, err := io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("writing file %s: %w", destPath, err)
	}

	ws.RecordAsset(url, filename, written, resp.Header.Get("Content-Type"))
	return nil
}

// DownloadError holds a URL and the error that occurred when downloading it.
type DownloadError struct {
	URL string
	Err error
}

func (e *DownloadError) Error() string {
	return fmt.Sprintf("download %s: %v", e.URL, e.Err)
}

// DownloadConcurrent downloads multiple files concurrently, respecting the fetcher's max concurrency.
// urls maps source URL to destination filename. Returns a slice of any errors that occurred.
func (f *Fetcher) DownloadConcurrent(ctx context.Context, urls map[string]string, ws *Workspace, headers map[string]string) []error {
	concurrency := min(f.maxConc, len(urls))
	sem := make(chan struct{}, concurrency)

	var (
		mu     sync.Mutex
		errs   []error
		wg     sync.WaitGroup
	)

	for url, filename := range urls {
		wg.Add(1)
		go func(u, fn string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := f.DownloadFile(ctx, u, ws, fn, headers); err != nil {
				mu.Lock()
				errs = append(errs, &DownloadError{URL: u, Err: err})
				mu.Unlock()
			}
		}(url, filename)
	}

	wg.Wait()
	return errs
}

// DetectAssetType determines the AssetType for a given URL and content snippet.
// It first checks the URL extension, then falls back to content inspection.
func DetectAssetType(url string, content []byte) AssetType {
	ext := strings.ToLower(path.Ext(path.Base(url)))
	switch ext {
	case ".m3u8":
		return AssetTypeHLS
	case ".mpd":
		return AssetTypeDASH
	case ".mp4", ".ts", ".webm", ".mkv", ".avi", ".mov":
		return AssetTypeMedia
	}

	// Fall back to content inspection.
	snippet := string(content)
	if strings.HasPrefix(snippet, "#EXTM3U") {
		return AssetTypeHLS
	}
	if strings.HasPrefix(snippet, "<?xml") {
		return AssetTypeDASH
	}
	return AssetTypeMedia
}
