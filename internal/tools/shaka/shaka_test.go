package shaka_test

import (
	"os"
	"testing"

	"github.com/meinart/video-debug-mcp/internal/output"
	tools "github.com/meinart/video-debug-mcp/internal/tools"
	"github.com/meinart/video-debug-mcp/internal/tools/shaka"
)

const testImage = "google/shaka-packager:latest"

func newTool() *shaka.Shaka {
	return shaka.NewShaka(testImage)
}

func TestShaka_Name(t *testing.T) {
	s := newTool()
	if got := s.Name(); got != "run_shaka_packager" {
		t.Errorf("Name() = %q, want %q", got, "run_shaka_packager")
	}
}

func TestShaka_BuildCommand(t *testing.T) {
	s := newTool()
	input := tools.ToolInput{URL: "http://example.com/video.mp4"}
	cmd, err := s.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() returned error: %v", err)
	}

	if cmd.Binary != "packager" {
		t.Errorf("Binary = %q, want %q", cmd.Binary, "packager")
	}

	if len(cmd.Args) == 0 {
		t.Fatal("Args is empty, expected default arguments")
	}
}

func TestShaka_ParseOutput(t *testing.T) {
	raw, err := os.ReadFile("testdata/shaka_output.txt")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	s := newTool()
	result, err := s.ParseOutput(raw, output.VerbosityStandard)
	if err != nil {
		t.Fatalf("ParseOutput() returned error: %v", err)
	}

	// --- Anomalies: should detect the WARNING line ---
	if len(result.Anomalies) == 0 {
		t.Fatal("Anomalies is empty, expected at least one warning anomaly")
	}

	foundWarning := false
	for _, a := range result.Anomalies {
		if a.Severity == output.SeverityWarning {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("no warning anomaly found in %+v", result.Anomalies)
	}
}
