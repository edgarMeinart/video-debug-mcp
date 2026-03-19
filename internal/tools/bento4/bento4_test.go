package bento4

import (
	"os"
	"testing"

	tools "github.com/meinart/video-debug-mcp/internal/tools"
	"github.com/meinart/video-debug-mcp/internal/output"
)

func TestBento4_Name(t *testing.T) {
	b := NewBento4("bento4:latest")
	if got := b.Name(); got != "run_bento4" {
		t.Errorf("Name() = %q, want %q", got, "run_bento4")
	}
}

func TestBento4_BuildCommand_DefaultMP4Info(t *testing.T) {
	b := NewBento4("bento4:latest")
	input := tools.ToolInput{URL: "http://example.com/video.mp4"}

	cmd, err := b.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() error: %v", err)
	}

	if cmd.Binary != "mp4info" {
		t.Errorf("Binary = %q, want %q", cmd.Binary, "mp4info")
	}

	// Verify --format json is included.
	foundFormat := false
	foundJSON := false
	for i, arg := range cmd.Args {
		if arg == "--format" {
			foundFormat = true
			if i+1 < len(cmd.Args) && cmd.Args[i+1] == "json" {
				foundJSON = true
			}
		}
	}
	if !foundFormat || !foundJSON {
		t.Errorf("Args %v missing --format json", cmd.Args)
	}
}

func TestBento4_BuildCommand_CustomSubtool(t *testing.T) {
	b := NewBento4("bento4:latest")
	input := tools.ToolInput{
		URL:  "http://example.com/video.mp4",
		Args: []string{"mp4dump", "/workspace/input"},
	}

	cmd, err := b.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() error: %v", err)
	}

	if cmd.Binary != "mp4dump" {
		t.Errorf("Binary = %q, want %q", cmd.Binary, "mp4dump")
	}

	if len(cmd.Args) < 1 || cmd.Args[0] != "/workspace/input" {
		t.Errorf("Args = %v, want first arg to be /workspace/input", cmd.Args)
	}
}

func TestBento4_ParseOutput(t *testing.T) {
	raw, err := os.ReadFile("testdata/mp4info_output.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	b := NewBento4("bento4:latest")
	result, err := b.ParseOutput(raw, output.VerbosityStandard)
	if err != nil {
		t.Fatalf("ParseOutput() error: %v", err)
	}

	// Expect 2 streams.
	if len(result.Streams) != 2 {
		t.Fatalf("len(Streams) = %d, want 2", len(result.Streams))
	}

	// Check video stream.
	video := result.Streams[0]
	if video.Width != 1920 || video.Height != 1080 {
		t.Errorf("video stream dimensions = %dx%d, want 1920x1080", video.Width, video.Height)
	}

	// Check audio stream.
	audio := result.Streams[1]
	if audio.Channels != 2 {
		t.Errorf("audio channels = %d, want 2", audio.Channels)
	}

	// Check timing: 120500ms = 120.5s.
	if result.Timing == nil {
		t.Fatal("Timing is nil")
	}
	const wantDuration = 120.5
	if result.Timing.Duration != wantDuration {
		t.Errorf("Timing.Duration = %v, want %v", result.Timing.Duration, wantDuration)
	}
}
