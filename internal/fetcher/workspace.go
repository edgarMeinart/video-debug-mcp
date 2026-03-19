package fetcher

import (
	"os"
	"path/filepath"

	"github.com/meinart/video-debug-mcp/internal/output"
)

type Workspace struct {
	Dir          string
	ManifestPath string
	Assets       []output.DownloadedAsset
}

func NewWorkspace() (*Workspace, error) {
	dir, err := os.MkdirTemp("", "video-debug-*")
	if err != nil {
		return nil, err
	}
	return &Workspace{Dir: dir}, nil
}

func (w *Workspace) AssetPath(filename string) string {
	return filepath.Join(w.Dir, filename)
}

func (w *Workspace) RecordAsset(url, filename string, size int64, mimeType string) {
	w.Assets = append(w.Assets, output.DownloadedAsset{
		URL:      url,
		Path:     filename,
		Size:     size,
		MimeType: mimeType,
	})
}

func (w *Workspace) Cleanup() {
	os.RemoveAll(w.Dir)
}
