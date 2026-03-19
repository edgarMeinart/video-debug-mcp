package mp4

import (
	"os"
	"strings"
	"testing"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

// ---- MP4Box tests ----

func TestMP4Box_Name(t *testing.T) {
	tool := &MP4Box{}
	if got := tool.Name(); got != "run_mp4box" {
		t.Errorf("expected name=run_mp4box, got %q", got)
	}
}

func TestMP4Box_BuildCommand(t *testing.T) {
	tool := &MP4Box{}
	input := tools.ToolInput{URL: "/workspace/input"}
	cmd, err := tool.BuildCommand(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Binary != "MP4Box" {
		t.Errorf("expected binary=MP4Box, got %q", cmd.Binary)
	}
	// Should produce: MP4Box -info /workspace/input
	if len(cmd.Args) < 2 {
		t.Fatalf("expected at least 2 args, got %d: %v", len(cmd.Args), cmd.Args)
	}
	if cmd.Args[0] != "-info" {
		t.Errorf("expected args[0]=-info, got %q", cmd.Args[0])
	}
	if cmd.Args[1] != "/workspace/input" {
		t.Errorf("expected args[1]=/workspace/input, got %q", cmd.Args[1])
	}
}

func TestMP4Box_BuildCommand_RequiresURL(t *testing.T) {
	tool := &MP4Box{}
	_, err := tool.BuildCommand(tools.ToolInput{})
	if err == nil {
		t.Error("expected error when url is empty")
	}
}

func TestMP4Box_ParseOutput(t *testing.T) {
	raw, err := os.ReadFile("testdata/mp4box_info_output.txt")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	tool := &MP4Box{}
	result, err := tool.ParseOutput(raw, output.VerbosityStandard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least 1 stream.
	if len(result.Streams) == 0 {
		t.Fatal("expected at least 1 stream, got 0")
	}

	// Should have timing info.
	if result.Timing == nil {
		t.Fatal("expected timing info, got nil")
	}
	if result.Timing.Duration <= 0 {
		t.Errorf("expected positive duration, got %f", result.Timing.Duration)
	}

	// Verify video stream properties.
	var videoStream *output.StreamInfo
	for i := range result.Streams {
		if result.Streams[i].Type == "video" {
			videoStream = &result.Streams[i]
			break
		}
	}
	if videoStream == nil {
		t.Fatal("expected a video stream")
	}
	if videoStream.Width != 1920 || videoStream.Height != 1080 {
		t.Errorf("expected 1920x1080, got %dx%d", videoStream.Width, videoStream.Height)
	}
	if !strings.Contains(strings.ToLower(videoStream.Codec), "avc") &&
		!strings.Contains(strings.ToLower(videoStream.Codec), "h264") &&
		videoStream.Codec != "avc1" {
		t.Errorf("expected avc/h264 codec, got %q", videoStream.Codec)
	}

	// Bitrate info should be populated.
	if result.Bitrate == nil {
		t.Fatal("expected bitrate info, got nil")
	}
	if result.Bitrate.Average <= 0 {
		t.Errorf("expected positive average bitrate, got %d", result.Bitrate.Average)
	}
}

// ---- MP4Dump tests ----

func TestMP4Dump_Name(t *testing.T) {
	tool := &MP4Dump{}
	if got := tool.Name(); got != "run_mp4dump" {
		t.Errorf("expected name=run_mp4dump, got %q", got)
	}
}

func TestMP4Dump_BuildCommand(t *testing.T) {
	tool := &MP4Dump{}
	input := tools.ToolInput{URL: "/workspace/input"}
	cmd, err := tool.BuildCommand(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Binary != "mp4dump" {
		t.Errorf("expected binary=mp4dump, got %q", cmd.Binary)
	}
}

func TestMP4Dump_BuildCommand_RequiresURL(t *testing.T) {
	tool := &MP4Dump{}
	_, err := tool.BuildCommand(tools.ToolInput{})
	if err == nil {
		t.Error("expected error when url is empty")
	}
}

func TestMP4Dump_ParseOutput(t *testing.T) {
	raw, err := os.ReadFile("testdata/mp4dump_output.txt")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	tool := &MP4Dump{}
	result, err := tool.ParseOutput(raw, output.VerbosityStandard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have a BoxTree with root boxes.
	if len(result.ContainerLayout) == 0 {
		t.Fatal("expected container layout, got empty")
	}

	// Should have ftyp and moov as root boxes.
	roots := result.ContainerLayout
	boxTypes := make(map[string]bool)
	for _, b := range roots {
		boxTypes[b.Type] = true
	}
	if !boxTypes["ftyp"] {
		t.Error("expected ftyp root box")
	}
	if !boxTypes["moov"] {
		t.Error("expected moov root box")
	}

	// moov should have children.
	var moov *output.BoxTree
	for i := range roots {
		if roots[i].Type == "moov" {
			moov = &roots[i]
			break
		}
	}
	if moov == nil {
		t.Fatal("moov box not found")
	}
	if len(moov.Children) == 0 {
		t.Error("expected moov to have children")
	}

	// Verify sizes are parsed.
	for _, b := range roots {
		if b.Size <= 0 {
			t.Errorf("expected positive size for box %q, got %d", b.Type, b.Size)
		}
	}
}

// ---- parse helpers tests ----

func TestParseDurationHHMMSS(t *testing.T) {
	cases := []struct {
		input    string
		expected float64
	}{
		{"00:02:00.500", 120.5},
		{"00:00:01.000", 1.0},
		{"01:00:00.000", 3600.0},
	}
	for _, tc := range cases {
		got := parseDurationHHMMSS(tc.input)
		if got != tc.expected {
			t.Errorf("parseDurationHHMMSS(%q) = %f, want %f", tc.input, got, tc.expected)
		}
	}
}
