package bento4

import (
	"encoding/json"

	tools "github.com/meinart/video-debug-mcp/internal/tools"
	"github.com/meinart/video-debug-mcp/internal/output"
)

const bento4Name = "run_bento4"

// Bento4 implements tools.Tool using Bento4 utilities (mp4info, mp4dump, etc.)
// to extract media metadata.
type Bento4 struct {
	image string
}

// NewBento4 creates a Bento4 tool that runs inside the given Docker image.
func NewBento4(image string) *Bento4 {
	return &Bento4{image: image}
}

// Name returns the MCP tool name.
func (b *Bento4) Name() string {
	return bento4Name
}

// Description returns a human-readable description of the tool.
func (b *Bento4) Description() string {
	return "Run a Bento4 utility (mp4info, mp4dump, etc.) on a media file and return JSON-formatted metadata."
}

// InputSchema returns the JSON Schema for the tool's input.
func (b *Bento4) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "url":       {"type": "string", "description": "URL or path to the media file"},
    "verbosity": {"type": "string", "enum": ["summary","standard","deep","forensic"]},
    "args":      {"type": "array",  "items": {"type": "string"}, "description": "Override arguments; first element is the Bento4 sub-tool binary (e.g. mp4dump)"}
  },
  "required": ["url"]
}`)
}

// DockerImage returns the container image used to run the Bento4 tool.
func (b *Bento4) DockerImage() string {
	return b.image
}

// BuildCommand constructs the Bento4 Docker command for the given input.
// If input.Args is non-empty, the first element is used as the binary name and
// the remaining elements are passed as arguments verbatim.
// Otherwise the default command is: mp4info --format json /workspace/input
func (b *Bento4) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	if len(input.Args) > 0 {
		binary := input.Args[0]
		args := input.Args[1:]
		return &tools.DockerCommand{
			Binary:     binary,
			Args:       args,
			InputFiles: []string{input.URL},
		}, nil
	}

	return &tools.DockerCommand{
		Binary:     "mp4info",
		Args:       []string{"--format", "json", "/workspace/input"},
		InputFiles: []string{input.URL},
	}, nil
}

// ParseOutput parses raw Bento4 JSON output into an AnalysisResult.
func (b *Bento4) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	result, err := parseBento4Output(raw, b.Name())
	if err != nil {
		return nil, err
	}
	result.Verbosity = verbosity
	return result, nil
}
