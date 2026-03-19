package manifest

import "net/url"

type ManifestType string

const (
	ManifestTypeHLS  ManifestType = "hls"
	ManifestTypeDASH ManifestType = "dash"
)

type Manifest struct {
	Type           ManifestType
	URL            string
	Variants       []Variant
	AudioTracks    []AudioTrack
	SubtitleTracks []SubTrack
	InitSegments   []Segment
	MediaSegments  []Segment
	DRM            *DRMInfo
	Raw            string
}

type Variant struct {
	URI        string
	Bandwidth  int
	Resolution string
	Codecs     string
	FrameRate  float64
	Segments   []Segment
}

type AudioTrack struct {
	URI      string
	Language string
	Name     string
	Codecs   string
	Channels int
	Segments []Segment
}

type SubTrack struct {
	URI      string
	Language string
	Name     string
	Format   string
	Segments []Segment
}

type Segment struct {
	URI      string
	Duration float64
	Sequence int
	IsInit   bool
	TrackRef string
}

type DRMInfo struct {
	System    string
	SchemeURI string
	KeyID     string
	PSSH      string
	EXTXKey   string
}

func (m *Manifest) AllSegmentURLs() []string {
	seen := make(map[string]bool)
	var urls []string
	add := func(uri string) {
		if uri != "" && !seen[uri] { seen[uri] = true; urls = append(urls, uri) }
	}
	for _, s := range m.InitSegments { add(s.URI) }
	for _, v := range m.Variants { for _, s := range v.Segments { add(s.URI) } }
	for _, a := range m.AudioTracks { for _, s := range a.Segments { add(s.URI) } }
	for _, st := range m.SubtitleTracks { for _, s := range st.Segments { add(s.URI) } }
	for _, s := range m.MediaSegments { add(s.URI) }
	return urls
}

func (m *Manifest) SampledSegmentURLs(n int) []string {
	seen := make(map[string]bool)
	var urls []string
	add := func(uri string) {
		if uri != "" && !seen[uri] { seen[uri] = true; urls = append(urls, uri) }
	}
	for _, s := range m.InitSegments { add(s.URI) }
	sampleSlice := func(segs []Segment) {
		if len(segs) <= 2*n {
			for _, s := range segs { add(s.URI) }
			return
		}
		for i := 0; i < n; i++ { add(segs[i].URI) }
		for i := len(segs) - n; i < len(segs); i++ { add(segs[i].URI) }
	}
	for _, v := range m.Variants { sampleSlice(v.Segments) }
	for _, a := range m.AudioTracks { sampleSlice(a.Segments) }
	for _, st := range m.SubtitleTracks { sampleSlice(st.Segments) }
	return urls
}

func ResolveURI(base, ref string) (string, error) {
	refURL, err := url.Parse(ref)
	if err != nil { return "", err }
	if refURL.IsAbs() { return ref, nil }
	baseURL, err := url.Parse(base)
	if err != nil { return "", err }
	return baseURL.ResolveReference(refURL).String(), nil
}
