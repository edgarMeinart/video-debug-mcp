package ffmpeg

import (
	"os"
	"testing"

	"github.com/meinart/video-debug-mcp/internal/output"
	tools "github.com/meinart/video-debug-mcp/internal/tools"
)

func TestFFmpeg_Name(t *testing.T) {
	f := NewFFmpeg("linuxserver/ffmpeg:latest")
	if got := f.Name(); got != "run_ffmpeg" {
		t.Errorf("Name() = %q, want %q", got, "run_ffmpeg")
	}
}

func TestFFmpeg_BuildCommand_DefaultDuration(t *testing.T) {
	f := NewFFmpeg("linuxserver/ffmpeg:latest")
	input := tools.ToolInput{URL: "http://example.com/video.mp4"}

	cmd, err := f.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() error: %v", err)
	}

	if cmd.Binary != "ffmpeg" {
		t.Errorf("Binary = %q, want %q", cmd.Binary, "ffmpeg")
	}

	// Expected args: -v verbose -i /workspace/input -f null -
	want := []string{"-v", "verbose", "-i", "/workspace/input", "-f", "null", "-"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("Args = %v, want %v", cmd.Args, want)
	}
	for i, a := range want {
		if cmd.Args[i] != a {
			t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], a)
		}
	}
}

func TestFFmpeg_BuildCommand_CustomDuration(t *testing.T) {
	f := NewFFmpeg("linuxserver/ffmpeg:latest")
	input := tools.ToolInput{URL: "http://example.com/video.mp4", Duration: 30}

	cmd, err := f.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() error: %v", err)
	}

	// Must contain -t 30
	found := false
	for i, a := range cmd.Args {
		if a == "-t" && i+1 < len(cmd.Args) && cmd.Args[i+1] == "30" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Args %v does not contain -t 30", cmd.Args)
	}
}

func TestFFmpeg_ParseOutput(t *testing.T) {
	raw, err := os.ReadFile("testdata/ffmpeg_null_output.txt")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	f := NewFFmpeg("linuxserver/ffmpeg:latest")
	result, err := f.ParseOutput(raw, output.VerbosityStandard)
	if err != nil {
		t.Fatalf("ParseOutput() error: %v", err)
	}

	var warnings, errors int
	for _, a := range result.Anomalies {
		switch a.Severity {
		case output.SeverityWarning:
			warnings++
		case output.SeverityError:
			errors++
		}
	}

	if warnings == 0 {
		t.Error("expected at least one warning anomaly (PTS discontinuity), got none")
	}
	if errors == 0 {
		t.Error("expected at least one error anomaly (concealment errors), got none")
	}
}
