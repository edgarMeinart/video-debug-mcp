package mediainfo

import (
	"encoding/json"

	"github.com/meinart/video-debug-mcp/internal/output"
	tools "github.com/meinart/video-debug-mcp/internal/tools"
)

const mediainfoName = "run_mediainfo"

// MediaInfo implements tools.Tool using mediainfo to extract media metadata.
type MediaInfo struct {
	image string
}

// NewMediaInfo creates a MediaInfo tool that runs inside the given Docker image.
func NewMediaInfo(image string) *MediaInfo {
	return &MediaInfo{image: image}
}

// Name returns the MCP tool name.
func (m *MediaInfo) Name() string {
	return mediainfoName
}

// Description returns a human-readable description of the tool.
func (m *MediaInfo) Description() string {
	return "Run mediainfo on a media file and return JSON-formatted stream and format metadata."
}

// InputSchema returns the JSON Schema for the tool's input.
func (m *MediaInfo) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "url":       {"type": "string", "description": "URL or path to the media file"},
    "verbosity": {"type": "string", "enum": ["summary","standard","deep","forensic"]},
    "args":      {"type": "array",  "items": {"type": "string"}, "description": "Override mediainfo arguments"}
  },
  "required": ["url"]
}`)
}

// DockerImage returns the container image used to run mediainfo.
func (m *MediaInfo) DockerImage() string {
	return m.image
}

// BuildCommand constructs the mediainfo Docker command for the given input.
// If input.Args is non-empty the caller's args are used verbatim; otherwise a
// sensible set of defaults is applied.
func (m *MediaInfo) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	args := input.Args
	if len(args) == 0 {
		args = []string{
			"--Output=JSON",
			"/workspace/input",
		}
	}

	return &tools.DockerCommand{
		Binary:     "mediainfo",
		Args:       args,
		InputFiles: []string{input.URL},
	}, nil
}

// ParseOutput parses raw mediainfo JSON output into an AnalysisResult.
func (m *MediaInfo) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	result, err := parseMediaInfoOutput(raw, m.Name())
	if err != nil {
		return nil, err
	}
	result.Verbosity = verbosity
	return result, nil
}
