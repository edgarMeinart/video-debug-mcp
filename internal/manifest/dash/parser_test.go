package dash

import (
	"os"
	"testing"
	"github.com/meinart/video-debug-mcp/internal/manifest"
)

func TestParseMPD_Simple(t *testing.T) {
	data, _ := os.ReadFile("testdata/simple.mpd")
	m, err := Parse(data, "https://cdn.example.com/dash/manifest.mpd")
	if err != nil { t.Fatalf("Parse() error: %v", err) }
	if m.Type != manifest.ManifestTypeDASH { t.Errorf("expected type=dash, got %v", m.Type) }
	if len(m.Variants) != 2 { t.Fatalf("expected 2 variants, got %d", len(m.Variants)) }
	v := m.Variants[0]
	if v.Bandwidth != 3000000 { t.Errorf("expected bandwidth=3000000, got %d", v.Bandwidth) }
	if v.Resolution != "1280x720" { t.Errorf("expected resolution=1280x720, got %s", v.Resolution) }
	if len(m.InitSegments) < 2 { t.Errorf("expected at least 2 init segments, got %d", len(m.InitSegments)) }
	if len(m.AudioTracks) != 1 { t.Fatalf("expected 1 audio track, got %d", len(m.AudioTracks)) }
	if m.AudioTracks[0].Language != "en" { t.Errorf("expected language=en, got %s", m.AudioTracks[0].Language) }
	if len(m.SubtitleTracks) != 1 { t.Fatalf("expected 1 subtitle track, got %d", len(m.SubtitleTracks)) }
}

func TestParseMPD_SegmentURLResolution(t *testing.T) {
	data, _ := os.ReadFile("testdata/simple.mpd")
	m, err := Parse(data, "https://cdn.example.com/dash/manifest.mpd")
	if err != nil { t.Fatalf("Parse() error: %v", err) }
	foundInit := false
	for _, s := range m.InitSegments {
		if s.URI == "https://cdn.example.com/dash/video_720p_init.mp4" { foundInit = true; break }
	}
	if !foundInit { t.Error("expected resolved init segment URL for 720p") }
	if len(m.Variants[0].Segments) == 0 { t.Fatal("expected segments in first variant") }
	seg := m.Variants[0].Segments[0]
	if seg.URI != "https://cdn.example.com/dash/video_720p_1.m4s" { t.Errorf("expected resolved segment URL, got %s", seg.URI) }
}

func TestParseMPD_DRM(t *testing.T) {
	data, _ := os.ReadFile("testdata/drm.mpd")
	m, err := Parse(data, "https://cdn.example.com/dash/drm.mpd")
	if err != nil { t.Fatalf("Parse() error: %v", err) }
	if m.DRM == nil { t.Fatal("expected DRM info, got nil") }
	if m.DRM.SchemeURI == "" { t.Error("expected scheme URI, got empty") }
	if m.DRM.KeyID == "" { t.Error("expected key ID, got empty") }
}

func TestParse_InvalidXML(t *testing.T) {
	_, err := Parse([]byte("not valid xml"), "https://example.com/bad.mpd")
	if err == nil { t.Error("expected error for invalid MPD, got nil") }
}
