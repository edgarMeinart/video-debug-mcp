package ffmpeg

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"

	tools "github.com/meinart/video-debug-mcp/internal/tools"
	"github.com/meinart/video-debug-mcp/internal/output"
)

const ffmpegName = "run_ffmpeg"

// Patterns for anomaly detection in ffmpeg output.
var (
	rePTSDiscontinuity = regexp.MustCompile(`discarding pts discontinuity`)
	reConcealment      = regexp.MustCompile(`concealing.*errors`)
	reDTSDiscontinuity = regexp.MustCompile(`DTS.*discontinuity`)
)

// FFmpeg implements tools.Tool using ffmpeg to perform playback simulation via
// the null muxer, detecting decode errors and timing anomalies.
type FFmpeg struct {
	image string
}

// NewFFmpeg creates an FFmpeg tool that runs inside the given Docker image.
func NewFFmpeg(image string) *FFmpeg {
	return &FFmpeg{image: image}
}

// Name returns the MCP tool name.
func (f *FFmpeg) Name() string {
	return ffmpegName
}

// Description returns a human-readable description of the tool.
func (f *FFmpeg) Description() string {
	return "Run ffmpeg playback simulation using the null muxer to detect decode errors and timing anomalies."
}

// InputSchema returns the JSON Schema for the tool's input.
func (f *FFmpeg) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "url":       {"type": "string", "description": "URL or path to the media file"},
    "verbosity": {"type": "string", "enum": ["summary","standard","deep","forensic"]},
    "duration":  {"type": "integer", "description": "Limit playback to this many seconds"},
    "args":      {"type": "array",  "items": {"type": "string"}, "description": "Override ffmpeg arguments"}
  },
  "required": ["url"]
}`)
}

// DockerImage returns the container image used to run ffmpeg.
func (f *FFmpeg) DockerImage() string {
	return f.image
}

// BuildCommand constructs the ffmpeg Docker command for the given input.
// Default command: ffmpeg -v verbose [-t <duration>] -i /workspace/input -f null -
// If input.Args is non-empty the caller's args are used verbatim.
func (f *FFmpeg) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	if len(input.Args) > 0 {
		return &tools.DockerCommand{
			Binary:     "ffmpeg",
			Args:       input.Args,
			InputFiles: []string{input.URL},
		}, nil
	}

	args := []string{"-v", "verbose"}

	if input.Duration > 0 {
		args = append(args, "-t", strconv.Itoa(input.Duration))
	}

	args = append(args, "-i", "/workspace/input", "-f", "null", "-")

	return &tools.DockerCommand{
		Binary:     "ffmpeg",
		Args:       args,
		InputFiles: []string{input.URL},
	}, nil
}

// ParseOutput scans raw ffmpeg output for known anomaly patterns and returns
// an AnalysisResult with any detected warnings and errors.
func (f *FFmpeg) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	result := &output.AnalysisResult{
		Tool:      f.Name(),
		Verbosity: verbosity,
		RawOutput: string(raw),
	}

	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case rePTSDiscontinuity.MatchString(line):
			result.Anomalies = append(result.Anomalies, output.Anomaly{
				Severity:    output.SeverityWarning,
				Description: "PTS discontinuity detected",
				Detail:      fmt.Sprintf("ffmpeg reported: %s", line),
			})

		case reConcealment.MatchString(line):
			result.Anomalies = append(result.Anomalies, output.Anomaly{
				Severity:    output.SeverityError,
				Description: "Decode concealment errors detected",
				Detail:      fmt.Sprintf("ffmpeg reported: %s", line),
			})

		case reDTSDiscontinuity.MatchString(line):
			result.Anomalies = append(result.Anomalies, output.Anomaly{
				Severity:    output.SeverityWarning,
				Description: "DTS discontinuity detected",
				Detail:      fmt.Sprintf("ffmpeg reported: %s", line),
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading ffmpeg output: %w", err)
	}

	return result, nil
}
