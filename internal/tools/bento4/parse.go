package bento4

import (
	"encoding/json"
	"fmt"

	"github.com/meinart/video-debug-mcp/internal/output"
)

// bento4JSON mirrors the top-level structure produced by mp4info --format json.
type bento4JSON struct {
	Movie bento4Movie `json:"movie"`
}

type bento4Movie struct {
	DurationMS int64         `json:"duration_ms"`
	Timescale  int           `json:"timescale"`
	Fragments  bool          `json:"fragments"`
	Tracks     []bento4Track `json:"tracks"`
}

type bento4Track struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	Codec       string `json:"codec"`
	DurationMS  int64  `json:"duration_ms"`
	Timescale   int    `json:"timescale"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	SampleCount int    `json:"sample_count"`
	Bitrate     int64  `json:"bitrate"`
	SampleRate  int    `json:"sample_rate"`
	Channels    int    `json:"channels"`
}

// parseBento4Output parses mp4info JSON output into an AnalysisResult.
func parseBento4Output(raw []byte, toolName string) (*output.AnalysisResult, error) {
	var data bento4JSON
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("failed to parse bento4 JSON: %w", err)
	}

	result := &output.AnalysisResult{
		Tool: toolName,
	}

	// Parse timing from the movie-level duration.
	durationSec := float64(data.Movie.DurationMS) / 1000.0
	result.Timing = &output.TimingInfo{
		Duration: durationSec,
	}

	// Parse tracks into streams.
	for i, t := range data.Movie.Tracks {
		si := output.StreamInfo{
			Index:   i,
			Type:    t.Type,
			Codec:   t.Codec,
			Width:   t.Width,
			Height:  t.Height,
			Bitrate: t.Bitrate,
		}

		if t.DurationMS > 0 {
			si.Duration = float64(t.DurationMS) / 1000.0
		}

		if t.SampleRate > 0 {
			si.SampleRate = t.SampleRate
		}

		si.Channels = t.Channels

		result.Streams = append(result.Streams, si)
	}

	return result, nil
}
