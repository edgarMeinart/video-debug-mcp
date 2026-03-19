package tools

import (
	"encoding/json"
	"github.com/meinart/video-debug-mcp/internal/output"
)

type Tool interface {
	Name() string
	Description() string
	InputSchema() json.RawMessage
	DockerImage() string
	BuildCommand(input ToolInput) (*DockerCommand, error)
	ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error)
}

type ToolInput struct {
	URL         string            `json:"url"`
	Verbosity   string            `json:"verbosity,omitempty"`
	Duration    int               `json:"duration,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Args        []string          `json:"args,omitempty"`
	DownloadAll bool              `json:"download_all,omitempty"`
}

func (ti ToolInput) ParsedVerbosity() (output.Verbosity, error) {
	if ti.Verbosity == "" {
		return output.VerbosityStandard, nil
	}
	return output.ParseVerbosity(ti.Verbosity)
}

type DockerCommand struct {
	Binary     string
	Args       []string
	InputFiles []string
}
