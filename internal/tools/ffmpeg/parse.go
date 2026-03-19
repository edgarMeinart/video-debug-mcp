package ffmpeg

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/meinart/video-debug-mcp/internal/output"
)

// ffprobeJSON mirrors the top-level structure produced by ffprobe -print_format json.
type ffprobeJSON struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	Index              int               `json:"index"`
	CodecName          string            `json:"codec_name"`
	CodecType          string            `json:"codec_type"`
	Profile            string            `json:"profile"`
	Level              int               `json:"level"`
	Width              int               `json:"width"`
	Height             int               `json:"height"`
	RFrameRate         string            `json:"r_frame_rate"`
	BitRate            string            `json:"bit_rate"`
	SampleAspectRatio  string            `json:"sample_aspect_ratio"`
	DisplayAspectRatio string            `json:"display_aspect_ratio"`
	PixFmt             string            `json:"pix_fmt"`
	SampleRate         string            `json:"sample_rate"`
	Channels           int               `json:"channels"`
	Tags               map[string]string `json:"tags"`
}

type ffprobeFormat struct {
	Duration   string `json:"duration"`
	StartTime  string `json:"start_time"`
	BitRate    string `json:"bit_rate"`
	FormatName string `json:"format_name"`
}

// parseFFprobeOutput parses ffprobe JSON output into an AnalysisResult.
func parseFFprobeOutput(raw []byte, toolName string) (*output.AnalysisResult, error) {
	var data ffprobeJSON
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe JSON: %w", err)
	}

	result := &output.AnalysisResult{
		Tool: toolName,
	}

	// Parse streams.
	for _, s := range data.Streams {
		si := output.StreamInfo{
			Index:  s.Index,
			Type:   s.CodecType,
			Codec:  s.CodecName,
			Width:  s.Width,
			Height: s.Height,
		}

		if s.BitRate != "" {
			if br, err := strconv.ParseInt(s.BitRate, 10, 64); err == nil {
				si.Bitrate = br
			}
		}

		if s.RFrameRate != "" {
			si.FrameRate = parseFrameRate(s.RFrameRate)
		}

		if s.SampleRate != "" {
			if sr, err := strconv.Atoi(s.SampleRate); err == nil {
				si.SampleRate = sr
			}
		}

		si.Channels = s.Channels

		if s.Tags != nil {
			si.Language = s.Tags["language"]
		}

		result.Streams = append(result.Streams, si)
	}

	// Parse format-level timing.
	timing := &output.TimingInfo{}
	if data.Format.Duration != "" {
		if d, err := strconv.ParseFloat(data.Format.Duration, 64); err == nil {
			timing.Duration = d
		}
	}
	if data.Format.StartTime != "" {
		if st, err := strconv.ParseFloat(data.Format.StartTime, 64); err == nil {
			timing.StartTime = st
		}
	}
	if data.Format.BitRate != "" {
		if br, err := strconv.ParseInt(data.Format.BitRate, 10, 64); err == nil {
			timing.Bitrate = br
		}
	}
	result.Timing = timing

	// Parse format-level bitrate into BitrateInfo.
	if data.Format.BitRate != "" {
		if br, err := strconv.ParseInt(data.Format.BitRate, 10, 64); err == nil {
			result.Bitrate = &output.BitrateInfo{
				Average: br,
				Unit:    "bps",
			}
		}
	}

	// Populate CodecDetails from the first video stream.
	for _, s := range data.Streams {
		if s.CodecType == "video" {
			level := ""
			if s.Level != 0 {
				// ffprobe level 40 → "4.0"
				major := s.Level / 10
				minor := s.Level % 10
				level = fmt.Sprintf("%d.%d", major, minor)
			}
			result.Codec = &output.CodecDetails{
				Profile:     s.Profile,
				Level:       level,
				PixelFormat: s.PixFmt,
			}
			break
		}
	}

	return result, nil
}

// parseFrameRate converts a rational frame rate string like "30000/1001" to a float64.
func parseFrameRate(s string) float64 {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}
	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return 0
	}
	return num / den
}
