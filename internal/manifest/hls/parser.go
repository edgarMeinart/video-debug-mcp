package hls

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/grafov/m3u8"
	"github.com/meinart/video-debug-mcp/internal/manifest"
)

// Parse decodes HLS playlist data (master or media) and returns a Manifest.
// baseURL is used to resolve relative URIs.
func Parse(data []byte, baseURL string) (*manifest.Manifest, error) {
	if !bytes.Contains(data, []byte("#EXTM3U")) {
		return nil, fmt.Errorf("hls: not a valid HLS playlist (missing #EXTM3U)")
	}

	buf := bytes.NewBuffer(data)
	playlist, listType, err := m3u8.Decode(*buf, false)
	if err != nil {
		return nil, fmt.Errorf("hls: decode error: %w", err)
	}

	m := &manifest.Manifest{
		Type: manifest.ManifestTypeHLS,
		URL:  baseURL,
		Raw:  string(data),
	}

	switch listType {
	case m3u8.MASTER:
		return parseMaster(m, playlist.(*m3u8.MasterPlaylist), data, baseURL)
	case m3u8.MEDIA:
		return parseMedia(m, playlist.(*m3u8.MediaPlaylist), baseURL)
	default:
		return nil, fmt.Errorf("hls: unknown playlist type: %v", listType)
	}
}

func parseMaster(m *manifest.Manifest, pl *m3u8.MasterPlaylist, raw []byte, baseURL string) (*manifest.Manifest, error) {
	for _, v := range pl.Variants {
		if v == nil {
			continue
		}
		resolvedURI, err := manifest.ResolveURI(baseURL, v.URI)
		if err != nil {
			return nil, fmt.Errorf("hls: resolve variant URI %q: %w", v.URI, err)
		}
		m.Variants = append(m.Variants, manifest.Variant{
			URI:        resolvedURI,
			Bandwidth:  int(v.Bandwidth),
			Resolution: v.Resolution,
			Codecs:     v.Codecs,
			FrameRate:  v.FrameRate,
		})
	}

	// Parse EXT-X-MEDIA manually since grafov/m3u8 doesn't expose it
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#EXT-X-MEDIA:") {
			continue
		}
		attrs := parseAttributeList(strings.TrimPrefix(line, "#EXT-X-MEDIA:"))
		altType := attrs["TYPE"]
		altURI := attrs["URI"]
		altLang := attrs["LANGUAGE"]
		altName := attrs["NAME"]

		resolvedAltURI := ""
		if altURI != "" {
			var err error
			resolvedAltURI, err = manifest.ResolveURI(baseURL, altURI)
			if err != nil {
				return nil, fmt.Errorf("hls: resolve alt URI %q: %w", altURI, err)
			}
		}

		switch strings.ToUpper(altType) {
		case "AUDIO":
			m.AudioTracks = append(m.AudioTracks, manifest.AudioTrack{
				URI:      resolvedAltURI,
				Language: altLang,
				Name:     altName,
			})
		case "SUBTITLES":
			m.SubtitleTracks = append(m.SubtitleTracks, manifest.SubTrack{
				URI:      resolvedAltURI,
				Language: altLang,
				Name:     altName,
			})
		}
	}

	return m, nil
}

// parseAttributeList parses an HLS attribute list string into a map.
// e.g. TYPE=AUDIO,GROUP-ID="audio",NAME="English",LANGUAGE="en",URI="audio/en.m3u8"
func parseAttributeList(s string) map[string]string {
	attrs := make(map[string]string)
	for len(s) > 0 {
		// Find key
		eqIdx := strings.Index(s, "=")
		if eqIdx < 0 {
			break
		}
		key := strings.TrimSpace(s[:eqIdx])
		s = s[eqIdx+1:]

		var val string
		if strings.HasPrefix(s, `"`) {
			// Quoted string
			end := strings.Index(s[1:], `"`)
			if end < 0 {
				val = s[1:]
				s = ""
			} else {
				val = s[1 : end+1]
				s = s[end+2:] // skip closing quote
				if strings.HasPrefix(s, ",") {
					s = s[1:]
				}
			}
		} else {
			// Unquoted value
			commaIdx := strings.Index(s, ",")
			if commaIdx < 0 {
				val = s
				s = ""
			} else {
				val = s[:commaIdx]
				s = s[commaIdx+1:]
			}
		}
		attrs[key] = val
	}
	return attrs
}

func parseMedia(m *manifest.Manifest, pl *m3u8.MediaPlaylist, baseURL string) (*manifest.Manifest, error) {
	// Build a single variant to hold all segments (media playlist = one quality level)
	v := manifest.Variant{}

	// Track EXT-X-MAP init segment (only add once)
	var initURI string

	// Track DRM key
	var extXKey string

	for i, seg := range pl.Segments {
		if seg == nil {
			continue
		}

		// Handle init segment (EXT-X-MAP)
		if seg.Map != nil && seg.Map.URI != "" && initURI == "" {
			resolved, err := manifest.ResolveURI(baseURL, seg.Map.URI)
			if err != nil {
				return nil, fmt.Errorf("hls: resolve map URI %q: %w", seg.Map.URI, err)
			}
			initURI = resolved
			m.InitSegments = append(m.InitSegments, manifest.Segment{
				URI:    resolved,
				IsInit: true,
			})
		}

		// Handle EXT-X-KEY per segment
		if seg.Key != nil && seg.Key.URI != "" && extXKey == "" {
			extXKey = buildExtXKeyString(seg.Key)
		}

		resolvedURI, err := manifest.ResolveURI(baseURL, seg.URI)
		if err != nil {
			return nil, fmt.Errorf("hls: resolve segment URI %q: %w", seg.URI, err)
		}
		v.Segments = append(v.Segments, manifest.Segment{
			URI:      resolvedURI,
			Duration: seg.Duration,
			Sequence: i,
		})
	}

	// Also check playlist-level key if no per-segment key was found
	if extXKey == "" && pl.Key != nil && pl.Key.URI != "" {
		extXKey = buildExtXKeyString(pl.Key)
	}

	if extXKey != "" {
		m.DRM = &manifest.DRMInfo{
			EXTXKey: extXKey,
		}
	}

	m.Variants = append(m.Variants, v)
	return m, nil
}

func buildExtXKeyString(key *m3u8.Key) string {
	if key == nil {
		return ""
	}
	s := fmt.Sprintf("METHOD=%s", key.Method)
	if key.URI != "" {
		s += fmt.Sprintf(",URI=%q", key.URI)
	}
	if key.IV != "" {
		s += fmt.Sprintf(",IV=%s", key.IV)
	}
	return s
}

