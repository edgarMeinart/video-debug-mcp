package output

import "fmt"

// Verbosity controls the level of detail in analysis output.
type Verbosity string

const (
	VerbositySummary  Verbosity = "summary"
	VerbosityStandard Verbosity = "standard"
	VerbosityDeep     Verbosity = "deep"
	VerbosityForensic Verbosity = "forensic"
)

// ParseVerbosity converts a string to a Verbosity value, returning an error for unknown values.
func ParseVerbosity(s string) (Verbosity, error) {
	switch Verbosity(s) {
	case VerbositySummary, VerbosityStandard, VerbosityDeep, VerbosityForensic:
		return Verbosity(s), nil
	default:
		return "", fmt.Errorf("invalid verbosity %q: must be one of summary, standard, deep, forensic", s)
	}
}

// Severity indicates the seriousness of an anomaly.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// ErrorCode is a machine-readable error classification string.
type ErrorCode string

const (
	ErrManifestFetch   ErrorCode = "MANIFEST_FETCH_FAILED"
	ErrSegmentFetch    ErrorCode = "SEGMENT_FETCH_FAILED"
	ErrDecode          ErrorCode = "DECODE_FAILED"
	ErrToolNotFound    ErrorCode = "TOOL_NOT_FOUND"
	ErrInvalidInput    ErrorCode = "INVALID_INPUT"
	ErrTimeout         ErrorCode = "TIMEOUT"
	ErrInternalFailure ErrorCode = "INTERNAL_FAILURE"
)

// ErrorResponse is returned when a tool call fails.
type ErrorResponse struct {
	Code    ErrorCode         `json:"code"`
	Message string            `json:"message"`
	Phase   string            `json:"phase,omitempty"`
	URL     string            `json:"url,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

// Anomaly represents a detected problem or notable condition in the media.
type Anomaly struct {
	Severity    Severity `json:"severity"`
	Description string   `json:"description"`
	Detail      string   `json:"detail,omitempty"`
	Timestamp   float64  `json:"timestamp,omitempty"`
	StreamIndex int      `json:"stream_index,omitempty"`
}

// StreamInfo summarises a single audio, video, or data stream.
type StreamInfo struct {
	Index      int     `json:"index"`
	Type       string  `json:"type"`
	Codec      string  `json:"codec,omitempty"`
	Width      int     `json:"width,omitempty"`
	Height     int     `json:"height,omitempty"`
	Bitrate    int64   `json:"bitrate,omitempty"`
	FrameRate  float64 `json:"frame_rate,omitempty"`
	Duration   float64 `json:"duration,omitempty"`
	Language   string  `json:"language,omitempty"`
	SampleRate int     `json:"sample_rate,omitempty"`
	Channels   int     `json:"channels,omitempty"`
}

// CodecDetails holds extended codec-level information for a stream.
type CodecDetails struct {
	Profile      string `json:"profile,omitempty"`
	Level        string `json:"level,omitempty"`
	PixelFormat  string `json:"pixel_format,omitempty"`
	ColorSpace   string `json:"color_space,omitempty"`
	SampleRate   int    `json:"sample_rate,omitempty"`
	Channels     int    `json:"channels,omitempty"`
	ChannelLayout string `json:"channel_layout,omitempty"`
}

// TimingInfo captures timing-related metadata.
type TimingInfo struct {
	StartTime   float64 `json:"start_time,omitempty"`
	Duration    float64 `json:"duration,omitempty"`
	Bitrate     int64   `json:"bitrate,omitempty"`
	TimeBase    string  `json:"time_base,omitempty"`
	PTSWrapBits int     `json:"pts_wrap_bits,omitempty"`
}

// BitrateInfo captures bitrate measurements over time or per-segment.
type BitrateInfo struct {
	Min     int64   `json:"min,omitempty"`
	Max     int64   `json:"max,omitempty"`
	Average int64   `json:"average,omitempty"`
	Unit    string  `json:"unit,omitempty"`
	Samples []int64 `json:"samples,omitempty"`
}

// BoxTree represents an ISO BMFF / MP4 box hierarchy node.
type BoxTree struct {
	Type     string    `json:"type"`
	Size     int64     `json:"size,omitempty"`
	Offset   int64     `json:"offset,omitempty"`
	Children []BoxTree `json:"children,omitempty"`
}

// VariantInfo describes a single HLS/DASH variant or representation.
type VariantInfo struct {
	URL        string `json:"url"`
	Bandwidth  int64  `json:"bandwidth,omitempty"`
	Resolution string `json:"resolution,omitempty"`
	Codecs     string `json:"codecs,omitempty"`
	Language   string `json:"language,omitempty"`
	GroupID    string `json:"group_id,omitempty"`
}

// DRMSummary summarises DRM protection found in the manifest or container.
type DRMSummary struct {
	System      string `json:"system,omitempty"`
	KeyID       string `json:"key_id,omitempty"`
	LicenseURL  string `json:"license_url,omitempty"`
	Encrypted   bool   `json:"encrypted"`
}

// ManifestSummary holds parsed information from an HLS or DASH manifest.
type ManifestSummary struct {
	Type          string        `json:"type,omitempty"`
	URL           string        `json:"url,omitempty"`
	Variants      []VariantInfo `json:"variants,omitempty"`
	VariantCount  int           `json:"variant_count,omitempty"`
	DRM           *DRMSummary   `json:"drm,omitempty"`
	SegmentCount  int           `json:"segment_count,omitempty"`
	TotalDuration float64       `json:"total_duration,omitempty"`
}

// DownloadedAsset records a fetched segment or file for offline inspection.
type DownloadedAsset struct {
	URL       string `json:"url"`
	Path      string `json:"path,omitempty"`
	LocalPath string `json:"local_path,omitempty"`
	Size      int64  `json:"size,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	MimeType  string `json:"mime_type,omitempty"`
	Error     string `json:"error,omitempty"`
}

// AnalysisResult is the top-level output type returned by all analysis tools.
type AnalysisResult struct {
	Tool            string            `json:"tool"`
	Command         string            `json:"command,omitempty"`
	InputURL        string            `json:"input_url,omitempty"`
	Streams         []StreamInfo      `json:"streams,omitempty"`
	Codec           *CodecDetails     `json:"codec,omitempty"`
	Timing          *TimingInfo       `json:"timing,omitempty"`
	Bitrate         *BitrateInfo      `json:"bitrate,omitempty"`
	Manifest        *ManifestSummary  `json:"manifest,omitempty"`
	ContainerLayout []BoxTree         `json:"container_layout,omitempty"`
	Anomalies       []Anomaly         `json:"anomalies,omitempty"`
	RawOutput       string            `json:"raw_output,omitempty"`
	ResolvedURLs    []string          `json:"resolved_urls,omitempty"`
	Downloads       []DownloadedAsset `json:"downloads,omitempty"`
	Verbosity       Verbosity         `json:"verbosity,omitempty"`
}
