package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/meinart/video-debug-mcp/internal/config"
	"github.com/meinart/video-debug-mcp/internal/docker"
	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

type mockRunner struct {
	result *docker.RunResult
	err    error
}

func (m *mockRunner) Run(ctx context.Context, req docker.RunRequest) (*docker.RunResult, error) {
	return m.result, m.err
}

type mockTool struct {
	name     string
	image    string
	cmd      *tools.DockerCommand
	cmdErr   error
	parsed   *output.AnalysisResult
	parseErr error
}

func (m *mockTool) Name() string                { return m.name }
func (m *mockTool) Description() string         { return "mock" }
func (m *mockTool) InputSchema() json.RawMessage { return json.RawMessage(`{}`) }
func (m *mockTool) DockerImage() string          { return m.image }
func (m *mockTool) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	return m.cmd, m.cmdErr
}
func (m *mockTool) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	return m.parsed, m.parseErr
}

func TestOrchestrator_ExecuteTool_DirectAsset(t *testing.T) {
	runner := &mockRunner{
		result: &docker.RunResult{ExitCode: 0, Stdout: []byte(`{}`), Duration: 100 * time.Millisecond},
	}
	tool := &mockTool{
		name: "test_tool", image: "test-image",
		cmd:    &tools.DockerCommand{Binary: "testtool", Args: []string{"/workspace/input"}, InputFiles: []string{"input"}},
		parsed: &output.AnalysisResult{Tool: "test_tool", Streams: []output.StreamInfo{{Index: 0, Type: "video", Codec: "h264"}}},
	}
	cfg := config.Default()
	orch := NewOrchestrator(runner, output.NewFormatter(), cfg)
	input := tools.ToolInput{URL: "https://example.com/video.mp4", Verbosity: "standard"}
	result, err := orch.ExecuteToolDirect(context.Background(), tool, input, "/tmp/test-workspace")
	if err != nil {
		t.Fatalf("ExecuteToolDirect() error: %v", err)
	}
	if result.Tool != "test_tool" {
		t.Errorf("expected tool=test_tool, got %s", result.Tool)
	}
}

func TestOrchestrator_ExecuteTool_RunnerError(t *testing.T) {
	runner := &mockRunner{
		result: &docker.RunResult{ExitCode: 1, Stderr: []byte("command not found"), Duration: 50 * time.Millisecond},
	}
	tool := &mockTool{
		name: "test_tool", image: "test-image",
		cmd: &tools.DockerCommand{Binary: "testtool", Args: []string{"/workspace/input"}, InputFiles: []string{"input"}},
	}
	cfg := config.Default()
	orch := NewOrchestrator(runner, output.NewFormatter(), cfg)
	input := tools.ToolInput{URL: "https://example.com/video.mp4"}
	_, err := orch.ExecuteToolDirect(context.Background(), tool, input, "/tmp/test-workspace")
	if err == nil {
		t.Error("expected error for non-zero exit code, got nil")
	}
}
