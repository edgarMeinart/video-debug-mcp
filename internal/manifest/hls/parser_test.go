package hls

import (
	"os"
	"testing"
	"github.com/meinart/video-debug-mcp/internal/manifest"
)

func TestParseMasterPlaylist(t *testing.T) {
	data, _ := os.ReadFile("testdata/master.m3u8")
	m, err := Parse(data, "https://cdn.example.com/hls/master.m3u8")
	if err != nil { t.Fatalf("Parse() error: %v", err) }
	if m.Type != manifest.ManifestTypeHLS { t.Errorf("expected type=hls, got %v", m.Type) }
	if len(m.Variants) != 2 { t.Fatalf("expected 2 variants, got %d", len(m.Variants)) }
	v := m.Variants[0]
	if v.Bandwidth != 3000000 { t.Errorf("expected bandwidth=3000000, got %d", v.Bandwidth) }
	if v.Resolution != "1280x720" { t.Errorf("expected resolution=1280x720, got %s", v.Resolution) }
	if v.URI != "https://cdn.example.com/hls/720p/playlist.m3u8" { t.Errorf("expected resolved URI, got %s", v.URI) }
	if len(m.AudioTracks) != 1 { t.Fatalf("expected 1 audio track, got %d", len(m.AudioTracks)) }
	if m.AudioTracks[0].Language != "en" { t.Errorf("expected language=en, got %s", m.AudioTracks[0].Language) }
	if len(m.SubtitleTracks) != 1 { t.Fatalf("expected 1 subtitle track, got %d", len(m.SubtitleTracks)) }
}

func TestParseMediaPlaylist(t *testing.T) {
	data, _ := os.ReadFile("testdata/media.m3u8")
	m, err := Parse(data, "https://cdn.example.com/hls/720p/playlist.m3u8")
	if err != nil { t.Fatalf("Parse() error: %v", err) }
	if len(m.Variants) != 1 { t.Fatalf("expected 1 variant, got %d", len(m.Variants)) }
	v := m.Variants[0]
	if len(v.Segments) != 3 { t.Fatalf("expected 3 segments, got %d", len(v.Segments)) }
	if len(m.InitSegments) != 1 { t.Fatalf("expected 1 init segment, got %d", len(m.InitSegments)) }
	if m.InitSegments[0].URI != "https://cdn.example.com/hls/720p/init.mp4" { t.Errorf("expected resolved init URI, got %s", m.InitSegments[0].URI) }
	if v.Segments[0].URI != "https://cdn.example.com/hls/720p/segment_000.ts" { t.Errorf("expected resolved URI, got %s", v.Segments[0].URI) }
	if v.Segments[2].Duration != 4.5 { t.Errorf("expected duration=4.5, got %f", v.Segments[2].Duration) }
}

func TestParseEncryptedPlaylist(t *testing.T) {
	data, _ := os.ReadFile("testdata/encrypted.m3u8")
	m, err := Parse(data, "https://cdn.example.com/hls/encrypted.m3u8")
	if err != nil { t.Fatalf("Parse() error: %v", err) }
	if m.DRM == nil { t.Fatal("expected DRM info, got nil") }
	if m.DRM.EXTXKey == "" { t.Error("expected EXT-X-KEY value, got empty") }
}

func TestParse_InvalidData(t *testing.T) {
	_, err := Parse([]byte("not a valid playlist"), "https://example.com/bad.m3u8")
	if err == nil { t.Error("expected error for invalid playlist, got nil") }
}
