package fetcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspace_CreateAndCleanup(t *testing.T) {
	ws, err := NewWorkspace()
	if err != nil { t.Fatalf("NewWorkspace() error: %v", err) }
	info, err := os.Stat(ws.Dir)
	if err != nil { t.Fatalf("workspace dir does not exist: %v", err) }
	if !info.IsDir() { t.Error("workspace path is not a directory") }
	dir := ws.Dir
	ws.Cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) { t.Error("workspace dir still exists after cleanup") }
}

func TestWorkspace_AssetPath(t *testing.T) {
	ws, err := NewWorkspace()
	if err != nil { t.Fatalf("NewWorkspace() error: %v", err) }
	defer ws.Cleanup()
	path := ws.AssetPath("segment_001.ts")
	expected := filepath.Join(ws.Dir, "segment_001.ts")
	if path != expected { t.Errorf("expected %s, got %s", expected, path) }
}

func TestWorkspace_RecordAsset(t *testing.T) {
	ws, err := NewWorkspace()
	if err != nil { t.Fatalf("NewWorkspace() error: %v", err) }
	defer ws.Cleanup()
	ws.RecordAsset("https://example.com/seg.ts", "seg.ts", 1024, "video/mp2t")
	if len(ws.Assets) != 1 { t.Fatalf("expected 1 asset, got %d", len(ws.Assets)) }
	if ws.Assets[0].URL != "https://example.com/seg.ts" { t.Errorf("expected URL, got %s", ws.Assets[0].URL) }
	if ws.Assets[0].Size != 1024 { t.Errorf("expected size 1024, got %d", ws.Assets[0].Size) }
}
