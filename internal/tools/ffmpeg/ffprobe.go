package ffmpeg

import (
	"encoding/json"

	tools "github.com/meinart/video-debug-mcp/internal/tools"
	"github.com/meinart/video-debug-mcp/internal/output"
)

const ffprobeName = "run_ffprobe"

// FFprobe implements tools.Tool using ffprobe to extract media metadata.
type FFprobe struct {
	image string
}

// NewFFprobe creates an FFprobe tool that runs inside the given Docker image.
func NewFFprobe(image string) *FFprobe {
	return &FFprobe{image: image}
}

// Name returns the MCP tool name.
func (f *FFprobe) Name() string {
	return ffprobeName
}

// Description returns a human-readable description of the tool.
func (f *FFprobe) Description() string {
	return "Run ffprobe on a media file and return JSON-formatted stream and format metadata."
}

// InputSchema returns the JSON Schema for the tool's input.
func (f *FFprobe) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "url":       {"type": "string", "description": "URL or path to the media file"},
    "verbosity": {"type": "string", "enum": ["summary","standard","deep","forensic"]},
    "args":      {"type": "array",  "items": {"type": "string"}, "description": "Override ffprobe arguments"}
  },
  "required": ["url"]
}`)
}

// DockerImage returns the container image used to run ffprobe.
func (f *FFprobe) DockerImage() string {
	return f.image
}

// BuildCommand constructs the ffprobe Docker command for the given input.
// If input.Args is non-empty the caller's args are used verbatim; otherwise a
// sensible set of defaults is applied.
func (f *FFprobe) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	args := input.Args
	if len(args) == 0 {
		args = []string{
			"-v", "quiet",
			"-print_format", "json",
			"-show_streams",
			"-show_format",
			input.URL,
		}
	}

	return &tools.DockerCommand{
		Binary:     "ffprobe",
		Args:       args,
		InputFiles: []string{input.URL},
	}, nil
}

// ParseOutput parses raw ffprobe JSON output into an AnalysisResult.
func (f *FFprobe) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	result, err := parseFFprobeOutput(raw, f.Name())
	if err != nil {
		return nil, err
	}
	result.Verbosity = verbosity
	return result, nil
}
