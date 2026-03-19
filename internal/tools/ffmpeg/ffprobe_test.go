package ffmpeg_test

import (
	"math"
	"os"
	"testing"

	"github.com/meinart/video-debug-mcp/internal/output"
	tools "github.com/meinart/video-debug-mcp/internal/tools"
	"github.com/meinart/video-debug-mcp/internal/tools/ffmpeg"
)

const testImage = "linuxserver/ffmpeg:latest"

func newTool() *ffmpeg.FFprobe {
	return ffmpeg.NewFFprobe(testImage)
}

func TestFFprobe_Name(t *testing.T) {
	f := newTool()
	if got := f.Name(); got != "run_ffprobe" {
		t.Errorf("Name() = %q, want %q", got, "run_ffprobe")
	}
}

func TestFFprobe_DockerImage(t *testing.T) {
	f := newTool()
	if got := f.DockerImage(); got != testImage {
		t.Errorf("DockerImage() = %q, want %q", got, testImage)
	}
}

func TestFFprobe_BuildCommand_Default(t *testing.T) {
	f := newTool()
	input := tools.ToolInput{URL: "http://example.com/video.mp4"}
	cmd, err := f.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() returned error: %v", err)
	}

	wantArgs := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		"/workspace/input",
	}

	if cmd.Binary != "ffprobe" {
		t.Errorf("Binary = %q, want %q", cmd.Binary, "ffprobe")
	}

	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("Args len = %d, want %d; got %v", len(cmd.Args), len(wantArgs), cmd.Args)
	}
	for i, a := range wantArgs {
		if cmd.Args[i] != a {
			t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], a)
		}
	}
}

func TestFFprobe_BuildCommand_CustomArgs(t *testing.T) {
	f := newTool()
	custom := []string{"-v", "error", "-show_packets", "file.mp4"}
	input := tools.ToolInput{URL: "file.mp4", Args: custom}
	cmd, err := f.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() returned error: %v", err)
	}

	if len(cmd.Args) != len(custom) {
		t.Fatalf("Args len = %d, want %d", len(cmd.Args), len(custom))
	}
	for i, a := range custom {
		if cmd.Args[i] != a {
			t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], a)
		}
	}
}

func TestFFprobe_ParseOutput(t *testing.T) {
	raw, err := os.ReadFile("testdata/ffprobe_output.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	f := newTool()
	result, err := f.ParseOutput(raw, output.VerbosityStandard)
	if err != nil {
		t.Fatalf("ParseOutput() returned error: %v", err)
	}

	// --- Streams ---
	if len(result.Streams) != 2 {
		t.Fatalf("Streams len = %d, want 2", len(result.Streams))
	}

	video := result.Streams[0]
	if video.Type != "video" {
		t.Errorf("Streams[0].Type = %q, want %q", video.Type, "video")
	}
	if video.Codec != "h264" {
		t.Errorf("Streams[0].Codec = %q, want %q", video.Codec, "h264")
	}
	if video.Width != 1920 {
		t.Errorf("Streams[0].Width = %d, want 1920", video.Width)
	}
	if video.Height != 1080 {
		t.Errorf("Streams[0].Height = %d, want 1080", video.Height)
	}

	// Frame rate: 30000/1001 ≈ 29.97
	wantFPS := 30000.0 / 1001.0
	if math.Abs(video.FrameRate-wantFPS) > 0.01 {
		t.Errorf("Streams[0].FrameRate = %f, want ~%f", video.FrameRate, wantFPS)
	}

	audio := result.Streams[1]
	if audio.Type != "audio" {
		t.Errorf("Streams[1].Type = %q, want %q", audio.Type, "audio")
	}
	if audio.Codec != "aac" {
		t.Errorf("Streams[1].Codec = %q, want %q", audio.Codec, "aac")
	}
	if audio.Channels != 2 {
		t.Errorf("Streams[1].Channels = %d, want 2", audio.Channels)
	}

	// --- Timing ---
	if result.Timing == nil {
		t.Fatal("Timing is nil")
	}
	if math.Abs(result.Timing.Duration-120.5) > 0.001 {
		t.Errorf("Timing.Duration = %f, want 120.5", result.Timing.Duration)
	}

	// --- Bitrate ---
	if result.Bitrate == nil {
		t.Fatal("Bitrate is nil")
	}
	if result.Bitrate.Average != 5128000 {
		t.Errorf("Bitrate.Average = %d, want 5128000", result.Bitrate.Average)
	}

	// --- Codec details ---
	if result.Codec == nil {
		t.Fatal("Codec is nil")
	}
	if result.Codec.Profile != "High" {
		t.Errorf("Codec.Profile = %q, want %q", result.Codec.Profile, "High")
	}
	if result.Codec.Level != "4.0" {
		t.Errorf("Codec.Level = %q, want %q", result.Codec.Level, "4.0")
	}
	if result.Codec.PixelFormat != "yuv420p" {
		t.Errorf("Codec.PixelFormat = %q, want %q", result.Codec.PixelFormat, "yuv420p")
	}
}
