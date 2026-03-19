package mp4

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

const mp4boxInputPath = "/workspace/input"

type MP4Box struct {
	image string
}

func NewMP4Box(image string) *MP4Box {
	return &MP4Box{image: image}
}

func (t *MP4Box) Name() string { return "run_mp4box" }

func (t *MP4Box) Description() string {
	return "Run MP4Box -info to inspect MP4 container structure, streams, and timing"
}

func (t *MP4Box) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "Path or URL to the MP4 file"},
			"verbosity": {"type": "string", "enum": ["summary", "standard", "deep", "forensic"]}
		},
		"required": ["url"]
	}`)
}

func (t *MP4Box) DockerImage() string { return t.image }

// BuildCommand builds the MP4Box command for the given input.
func (t *MP4Box) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	if input.URL == "" {
		return nil, fmt.Errorf("mp4box: url is required")
	}

	inputPath := mp4boxInputPath
	if input.URL != "" && !isURL(input.URL) {
		inputPath = filepath.Join("/workspace", filepath.Base(input.URL))
	}

	return &tools.DockerCommand{
		Binary:     "MP4Box",
		Args:       []string{"-info", inputPath},
		InputFiles: []string{input.URL},
	}, nil
}

// ParseOutput parses MP4Box -info text output into an AnalysisResult.
func (t *MP4Box) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	result, err := parseMP4BoxOutput(raw, t.Name())
	if err != nil {
		return nil, err
	}
	result.Verbosity = verbosity
	if verbosity == output.VerbosityDeep || verbosity == output.VerbosityForensic {
		result.RawOutput = string(raw)
	}
	return result, nil
}

// isURL returns true if the string looks like an HTTP/HTTPS URL.
func isURL(s string) bool {
	return len(s) > 7 && (s[:7] == "http://" || s[:8] == "https://")
}
