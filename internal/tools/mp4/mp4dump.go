package mp4

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

type MP4Dump struct {
	image string
}

func NewMP4Dump(image string) *MP4Dump {
	return &MP4Dump{image: image}
}

func (t *MP4Dump) Name() string { return "run_mp4dump" }

func (t *MP4Dump) Description() string {
	return "Run mp4dump to display the ISO BMFF box structure of an MP4 file"
}

func (t *MP4Dump) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "Path or URL to the MP4 file"},
			"verbosity": {"type": "string", "enum": ["summary", "standard", "deep", "forensic"]}
		},
		"required": ["url"]
	}`)
}

func (t *MP4Dump) DockerImage() string { return t.image }

// BuildCommand builds the mp4dump command for the given input.
func (t *MP4Dump) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	if input.URL == "" {
		return nil, fmt.Errorf("mp4dump: url is required")
	}

	inputPath := "/workspace/input"
	if !isURL(input.URL) {
		inputPath = filepath.Join("/workspace", filepath.Base(input.URL))
	}

	return &tools.DockerCommand{
		Binary:     "mp4dump",
		Args:       []string{inputPath},
		InputFiles: []string{input.URL},
	}, nil
}

// ParseOutput parses mp4dump text output into an AnalysisResult containing a BoxTree.
func (t *MP4Dump) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	boxes := parseMP4DumpOutput(raw)
	result := &output.AnalysisResult{
		Tool:            t.Name(),
		ContainerLayout: boxes,
		Verbosity:       verbosity,
	}
	if verbosity == output.VerbosityDeep || verbosity == output.VerbosityForensic {
		result.RawOutput = string(raw)
	}
	return result, nil
}
