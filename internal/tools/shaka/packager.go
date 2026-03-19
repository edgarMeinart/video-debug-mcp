package shaka

import (
	"encoding/json"

	"github.com/meinart/video-debug-mcp/internal/output"
	tools "github.com/meinart/video-debug-mcp/internal/tools"
)

const shakaName = "run_shaka_packager"

// Shaka implements tools.Tool using Shaka Packager to inspect media streams.
type Shaka struct {
	image string
}

// NewShaka creates a Shaka tool that runs inside the given Docker image.
func NewShaka(image string) *Shaka {
	return &Shaka{image: image}
}

// Name returns the MCP tool name.
func (s *Shaka) Name() string {
	return shakaName
}

// Description returns a human-readable description of the tool.
func (s *Shaka) Description() string {
	return "Run Shaka Packager on a media file to inspect streams and detect packaging anomalies."
}

// InputSchema returns the JSON Schema for the tool's input.
func (s *Shaka) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "url":       {"type": "string", "description": "URL or path to the media file"},
    "verbosity": {"type": "string", "enum": ["summary","standard","deep","forensic"]},
    "args":      {"type": "array",  "items": {"type": "string"}, "description": "Override packager arguments"}
  },
  "required": ["url"]
}`)
}

// DockerImage returns the container image used to run Shaka Packager.
func (s *Shaka) DockerImage() string {
	return s.image
}

// BuildCommand constructs the Shaka Packager Docker command for the given input.
// If input.Args is non-empty the caller's args are used verbatim; otherwise a
// sensible set of defaults is applied.
func (s *Shaka) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	args := input.Args
	if len(args) == 0 {
		args = []string{
			"input=/workspace/input,stream=video,output=/tmp/out.mp4",
			"--dump_stream_info",
		}
	}

	return &tools.DockerCommand{
		Binary:     "packager",
		Args:       args,
		InputFiles: []string{input.URL},
	}, nil
}

// ParseOutput parses raw Shaka Packager log output into an AnalysisResult.
func (s *Shaka) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	result, err := parseShakaOutput(raw, s.Name())
	if err != nil {
		return nil, err
	}
	result.Verbosity = verbosity
	return result, nil
}
