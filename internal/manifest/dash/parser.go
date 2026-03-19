package dash

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/meinart/video-debug-mcp/internal/manifest"
)

// XML struct definitions

type mpdXML struct {
	XMLName                    xml.Name   `xml:"MPD"`
	Type                       string     `xml:"type,attr"`
	MediaPresentationDuration  string     `xml:"mediaPresentationDuration,attr"`
	Periods                    []periodXML `xml:"Period"`
}

type periodXML struct {
	AdaptationSets []adaptationSetXML `xml:"AdaptationSet"`
}

type adaptationSetXML struct {
	MimeType           string               `xml:"mimeType,attr"`
	ContentType        string               `xml:"contentType,attr"`
	Lang               string               `xml:"lang,attr"`
	ContentProtections []contentProtectionXML `xml:"ContentProtection"`
	Representations    []representationXML  `xml:"Representation"`
	SegmentTemplate    *segmentTemplateXML  `xml:"SegmentTemplate"`
}

// contentProtectionXML uses custom unmarshaling to capture namespaced attributes
type contentProtectionXML struct {
	SchemeIdUri string `xml:"schemeIdUri,attr"`
	Value       string `xml:"value,attr"`
	DefaultKID  string // populated by UnmarshalXML from cenc:default_KID
	PSSH        string `xml:"pssh"`
	RawAttrs    []xml.Attr
}

func (cp *contentProtectionXML) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "schemeIdUri":
			cp.SchemeIdUri = attr.Value
		case "value":
			cp.Value = attr.Value
		case "default_KID":
			cp.DefaultKID = attr.Value
		}
	}
	// decode children
	type inner struct {
		PSSH string `xml:"pssh"`
	}
	var in inner
	if err := d.DecodeElement(&in, &start); err != nil {
		return err
	}
	cp.PSSH = in.PSSH
	return nil
}

type representationXML struct {
	ID              string              `xml:"id,attr"`
	Bandwidth       int                 `xml:"bandwidth,attr"`
	Width           int                 `xml:"width,attr"`
	Height          int                 `xml:"height,attr"`
	Codecs          string              `xml:"codecs,attr"`
	SegmentTemplate *segmentTemplateXML `xml:"SegmentTemplate"`
	BaseURL         string              `xml:"BaseURL"`
}

type segmentTemplateXML struct {
	Media          string `xml:"media,attr"`
	Initialization string `xml:"initialization,attr"`
	StartNumber    int    `xml:"startNumber,attr"`
	Duration       int    `xml:"duration,attr"`
	Timescale      int    `xml:"timescale,attr"`
}

// parseISO8601Duration parses a subset of ISO 8601 durations (PTxHxMxS).
// Returns total seconds as float64.
func parseISO8601Duration(s string) (float64, error) {
	// Supported: PT60S, PT1M30S, PT1H2M3S, PT1.5S etc.
	re := regexp.MustCompile(`^PT(?:(\d+(?:\.\d+)?)H)?(?:(\d+(?:\.\d+)?)M)?(?:(\d+(?:\.\d+)?)S)?$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(s))
	if matches == nil {
		return 0, fmt.Errorf("unsupported ISO 8601 duration: %q", s)
	}
	var total float64
	if matches[1] != "" {
		h, _ := strconv.ParseFloat(matches[1], 64)
		total += h * 3600
	}
	if matches[2] != "" {
		m, _ := strconv.ParseFloat(matches[2], 64)
		total += m * 60
	}
	if matches[3] != "" {
		sec, _ := strconv.ParseFloat(matches[3], 64)
		total += sec
	}
	return total, nil
}

// resolveTemplate replaces $Number$ and $RepresentationID$ in a segment template URI.
func resolveTemplate(tmpl, repID string, number int) string {
	s := strings.ReplaceAll(tmpl, "$Number$", strconv.Itoa(number))
	s = strings.ReplaceAll(s, "$RepresentationID$", repID)
	return s
}

// segmentDurationSec returns the duration in seconds for a SegmentTemplate.
func segmentDurationSec(st *segmentTemplateXML) float64 {
	ts := st.Timescale
	if ts == 0 {
		ts = 1
	}
	return float64(st.Duration) / float64(ts)
}

// Parse parses a DASH MPD document and returns a manifest.Manifest.
func Parse(data []byte, baseURL string) (*manifest.Manifest, error) {
	var mpd mpdXML
	if err := xml.Unmarshal(data, &mpd); err != nil {
		return nil, fmt.Errorf("dash: xml unmarshal: %w", err)
	}

	// Validate it looks like an MPD (xml.Unmarshal doesn't fail on arbitrary XML)
	if mpd.XMLName.Local != "MPD" {
		return nil, fmt.Errorf("dash: not a valid MPD document")
	}

	m := &manifest.Manifest{
		Type: manifest.ManifestTypeDASH,
		URL:  baseURL,
		Raw:  string(data),
	}

	// Parse total duration
	var totalDuration float64
	if mpd.MediaPresentationDuration != "" {
		d, err := parseISO8601Duration(mpd.MediaPresentationDuration)
		if err == nil {
			totalDuration = d
		}
	}

	resolve := func(ref string) string {
		resolved, err := manifest.ResolveURI(baseURL, ref)
		if err != nil {
			return ref
		}
		return resolved
	}

	for _, period := range mpd.Periods {
		for _, as := range period.AdaptationSets {
			contentType := as.ContentType
			if contentType == "" {
				// Infer from mimeType
				mt := strings.ToLower(as.MimeType)
				switch {
				case strings.HasPrefix(mt, "video"):
					contentType = "video"
				case strings.HasPrefix(mt, "audio"):
					contentType = "audio"
				case strings.HasPrefix(mt, "text"):
					contentType = "text"
				}
			}

			// Extract DRM from ContentProtection elements
			if m.DRM == nil && len(as.ContentProtections) > 0 {
				drm := extractDRM(as.ContentProtections)
				if drm != nil {
					m.DRM = drm
				}
			}

			switch contentType {
			case "video":
				for _, rep := range as.Representations {
					st := effectiveSegmentTemplate(as.SegmentTemplate, rep.SegmentTemplate)
					var segs []manifest.Segment
					var initURI string
					if st != nil {
						initURI = resolve(resolveTemplate(st.Initialization, rep.ID, 0))
						segDur := segmentDurationSec(st)
						var count int
						if totalDuration > 0 && segDur > 0 {
							count = int(totalDuration / segDur)
						}
						for i := 0; i < count; i++ {
							num := st.StartNumber + i
							uri := resolve(resolveTemplate(st.Media, rep.ID, num))
							segs = append(segs, manifest.Segment{
								URI:      uri,
								Duration: segDur,
								Sequence: num,
							})
						}
					}
					res := ""
					if rep.Width > 0 && rep.Height > 0 {
						res = fmt.Sprintf("%dx%d", rep.Width, rep.Height)
					}
					v := manifest.Variant{
						Bandwidth:  rep.Bandwidth,
						Resolution: res,
						Codecs:     rep.Codecs,
						Segments:   segs,
					}
					if st != nil && initURI != "" {
						m.InitSegments = append(m.InitSegments, manifest.Segment{
							URI:    initURI,
							IsInit: true,
						})
					}
					m.Variants = append(m.Variants, v)
				}

			case "audio":
				for _, rep := range as.Representations {
					st := effectiveSegmentTemplate(as.SegmentTemplate, rep.SegmentTemplate)
					var segs []manifest.Segment
					if st != nil {
						initURI := resolve(resolveTemplate(st.Initialization, rep.ID, 0))
						m.InitSegments = append(m.InitSegments, manifest.Segment{
							URI:    initURI,
							IsInit: true,
						})
						segDur := segmentDurationSec(st)
						var count int
						if totalDuration > 0 && segDur > 0 {
							count = int(totalDuration / segDur)
						}
						for i := 0; i < count; i++ {
							num := st.StartNumber + i
							uri := resolve(resolveTemplate(st.Media, rep.ID, num))
							segs = append(segs, manifest.Segment{
								URI:      uri,
								Duration: segDur,
								Sequence: num,
							})
						}
					} else if rep.BaseURL != "" {
						segs = append(segs, manifest.Segment{URI: resolve(rep.BaseURL)})
					}
					at := manifest.AudioTrack{
						Language: as.Lang,
						Codecs:   rep.Codecs,
						Segments: segs,
					}
					m.AudioTracks = append(m.AudioTracks, at)
				}

			case "text":
				for _, rep := range as.Representations {
					st := effectiveSegmentTemplate(as.SegmentTemplate, rep.SegmentTemplate)
					var segs []manifest.Segment
					if st != nil {
						segDur := segmentDurationSec(st)
						var count int
						if totalDuration > 0 && segDur > 0 {
							count = int(totalDuration / segDur)
						}
						for i := 0; i < count; i++ {
							num := st.StartNumber + i
							uri := resolve(resolveTemplate(st.Media, rep.ID, num))
							segs = append(segs, manifest.Segment{
								URI:      uri,
								Duration: segDur,
								Sequence: num,
							})
						}
					} else if rep.BaseURL != "" {
						segs = append(segs, manifest.Segment{URI: resolve(rep.BaseURL)})
					}
					sub := manifest.SubTrack{
						Language: as.Lang,
						URI:      resolve(rep.BaseURL),
						Segments: segs,
					}
					m.SubtitleTracks = append(m.SubtitleTracks, sub)
				}
			}
		}
	}

	return m, nil
}

// effectiveSegmentTemplate returns the representation-level template if set,
// otherwise falls back to the adaptation-set level template.
func effectiveSegmentTemplate(asST, repST *segmentTemplateXML) *segmentTemplateXML {
	if repST != nil {
		return repST
	}
	return asST
}

// extractDRM builds a DRMInfo from a slice of ContentProtection elements.
// Prefers the CENC default_KID entry for KeyID and the system-specific entry for PSSH.
func extractDRM(cps []contentProtectionXML) *manifest.DRMInfo {
	drm := &manifest.DRMInfo{}
	for _, cp := range cps {
		if cp.DefaultKID != "" {
			drm.KeyID = cp.DefaultKID
			drm.SchemeURI = cp.SchemeIdUri
		}
		if cp.PSSH != "" {
			drm.PSSH = cp.PSSH
			if drm.SchemeURI == "" {
				drm.SchemeURI = cp.SchemeIdUri
			}
		}
		// Always set SchemeURI if not yet set
		if drm.SchemeURI == "" && cp.SchemeIdUri != "" {
			drm.SchemeURI = cp.SchemeIdUri
		}
	}
	if drm.SchemeURI == "" && drm.KeyID == "" && drm.PSSH == "" {
		return nil
	}
	return drm
}
