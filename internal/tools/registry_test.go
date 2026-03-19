package tools

import (
	"encoding/json"
	"testing"
	"github.com/meinart/video-debug-mcp/internal/output"
)

type mockTool struct {
	name        string
	description string
	image       string
}

func (m *mockTool) Name() string                   { return m.name }
func (m *mockTool) Description() string            { return m.description }
func (m *mockTool) InputSchema() json.RawMessage    { return json.RawMessage(`{}`) }
func (m *mockTool) DockerImage() string             { return m.image }
func (m *mockTool) BuildCommand(input ToolInput) (*DockerCommand, error) {
	return &DockerCommand{Binary: "test", Args: []string{}}, nil
}
func (m *mockTool) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	return &output.AnalysisResult{Tool: m.name}, nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	tool := &mockTool{name: "ffprobe", description: "test", image: "test-image"}
	reg.Register(tool)
	got, ok := reg.Get("ffprobe")
	if !ok { t.Fatal("expected to find ffprobe in registry") }
	if got.Name() != "ffprobe" { t.Errorf("expected name=ffprobe, got %s", got.Name()) }
}

func TestRegistry_GetNotFound(t *testing.T) {
	reg := NewRegistry()
	_, ok := reg.Get("nonexistent")
	if ok { t.Error("expected not to find nonexistent tool") }
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockTool{name: "ffprobe", description: "a", image: "i"})
	reg.Register(&mockTool{name: "mp4box", description: "b", image: "j"})
	tools := reg.List()
	if len(tools) != 2 { t.Errorf("expected 2 tools, got %d", len(tools)) }
}

func TestToolInput_Verbosity(t *testing.T) {
	input := ToolInput{URL: "https://example.com/video.mp4", Verbosity: "deep"}
	v, err := input.ParsedVerbosity()
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if v != output.VerbosityDeep { t.Errorf("expected deep, got %v", v) }
}

func TestToolInput_DefaultVerbosity(t *testing.T) {
	input := ToolInput{URL: "https://example.com/video.mp4"}
	v, err := input.ParsedVerbosity()
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if v != output.VerbosityStandard { t.Errorf("expected standard (default), got %v", v) }
}
