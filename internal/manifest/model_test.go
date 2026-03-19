package manifest

import "testing"

func TestManifest_AllSegmentURLs(t *testing.T) {
	m := &Manifest{
		Type: ManifestTypeHLS,
		Variants: []Variant{
			{URI: "720p.m3u8", Bandwidth: 3000000, Segments: []Segment{
				{URI: "https://cdn.example.com/seg1.ts", Duration: 6.0, Sequence: 0},
				{URI: "https://cdn.example.com/seg2.ts", Duration: 6.0, Sequence: 1},
			}},
		},
		InitSegments: []Segment{{URI: "https://cdn.example.com/init.mp4", IsInit: true}},
	}
	urls := m.AllSegmentURLs()
	if len(urls) != 3 { t.Errorf("expected 3 URLs, got %d", len(urls)) }
}

func TestManifest_SampledSegmentURLs(t *testing.T) {
	segs := make([]Segment, 20)
	for i := range segs {
		segs[i] = Segment{URI: "https://cdn.example.com/seg" + string(rune('A'+i)) + ".ts", Duration: 6.0, Sequence: i}
	}
	m := &Manifest{
		Type: ManifestTypeHLS,
		Variants: []Variant{{Segments: segs}},
		InitSegments: []Segment{{URI: "https://cdn.example.com/init.mp4", IsInit: true}},
	}
	urls := m.SampledSegmentURLs(3)
	if len(urls) != 7 { t.Errorf("expected 7 sampled URLs, got %d", len(urls)) }
}

func TestManifest_SampledSegmentURLs_FewSegments(t *testing.T) {
	m := &Manifest{
		Type: ManifestTypeHLS,
		Variants: []Variant{{Segments: []Segment{
			{URI: "https://cdn.example.com/seg1.ts"},
			{URI: "https://cdn.example.com/seg2.ts"},
		}}},
	}
	urls := m.SampledSegmentURLs(3)
	if len(urls) != 2 { t.Errorf("expected 2 URLs (all segments), got %d", len(urls)) }
}

func TestResolveURI(t *testing.T) {
	tests := []struct{ base, ref, expected string }{
		{"https://cdn.example.com/hls/master.m3u8", "720p.m3u8", "https://cdn.example.com/hls/720p.m3u8"},
		{"https://cdn.example.com/hls/master.m3u8", "/absolute/720p.m3u8", "https://cdn.example.com/absolute/720p.m3u8"},
		{"https://cdn.example.com/hls/master.m3u8", "https://other.com/720p.m3u8", "https://other.com/720p.m3u8"},
	}
	for _, tt := range tests {
		got, err := ResolveURI(tt.base, tt.ref)
		if err != nil { t.Errorf("ResolveURI(%q, %q) error: %v", tt.base, tt.ref, err); continue }
		if got != tt.expected { t.Errorf("ResolveURI(%q, %q) = %q, want %q", tt.base, tt.ref, got, tt.expected) }
	}
}
