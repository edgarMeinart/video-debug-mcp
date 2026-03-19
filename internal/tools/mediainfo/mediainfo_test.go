package mediainfo_test

import (
	"math"
	"os"
	"strings"
	"testing"

	"github.com/meinart/video-debug-mcp/internal/output"
	tools "github.com/meinart/video-debug-mcp/internal/tools"
	"github.com/meinart/video-debug-mcp/internal/tools/mediainfo"
)

const testImage = "mediaarea/mediainfo:latest"

func newTool() *mediainfo.MediaInfo {
	return mediainfo.NewMediaInfo(testImage)
}

func TestMediaInfo_Name(t *testing.T) {
	m := newTool()
	if got := m.Name(); got != "run_mediainfo" {
		t.Errorf("Name() = %q, want %q", got, "run_mediainfo")
	}
}

func TestMediaInfo_BuildCommand(t *testing.T) {
	m := newTool()
	input := tools.ToolInput{URL: "http://example.com/video.mp4"}
	cmd, err := m.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() returned error: %v", err)
	}

	if cmd.Binary != "mediainfo" {
		t.Errorf("Binary = %q, want %q", cmd.Binary, "mediainfo")
	}

	// Default args must include --Output=JSON.
	hasOutputJSON := false
	for _, arg := range cmd.Args {
		if strings.Contains(arg, "--Output=JSON") {
			hasOutputJSON = true
			break
		}
	}
	if !hasOutputJSON {
		t.Errorf("Args %v does not contain --Output=JSON", cmd.Args)
	}
}

func TestMediaInfo_ParseOutput(t *testing.T) {
	raw, err := os.ReadFile("testdata/mediainfo_output.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	m := newTool()
	result, err := m.ParseOutput(raw, output.VerbosityStandard)
	if err != nil {
		t.Fatalf("ParseOutput() returned error: %v", err)
	}

	// --- Streams: 2 (video + audio; General is skipped) ---
	if len(result.Streams) != 2 {
		t.Fatalf("Streams len = %d, want 2", len(result.Streams))
	}

	video := result.Streams[0]
	if video.Type != "video" {
		t.Errorf("Streams[0].Type = %q, want %q", video.Type, "video")
	}
	if video.Width != 1920 {
		t.Errorf("Streams[0].Width = %d, want 1920", video.Width)
	}
	if video.Height != 1080 {
		t.Errorf("Streams[0].Height = %d, want 1080", video.Height)
	}

	// FrameRate: 29.970
	if math.Abs(video.FrameRate-29.970) > 0.001 {
		t.Errorf("Streams[0].FrameRate = %f, want ~29.970", video.FrameRate)
	}

	audio := result.Streams[1]
	if audio.Type != "audio" {
		t.Errorf("Streams[1].Type = %q, want %q", audio.Type, "audio")
	}
	if audio.Channels != 2 {
		t.Errorf("Streams[1].Channels = %d, want 2", audio.Channels)
	}

	// --- Timing (from General track) ---
	if result.Timing == nil {
		t.Fatal("Timing is nil")
	}
	if math.Abs(result.Timing.Duration-120.5) > 0.001 {
		t.Errorf("Timing.Duration = %f, want 120.5", result.Timing.Duration)
	}

	// --- Bitrate (from General track) ---
	if result.Bitrate == nil {
		t.Fatal("Bitrate is nil")
	}
	if result.Bitrate.Average != 5128000 {
		t.Errorf("Bitrate.Average = %d, want 5128000", result.Bitrate.Average)
	}
}
