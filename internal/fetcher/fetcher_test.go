package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestFetcher_DownloadFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Write([]byte("fake mp4 data"))
	}))
	defer server.Close()
	f := NewFetcher(10, 30*time.Second, "test/1.0")
	ws, err := NewWorkspace()
	if err != nil { t.Fatalf("NewWorkspace() error: %v", err) }
	defer ws.Cleanup()
	ctx := context.Background()
	err = f.DownloadFile(ctx, server.URL+"/video.mp4", ws, "video.mp4", nil)
	if err != nil { t.Fatalf("DownloadFile() error: %v", err) }
	data, err := os.ReadFile(ws.AssetPath("video.mp4"))
	if err != nil { t.Fatalf("failed to read: %v", err) }
	if string(data) != "fake mp4 data" { t.Errorf("expected 'fake mp4 data', got %q", string(data)) }
	if len(ws.Assets) != 1 { t.Fatalf("expected 1 asset, got %d", len(ws.Assets)) }
}

func TestFetcher_DownloadFile_CustomHeaders(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Write([]byte("data"))
	}))
	defer server.Close()
	f := NewFetcher(10, 30*time.Second, "test/1.0")
	ws, _ := NewWorkspace()
	defer ws.Cleanup()
	headers := map[string]string{"Authorization": "Bearer token123"}
	ctx := context.Background()
	err := f.DownloadFile(ctx, server.URL+"/file", ws, "file", headers)
	if err != nil { t.Fatalf("DownloadFile() error: %v", err) }
	if receivedAuth != "Bearer token123" { t.Errorf("expected auth header, got %q", receivedAuth) }
}

func TestFetcher_DownloadFile_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()
	f := NewFetcher(10, 30*time.Second, "test/1.0")
	ws, _ := NewWorkspace()
	defer ws.Cleanup()
	err := f.DownloadFile(context.Background(), server.URL+"/missing.mp4", ws, "missing.mp4", nil)
	if err == nil { t.Error("expected error for 404, got nil") }
}

func TestFetcher_DownloadConcurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("segment data"))
	}))
	defer server.Close()
	f := NewFetcher(5, 30*time.Second, "test/1.0")
	ws, _ := NewWorkspace()
	defer ws.Cleanup()
	urls := map[string]string{
		server.URL + "/seg1.ts": "seg1.ts",
		server.URL + "/seg2.ts": "seg2.ts",
		server.URL + "/seg3.ts": "seg3.ts",
	}
	errors := f.DownloadConcurrent(context.Background(), urls, ws, nil)
	if len(errors) != 0 { t.Errorf("expected no errors, got %v", errors) }
	if len(ws.Assets) != 3 { t.Errorf("expected 3 assets, got %d", len(ws.Assets)) }
}

func TestFetcher_DetectType(t *testing.T) {
	tests := []struct {
		url     string
		content string
		want    AssetType
	}{
		{"https://example.com/master.m3u8", "#EXTM3U", AssetTypeHLS},
		{"https://example.com/manifest.mpd", "<?xml", AssetTypeDASH},
		{"https://example.com/video.mp4", "\x00\x00", AssetTypeMedia},
		{"https://example.com/unknown", "#EXTM3U", AssetTypeHLS},
		{"https://example.com/unknown", "<?xml version", AssetTypeDASH},
	}
	for _, tt := range tests {
		got := DetectAssetType(tt.url, []byte(tt.content))
		if got != tt.want { t.Errorf("DetectAssetType(%q, ...) = %v, want %v", tt.url, got, tt.want) }
	}
}
