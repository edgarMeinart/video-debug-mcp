package mediainfo

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/meinart/video-debug-mcp/internal/output"
)

// mediainfoJSON mirrors the top-level structure produced by mediainfo --Output=JSON.
type mediainfoJSON struct {
	Media mediainfoMedia `json:"media"`
}

type mediainfoMedia struct {
	Tracks []mediainfoTrack `json:"track"`
}

type mediainfoTrack struct {
	Type              string `json:"@type"`
	Format            string `json:"Format"`
	FormatProfile     string `json:"Format_Profile"`
	FormatLevel       string `json:"Format_Level"`
	CodecID           string `json:"CodecID"`
	Duration          string `json:"Duration"`
	BitRate           string `json:"BitRate"`
	OverallBitRate    string `json:"OverallBitRate"`
	FileSize          string `json:"FileSize"`
	Width             string `json:"Width"`
	Height            string `json:"Height"`
	FrameRate         string `json:"FrameRate"`
	PixelAspectRatio  string `json:"PixelAspectRatio"`
	ColorSpace        string `json:"ColorSpace"`
	ChromaSubsampling string `json:"ChromaSubsampling"`
	SamplingRate      string `json:"SamplingRate"`
	Channels          string `json:"Channels"`
	Language          string `json:"Language"`
}

// parseMediaInfoOutput parses mediainfo JSON output into an AnalysisResult.
func parseMediaInfoOutput(raw []byte, toolName string) (*output.AnalysisResult, error) {
	var data mediainfoJSON
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("failed to parse mediainfo JSON: %w", err)
	}

	result := &output.AnalysisResult{
		Tool: toolName,
	}

	var streamIndex int
	for _, track := range data.Media.Tracks {
		trackType := strings.ToLower(track.Type)

		// Skip General track — it describes the container, not a stream.
		if trackType == "general" {
			// Extract timing and bitrate from General track.
			timing := &output.TimingInfo{}
			if track.Duration != "" {
				if d, err := strconv.ParseFloat(track.Duration, 64); err == nil {
					timing.Duration = d
				}
			}
			if track.OverallBitRate != "" {
				if br, err := strconv.ParseInt(track.OverallBitRate, 10, 64); err == nil {
					timing.Bitrate = br
					result.Bitrate = &output.BitrateInfo{
						Average: br,
						Unit:    "bps",
					}
				}
			}
			result.Timing = timing
			continue
		}

		si := output.StreamInfo{
			Index: streamIndex,
			Type:  trackType,
			Codec: track.Format,
		}
		streamIndex++

		if track.BitRate != "" {
			if br, err := strconv.ParseInt(track.BitRate, 10, 64); err == nil {
				si.Bitrate = br
			}
		}

		if track.Duration != "" {
			if d, err := strconv.ParseFloat(track.Duration, 64); err == nil {
				si.Duration = d
			}
		}

		if trackType == "video" {
			if track.Width != "" {
				if w, err := strconv.Atoi(track.Width); err == nil {
					si.Width = w
				}
			}
			if track.Height != "" {
				if h, err := strconv.Atoi(track.Height); err == nil {
					si.Height = h
				}
			}
			if track.FrameRate != "" {
				if fr, err := strconv.ParseFloat(track.FrameRate, 64); err == nil {
					si.FrameRate = fr
				}
			}

			// Populate CodecDetails from video track.
			if result.Codec == nil {
				result.Codec = &output.CodecDetails{
					Profile:    track.FormatProfile,
					Level:      track.FormatLevel,
					ColorSpace: track.ColorSpace,
				}
			}
		}

		if trackType == "audio" {
			if track.SamplingRate != "" {
				if sr, err := strconv.Atoi(track.SamplingRate); err == nil {
					si.SampleRate = sr
				}
			}
			if track.Channels != "" {
				if ch, err := strconv.Atoi(track.Channels); err == nil {
					si.Channels = ch
				}
			}
			si.Language = track.Language
		}

		result.Streams = append(result.Streams, si)
	}

	return result, nil
}
