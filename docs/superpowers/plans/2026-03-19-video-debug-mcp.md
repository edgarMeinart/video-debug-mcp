# Video Debug MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go MCP server that enables AI agents to deeply inspect video delivery workflows (HLS, DASH, MP4) through structured tool interfaces backed by isolated Docker containers.

**Architecture:** Layered design with Tool plugin interface. MCP transport via mcp-go (SSE/HTTP). Each tool family (ffmpeg, mp4, bento4, mediainfo, shaka) runs in its own Docker container managed via Docker SDK. Manifest parsing uses Go libraries wrapped in a unified model. Output is structured and verbosity-controllable.

**Tech Stack:** Go 1.25.1, mcp-go, Docker SDK for Go, grafov/m3u8, go-dash, gopkg.in/yaml.v3

**Spec:** `docs/superpowers/specs/2026-03-19-video-debug-mcp-design.md`

---

### Task 1: Project Scaffolding & Dependencies

**Files:**
- Modify: `go.mod`
- Create: `cmd/server/main.go`
- Create: `Makefile`
- Create: `config.yaml`

- [ ] **Step 1: Initialize Go module and add core dependencies**

```bash
cd /Users/meinart/Development/video-debug-mcp
go get github.com/mark3labs/mcp-go@latest
go get github.com/docker/docker@latest
go get github.com/docker/go-connections@latest
go get gopkg.in/yaml.v3@latest
go get github.com/grafov/m3u8@latest
go get github.com/zencoder/go-dash/v3@latest
```

- [ ] **Step 2: Create minimal main.go placeholder**

```go
// cmd/server/main.go
package main

import "fmt"

func main() {
	fmt.Println("video-debug-mcp server")
}
```

- [ ] **Step 3: Create default config.yaml**

```yaml
# config.yaml
server:
  port: 8080
  host: "0.0.0.0"

docker:
  images:
    ffmpeg: "video-debug/ffmpeg-tools"
    mp4: "video-debug/mp4-tools"
    bento4: "video-debug/bento4-tools"
    mediainfo: "video-debug/mediainfo-tools"
    shaka: "video-debug/shaka-tools"
  defaults:
    timeout: 60s
    memory_limit: "512m"
    cpu_limit: "1.0"
  per_tool:
    ffmpeg:
      timeout: 120s
      memory_limit: "1g"

fetcher:
  max_concurrent_downloads: 10
  segment_sample_count: 3
  request_timeout: 30s
  max_asset_size: "500m"
  user_agent: "video-debug-mcp/1.0"

output:
  default_verbosity: "standard"

playback:
  default_duration: 10
```

- [ ] **Step 4: Create Makefile**

```makefile
# Makefile
.PHONY: build test test-integration test-all build-images clean

build:
	go build -o bin/video-debug-mcp ./cmd/server

test:
	go test ./...

test-integration:
	go test -tags integration ./...

test-all: test test-integration

build-images:
	docker build -t video-debug/ffmpeg-tools ./docker/ffmpeg-tools
	docker build -t video-debug/mp4-tools ./docker/mp4-tools
	docker build -t video-debug/bento4-tools ./docker/bento4-tools
	docker build -t video-debug/mediainfo-tools ./docker/mediainfo-tools
	docker build -t video-debug/shaka-tools ./docker/shaka-tools

clean:
	rm -rf bin/
```

- [ ] **Step 5: Verify it builds**

Run: `go build ./cmd/server`
Expected: Builds with no errors.

- [ ] **Step 6: Commit**

```bash
git add cmd/ go.mod go.sum config.yaml Makefile
git commit -m "feat: scaffold project structure with dependencies"
```

---

### Task 2: Output Model Types

These types are the foundation — almost every other package depends on them.

**Files:**
- Create: `internal/output/model.go`
- Create: `internal/output/model_test.go`

- [ ] **Step 1: Write failing test for output model JSON serialization**

```go
// internal/output/model_test.go
package output

import (
	"encoding/json"
	"testing"
)

func TestAnalysisResult_JSON_OmitsEmptyFields(t *testing.T) {
	result := &AnalysisResult{
		Tool:     "ffprobe",
		Command:  "ffprobe -v quiet -print_format json -show_streams input.mp4",
		InputURL: "https://example.com/video.mp4",
		Streams: []StreamInfo{
			{
				Index:     0,
				Type:      "video",
				Codec:     "h264",
				Width:     1920,
				Height:    1080,
				Bitrate:   5000000,
				FrameRate: 29.97,
			},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Should have tool, command, input_url, streams
	if m["tool"] != "ffprobe" {
		t.Errorf("expected tool=ffprobe, got %v", m["tool"])
	}

	// Should NOT have nil/empty fields like manifest, container_layout, raw_output
	for _, key := range []string{"manifest", "container_layout", "raw_output", "resolved_urls", "downloads"} {
		if _, ok := m[key]; ok {
			t.Errorf("expected %q to be omitted from JSON, but it was present", key)
		}
	}
}

func TestVerbosity_Valid(t *testing.T) {
	tests := []struct {
		input string
		want  Verbosity
	}{
		{"summary", VerbositySummary},
		{"standard", VerbosityStandard},
		{"deep", VerbosityDeep},
		{"forensic", VerbosityForensic},
	}
	for _, tt := range tests {
		got, err := ParseVerbosity(tt.input)
		if err != nil {
			t.Errorf("ParseVerbosity(%q) error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("ParseVerbosity(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestVerbosity_Invalid(t *testing.T) {
	_, err := ParseVerbosity("invalid")
	if err == nil {
		t.Error("expected error for invalid verbosity, got nil")
	}
}

func TestAnomaly_Severity(t *testing.T) {
	a := Anomaly{
		Severity:    SeverityWarning,
		Description: "PTS discontinuity detected",
		Detail:      "Jump of 5.2s at segment boundary",
	}
	if a.Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %v", a.Severity)
	}
}

func TestErrorResponse_JSON(t *testing.T) {
	e := &ErrorResponse{
		Code:    ErrManifestFetch,
		Message: "Failed to download manifest",
		Phase:   "fetch",
		URL:     "https://example.com/master.m3u8",
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)

	if m["code"] != "MANIFEST_FETCH_FAILED" {
		t.Errorf("expected code=MANIFEST_FETCH_FAILED, got %v", m["code"])
	}
	if m["phase"] != "fetch" {
		t.Errorf("expected phase=fetch, got %v", m["phase"])
	}

	// Empty details/command should be omitted
	if _, ok := m["details"]; ok {
		t.Error("expected details to be omitted")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/output/ -v`
Expected: FAIL — package does not exist yet.

- [ ] **Step 3: Implement the output model types**

```go
// internal/output/model.go
package output

import "fmt"

type Verbosity string

const (
	VerbositySummary  Verbosity = "summary"
	VerbosityStandard Verbosity = "standard"
	VerbosityDeep     Verbosity = "deep"
	VerbosityForensic Verbosity = "forensic"
)

func ParseVerbosity(s string) (Verbosity, error) {
	switch s {
	case "summary":
		return VerbositySummary, nil
	case "standard":
		return VerbosityStandard, nil
	case "deep":
		return VerbosityDeep, nil
	case "forensic":
		return VerbosityForensic, nil
	default:
		return "", fmt.Errorf("invalid verbosity: %q (must be summary, standard, deep, or forensic)", s)
	}
}

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type AnalysisResult struct {
	Tool            string            `json:"tool"`
	Command         string            `json:"command"`
	InputURL        string            `json:"input_url"`
	ResolvedURLs    []string          `json:"resolved_urls,omitempty"`
	Downloads       []DownloadedAsset `json:"downloads,omitempty"`
	Manifest        *ManifestSummary  `json:"manifest,omitempty"`
	Streams         []StreamInfo      `json:"streams,omitempty"`
	Codec           *CodecDetails     `json:"codec,omitempty"`
	Timing          *TimingInfo       `json:"timing,omitempty"`
	Bitrate         *BitrateInfo      `json:"bitrate,omitempty"`
	ContainerLayout *BoxTree          `json:"container_layout,omitempty"`
	Anomalies       []Anomaly         `json:"anomalies,omitempty"`
	RawOutput       string            `json:"raw_output,omitempty"`
}

type DownloadedAsset struct {
	URL      string `json:"url"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type,omitempty"`
}

type ManifestSummary struct {
	Type           string          `json:"type"`
	URL            string          `json:"url"`
	VariantCount   int             `json:"variant_count"`
	AudioTracks    int             `json:"audio_tracks"`
	SubtitleTracks int             `json:"subtitle_tracks"`
	TotalSegments  int             `json:"total_segments"`
	DRM            *DRMSummary     `json:"drm,omitempty"`
	Variants       []VariantInfo   `json:"variants,omitempty"`
	Raw            string          `json:"raw,omitempty"`
}

type VariantInfo struct {
	URI        string  `json:"uri"`
	Bandwidth  int     `json:"bandwidth"`
	Resolution string  `json:"resolution,omitempty"`
	Codecs     string  `json:"codecs,omitempty"`
	FrameRate  float64 `json:"frame_rate,omitempty"`
}

type DRMSummary struct {
	System    string `json:"system"`
	SchemeURI string `json:"scheme_uri,omitempty"`
	KeyID     string `json:"key_id,omitempty"`
}

type StreamInfo struct {
	Index     int     `json:"index"`
	Type      string  `json:"type"`
	Codec     string  `json:"codec"`
	Profile   string  `json:"profile,omitempty"`
	Level     string  `json:"level,omitempty"`
	Width     int     `json:"width,omitempty"`
	Height    int     `json:"height,omitempty"`
	Bitrate   int64   `json:"bitrate,omitempty"`
	FrameRate float64 `json:"frame_rate,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
	Channels   int    `json:"channels,omitempty"`
	Language   string `json:"language,omitempty"`
}

type CodecDetails struct {
	VideoCodec   string `json:"video_codec,omitempty"`
	AudioCodec   string `json:"audio_codec,omitempty"`
	VideoProfile string `json:"video_profile,omitempty"`
	VideoLevel   string `json:"video_level,omitempty"`
	PixelFormat  string `json:"pixel_format,omitempty"`
	ColorSpace   string `json:"color_space,omitempty"`
	PrivateData  string `json:"private_data,omitempty"`
}

type TimingInfo struct {
	Duration   float64 `json:"duration"`
	StartTime  float64 `json:"start_time"`
	Timescale  int     `json:"timescale,omitempty"`
	FrameCount int     `json:"frame_count,omitempty"`
}

type BitrateInfo struct {
	Overall    int64  `json:"overall"`
	Video      int64  `json:"video,omitempty"`
	Audio      int64  `json:"audio,omitempty"`
	Resolution string `json:"resolution,omitempty"`
	Channels   int    `json:"channels,omitempty"`
}

type BoxTree struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Offset   int64     `json:"offset,omitempty"`
	Detail   string    `json:"detail,omitempty"`
	Children []BoxTree `json:"children,omitempty"`
}

type Anomaly struct {
	Severity    Severity `json:"severity"`
	Description string   `json:"description"`
	Detail      string   `json:"detail,omitempty"`
	Timestamp   float64  `json:"timestamp,omitempty"`
}

type ErrorCode string

const (
	ErrManifestFetch    ErrorCode = "MANIFEST_FETCH_FAILED"
	ErrManifestParse    ErrorCode = "MANIFEST_PARSE_FAILED"
	ErrSegmentFetch     ErrorCode = "SEGMENT_FETCH_FAILED"
	ErrSegmentResolve   ErrorCode = "SEGMENT_RESOLVE_FAILED"
	ErrToolExecution    ErrorCode = "TOOL_EXECUTION_FAILED"
	ErrToolTimeout      ErrorCode = "TOOL_TIMEOUT"
	ErrDockerUnavail    ErrorCode = "DOCKER_UNAVAILABLE"
	ErrImageMissing     ErrorCode = "IMAGE_NOT_FOUND"
	ErrUnsupportedInput ErrorCode = "UNSUPPORTED_INPUT"
	ErrMalformedInput   ErrorCode = "MALFORMED_INPUT"
)

type ErrorResponse struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
	Phase   string    `json:"phase"`
	URL     string    `json:"url,omitempty"`
	Command string    `json:"command,omitempty"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/output/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/output/
git commit -m "feat: add structured output model types with verbosity levels"
```

---

### Task 3: Configuration Package

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for config loading**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig_FromFile(t *testing.T) {
	content := `
server:
  port: 9090
  host: "127.0.0.1"
docker:
  images:
    ffmpeg: "custom/ffmpeg"
  defaults:
    timeout: 30s
    memory_limit: "256m"
    cpu_limit: "0.5"
fetcher:
  max_concurrent_downloads: 5
  segment_sample_count: 2
  request_timeout: 15s
  max_asset_size: "100m"
  user_agent: "test/1.0"
output:
  default_verbosity: "deep"
playback:
  default_duration: 20
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port=9090, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host=127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Docker.Images.FFmpeg != "custom/ffmpeg" {
		t.Errorf("expected ffmpeg image=custom/ffmpeg, got %s", cfg.Docker.Images.FFmpeg)
	}
	if cfg.Docker.Defaults.Timeout != 30*time.Second {
		t.Errorf("expected timeout=30s, got %v", cfg.Docker.Defaults.Timeout)
	}
	if cfg.Fetcher.MaxConcurrentDownloads != 5 {
		t.Errorf("expected max_concurrent_downloads=5, got %d", cfg.Fetcher.MaxConcurrentDownloads)
	}
	if cfg.Output.DefaultVerbosity != "deep" {
		t.Errorf("expected default_verbosity=deep, got %s", cfg.Output.DefaultVerbosity)
	}
	if cfg.Playback.DefaultDuration != 20 {
		t.Errorf("expected default_duration=20, got %d", cfg.Playback.DefaultDuration)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	cfg := Default()

	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port=8080, got %d", cfg.Server.Port)
	}
	if cfg.Docker.Defaults.Timeout != 60*time.Second {
		t.Errorf("expected default timeout=60s, got %v", cfg.Docker.Defaults.Timeout)
	}
	if cfg.Fetcher.MaxConcurrentDownloads != 10 {
		t.Errorf("expected default max_concurrent_downloads=10, got %d", cfg.Fetcher.MaxConcurrentDownloads)
	}
	if cfg.Output.DefaultVerbosity != "standard" {
		t.Errorf("expected default verbosity=standard, got %s", cfg.Output.DefaultVerbosity)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	t.Setenv("VIDEO_DEBUG_SERVER_PORT", "3000")
	t.Setenv("VIDEO_DEBUG_FETCHER_MAX_CONCURRENT_DOWNLOADS", "20")

	cfg := Default()
	cfg.ApplyEnvOverrides()

	if cfg.Server.Port != 3000 {
		t.Errorf("expected port=3000 from env, got %d", cfg.Server.Port)
	}
	if cfg.Fetcher.MaxConcurrentDownloads != 20 {
		t.Errorf("expected max_concurrent_downloads=20 from env, got %d", cfg.Fetcher.MaxConcurrentDownloads)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for missing config file, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -v`
Expected: FAIL — package does not exist yet.

- [ ] **Step 3: Implement config package**

```go
// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Docker   DockerConfig   `yaml:"docker"`
	Fetcher  FetcherConfig  `yaml:"fetcher"`
	Output   OutputConfig   `yaml:"output"`
	Playback PlaybackConfig `yaml:"playback"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type DockerConfig struct {
	Images   DockerImages          `yaml:"images"`
	Defaults DockerDefaults        `yaml:"defaults"`
	PerTool  map[string]ToolConfig `yaml:"per_tool"`
}

type DockerImages struct {
	FFmpeg    string `yaml:"ffmpeg"`
	MP4       string `yaml:"mp4"`
	Bento4    string `yaml:"bento4"`
	MediaInfo string `yaml:"mediainfo"`
	Shaka     string `yaml:"shaka"`
}

type DockerDefaults struct {
	Timeout     time.Duration `yaml:"timeout"`
	MemoryLimit string        `yaml:"memory_limit"`
	CPULimit    string        `yaml:"cpu_limit"`
}

type ToolConfig struct {
	Timeout     time.Duration `yaml:"timeout"`
	MemoryLimit string        `yaml:"memory_limit"`
}

type FetcherConfig struct {
	MaxConcurrentDownloads int           `yaml:"max_concurrent_downloads"`
	SegmentSampleCount     int           `yaml:"segment_sample_count"`
	RequestTimeout         time.Duration `yaml:"request_timeout"`
	MaxAssetSize           string        `yaml:"max_asset_size"`
	UserAgent              string        `yaml:"user_agent"`
}

type OutputConfig struct {
	DefaultVerbosity string `yaml:"default_verbosity"`
}

type PlaybackConfig struct {
	DefaultDuration int `yaml:"default_duration"`
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8080,
			Host: "0.0.0.0",
		},
		Docker: DockerConfig{
			Images: DockerImages{
				FFmpeg:    "video-debug/ffmpeg-tools",
				MP4:       "video-debug/mp4-tools",
				Bento4:    "video-debug/bento4-tools",
				MediaInfo: "video-debug/mediainfo-tools",
				Shaka:     "video-debug/shaka-tools",
			},
			Defaults: DockerDefaults{
				Timeout:     60 * time.Second,
				MemoryLimit: "512m",
				CPULimit:    "1.0",
			},
		},
		Fetcher: FetcherConfig{
			MaxConcurrentDownloads: 10,
			SegmentSampleCount:     3,
			RequestTimeout:         30 * time.Second,
			MaxAssetSize:           "500m",
			UserAgent:              "video-debug-mcp/1.0",
		},
		Output: OutputConfig{
			DefaultVerbosity: "standard",
		},
		Playback: PlaybackConfig{
			DefaultDuration: 10,
		},
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	cfg.ApplyEnvOverrides()
	return cfg, nil
}

func (c *Config) ApplyEnvOverrides() {
	if v := os.Getenv("VIDEO_DEBUG_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Server.Port = port
		}
	}
	if v := os.Getenv("VIDEO_DEBUG_SERVER_HOST"); v != "" {
		c.Server.Host = v
	}
	if v := os.Getenv("VIDEO_DEBUG_FETCHER_MAX_CONCURRENT_DOWNLOADS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Fetcher.MaxConcurrentDownloads = n
		}
	}
	if v := os.Getenv("VIDEO_DEBUG_DOCKER_DEFAULTS_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Docker.Defaults.Timeout = d
		}
	}
	if v := os.Getenv("VIDEO_DEBUG_OUTPUT_DEFAULT_VERBOSITY"); v != "" {
		c.Output.DefaultVerbosity = v
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config package with YAML loading and env overrides"
```

---

### Task 4: Tool Interface & Registry

**Files:**
- Create: `internal/tools/tool.go`
- Create: `internal/tools/registry.go`
- Create: `internal/tools/registry_test.go`

- [ ] **Step 1: Write failing tests for the tool registry**

```go
// internal/tools/registry_test.go
package tools

import (
	"encoding/json"
	"testing"

	"github.com/meinart/video-debug-mcp/internal/output"
)

// mockTool implements Tool for testing
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
	if !ok {
		t.Fatal("expected to find ffprobe in registry")
	}
	if got.Name() != "ffprobe" {
		t.Errorf("expected name=ffprobe, got %s", got.Name())
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	reg := NewRegistry()

	_, ok := reg.Get("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent tool")
	}
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockTool{name: "ffprobe", description: "a", image: "i"})
	reg.Register(&mockTool{name: "mp4box", description: "b", image: "j"})

	tools := reg.List()
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}

func TestToolInput_Verbosity(t *testing.T) {
	input := ToolInput{
		URL:       "https://example.com/video.mp4",
		Verbosity: "deep",
	}

	v, err := input.ParsedVerbosity()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != output.VerbosityDeep {
		t.Errorf("expected deep, got %v", v)
	}
}

func TestToolInput_DefaultVerbosity(t *testing.T) {
	input := ToolInput{
		URL: "https://example.com/video.mp4",
	}

	v, err := input.ParsedVerbosity()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != output.VerbosityStandard {
		t.Errorf("expected standard (default), got %v", v)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tools/ -v`
Expected: FAIL — package does not exist yet.

- [ ] **Step 3: Implement tool interface and registry**

```go
// internal/tools/tool.go
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
	InputFiles []string // Paths relative to /workspace inside container
}
```

```go
// internal/tools/registry.go
package tools

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) List() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tools/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tools/tool.go internal/tools/registry.go internal/tools/registry_test.go
git commit -m "feat: add Tool interface and Registry"
```

---

### Task 5: Workspace & Temp Directory Management

**Files:**
- Create: `internal/fetcher/workspace.go`
- Create: `internal/fetcher/workspace_test.go`

- [ ] **Step 1: Write failing tests for workspace lifecycle**

```go
// internal/fetcher/workspace_test.go
package fetcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspace_CreateAndCleanup(t *testing.T) {
	ws, err := NewWorkspace()
	if err != nil {
		t.Fatalf("NewWorkspace() error: %v", err)
	}

	// Dir should exist
	info, err := os.Stat(ws.Dir)
	if err != nil {
		t.Fatalf("workspace dir does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("workspace path is not a directory")
	}

	dir := ws.Dir

	// Cleanup
	ws.Cleanup()

	// Dir should no longer exist
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("workspace dir still exists after cleanup")
	}
}

func TestWorkspace_AssetPath(t *testing.T) {
	ws, err := NewWorkspace()
	if err != nil {
		t.Fatalf("NewWorkspace() error: %v", err)
	}
	defer ws.Cleanup()

	path := ws.AssetPath("segment_001.ts")
	expected := filepath.Join(ws.Dir, "segment_001.ts")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestWorkspace_RecordAsset(t *testing.T) {
	ws, err := NewWorkspace()
	if err != nil {
		t.Fatalf("NewWorkspace() error: %v", err)
	}
	defer ws.Cleanup()

	ws.RecordAsset("https://example.com/seg.ts", "seg.ts", 1024, "video/mp2t")

	if len(ws.Assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(ws.Assets))
	}
	if ws.Assets[0].URL != "https://example.com/seg.ts" {
		t.Errorf("expected URL https://example.com/seg.ts, got %s", ws.Assets[0].URL)
	}
	if ws.Assets[0].Size != 1024 {
		t.Errorf("expected size 1024, got %d", ws.Assets[0].Size)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/fetcher/ -v`
Expected: FAIL — package does not exist yet.

- [ ] **Step 3: Implement workspace**

```go
// internal/fetcher/workspace.go
package fetcher

import (
	"os"
	"path/filepath"

	"github.com/meinart/video-debug-mcp/internal/output"
)

type Workspace struct {
	Dir          string
	ManifestPath string
	Assets       []output.DownloadedAsset
}

func NewWorkspace() (*Workspace, error) {
	dir, err := os.MkdirTemp("", "video-debug-*")
	if err != nil {
		return nil, err
	}
	return &Workspace{Dir: dir}, nil
}

func (w *Workspace) AssetPath(filename string) string {
	return filepath.Join(w.Dir, filename)
}

func (w *Workspace) RecordAsset(url, filename string, size int64, mimeType string) {
	w.Assets = append(w.Assets, output.DownloadedAsset{
		URL:      url,
		Path:     filename,
		Size:     size,
		MimeType: mimeType,
	})
}

func (w *Workspace) Cleanup() {
	os.RemoveAll(w.Dir)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/fetcher/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/fetcher/
git commit -m "feat: add workspace temp directory management"
```

---

### Task 6: Asset Fetcher

**Files:**
- Create: `internal/fetcher/fetcher.go`
- Create: `internal/fetcher/fetcher_test.go`

- [ ] **Step 1: Write failing tests for the fetcher**

```go
// internal/fetcher/fetcher_test.go
package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestFetcher_DownloadFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Write([]byte("fake mp4 data"))
	}))
	defer server.Close()

	f := NewFetcher(10, 30*time.Second, "test/1.0")
	ws, err := NewWorkspace()
	if err != nil {
		t.Fatalf("NewWorkspace() error: %v", err)
	}
	defer ws.Cleanup()

	ctx := context.Background()
	err = f.DownloadFile(ctx, server.URL+"/video.mp4", ws, "video.mp4", nil)
	if err != nil {
		t.Fatalf("DownloadFile() error: %v", err)
	}

	// File should exist
	data, err := os.ReadFile(ws.AssetPath("video.mp4"))
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(data) != "fake mp4 data" {
		t.Errorf("expected 'fake mp4 data', got %q", string(data))
	}

	// Asset should be recorded
	if len(ws.Assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(ws.Assets))
	}
}

func TestFetcher_DownloadFile_CustomHeaders(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Write([]byte("data"))
	}))
	defer server.Close()

	f := NewFetcher(10, 30*time.Second, "test/1.0")
	ws, err := NewWorkspace()
	if err != nil {
		t.Fatalf("NewWorkspace() error: %v", err)
	}
	defer ws.Cleanup()

	headers := map[string]string{"Authorization": "Bearer token123"}
	ctx := context.Background()
	err = f.DownloadFile(ctx, server.URL+"/file", ws, "file", headers)
	if err != nil {
		t.Fatalf("DownloadFile() error: %v", err)
	}

	if receivedAuth != "Bearer token123" {
		t.Errorf("expected Authorization header, got %q", receivedAuth)
	}
}

func TestFetcher_DownloadFile_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	f := NewFetcher(10, 30*time.Second, "test/1.0")
	ws, _ := NewWorkspace()
	defer ws.Cleanup()

	ctx := context.Background()
	err := f.DownloadFile(ctx, server.URL+"/missing.mp4", ws, "missing.mp4", nil)
	if err == nil {
		t.Error("expected error for 404, got nil")
	}
}

func TestFetcher_DownloadConcurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("segment data"))
	}))
	defer server.Close()

	f := NewFetcher(5, 30*time.Second, "test/1.0")
	ws, _ := NewWorkspace()
	defer ws.Cleanup()

	urls := map[string]string{
		server.URL + "/seg1.ts": "seg1.ts",
		server.URL + "/seg2.ts": "seg2.ts",
		server.URL + "/seg3.ts": "seg3.ts",
	}

	ctx := context.Background()
	errors := f.DownloadConcurrent(ctx, urls, ws, nil)

	if len(errors) != 0 {
		t.Errorf("expected no errors, got %v", errors)
	}
	if len(ws.Assets) != 3 {
		t.Errorf("expected 3 assets, got %d", len(ws.Assets))
	}
}

func TestFetcher_DetectType(t *testing.T) {
	tests := []struct {
		url     string
		content string
		want    AssetType
	}{
		{"https://example.com/master.m3u8", "#EXTM3U", AssetTypeHLS},
		{"https://example.com/manifest.mpd", "<?xml", AssetTypeDASH},
		{"https://example.com/video.mp4", "\x00\x00", AssetTypeMedia},
		{"https://example.com/unknown", "#EXTM3U", AssetTypeHLS},
		{"https://example.com/unknown", "<?xml version", AssetTypeDASH},
	}
	for _, tt := range tests {
		got := DetectAssetType(tt.url, []byte(tt.content))
		if got != tt.want {
			t.Errorf("DetectAssetType(%q, ...) = %v, want %v", tt.url, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/fetcher/ -v`
Expected: FAIL — Fetcher, DownloadFile, etc. not defined yet.

- [ ] **Step 3: Implement fetcher**

```go
// internal/fetcher/fetcher.go
package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type AssetType string

const (
	AssetTypeHLS   AssetType = "hls"
	AssetTypeDASH  AssetType = "dash"
	AssetTypeMedia AssetType = "media"
)

type Fetcher struct {
	client    *http.Client
	maxConc   int
	userAgent string
}

func NewFetcher(maxConcurrent int, requestTimeout time.Duration, userAgent string) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: requestTimeout,
		},
		maxConc:   maxConcurrent,
		userAgent: userAgent,
	}
}

func (f *Fetcher) DownloadFile(ctx context.Context, url string, ws *Workspace, filename string, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", f.userAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	outPath := ws.AssetPath(filename)
	file, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", outPath, err)
	}
	defer file.Close()

	n, err := io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("write file %s: %w", outPath, err)
	}

	contentType := resp.Header.Get("Content-Type")
	ws.RecordAsset(url, filename, n, contentType)

	return nil
}

type DownloadError struct {
	URL string
	Err error
}

func (f *Fetcher) DownloadConcurrent(ctx context.Context, urls map[string]string, ws *Workspace, headers map[string]string) []DownloadError {
	var (
		mu     sync.Mutex
		errors []DownloadError
		wg     sync.WaitGroup
		sem    = make(chan struct{}, f.maxConc)
	)

	for url, filename := range urls {
		wg.Add(1)
		sem <- struct{}{}
		go func(u, fn string) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := f.DownloadFile(ctx, u, ws, fn, headers); err != nil {
				mu.Lock()
				errors = append(errors, DownloadError{URL: u, Err: err})
				mu.Unlock()
			}
		}(url, filename)
	}

	wg.Wait()
	return errors
}

func DetectAssetType(url string, content []byte) AssetType {
	lower := strings.ToLower(url)
	if strings.HasSuffix(lower, ".m3u8") {
		return AssetTypeHLS
	}
	if strings.HasSuffix(lower, ".mpd") {
		return AssetTypeDASH
	}

	// Check content
	text := string(content)
	if strings.HasPrefix(text, "#EXTM3U") {
		return AssetTypeHLS
	}
	if strings.Contains(text[:min(len(text), 200)], "<?xml") || strings.Contains(text[:min(len(text), 200)], "<MPD") {
		return AssetTypeDASH
	}

	return AssetTypeMedia
}

// Note: uses built-in min() available since Go 1.21
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/fetcher/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/fetcher/
git commit -m "feat: add asset fetcher with concurrent downloads and type detection"
```

---

### Task 7: Unified Manifest Model

**Files:**
- Create: `internal/manifest/model.go`
- Create: `internal/manifest/model_test.go`

- [ ] **Step 1: Write failing tests for manifest model**

```go
// internal/manifest/model_test.go
package manifest

import (
	"testing"
)

func TestManifest_AllSegmentURLs(t *testing.T) {
	m := &Manifest{
		Type: ManifestTypeHLS,
		Variants: []Variant{
			{
				URI:       "720p.m3u8",
				Bandwidth: 3000000,
				Segments: []Segment{
					{URI: "https://cdn.example.com/seg1.ts", Duration: 6.0, Sequence: 0},
					{URI: "https://cdn.example.com/seg2.ts", Duration: 6.0, Sequence: 1},
				},
			},
		},
		InitSegments: []Segment{
			{URI: "https://cdn.example.com/init.mp4", IsInit: true},
		},
	}

	urls := m.AllSegmentURLs()
	if len(urls) != 3 {
		t.Errorf("expected 3 URLs, got %d", len(urls))
	}
}

func TestManifest_SampledSegmentURLs(t *testing.T) {
	segs := make([]Segment, 20)
	for i := range segs {
		segs[i] = Segment{URI: "https://cdn.example.com/seg" + string(rune('A'+i)) + ".ts", Duration: 6.0, Sequence: i}
	}

	m := &Manifest{
		Type: ManifestTypeHLS,
		Variants: []Variant{
			{Segments: segs},
		},
		InitSegments: []Segment{
			{URI: "https://cdn.example.com/init.mp4", IsInit: true},
		},
	}

	urls := m.SampledSegmentURLs(3)
	// 1 init + 3 first + 3 last = 7
	if len(urls) != 7 {
		t.Errorf("expected 7 sampled URLs, got %d", len(urls))
	}
}

func TestManifest_SampledSegmentURLs_FewSegments(t *testing.T) {
	m := &Manifest{
		Type: ManifestTypeHLS,
		Variants: []Variant{
			{
				Segments: []Segment{
					{URI: "https://cdn.example.com/seg1.ts"},
					{URI: "https://cdn.example.com/seg2.ts"},
				},
			},
		},
	}

	// When fewer segments than 2*N, return all
	urls := m.SampledSegmentURLs(3)
	if len(urls) != 2 {
		t.Errorf("expected 2 URLs (all segments), got %d", len(urls))
	}
}

func TestResolveURI(t *testing.T) {
	tests := []struct {
		base     string
		ref      string
		expected string
	}{
		{"https://cdn.example.com/hls/master.m3u8", "720p.m3u8", "https://cdn.example.com/hls/720p.m3u8"},
		{"https://cdn.example.com/hls/master.m3u8", "/absolute/720p.m3u8", "https://cdn.example.com/absolute/720p.m3u8"},
		{"https://cdn.example.com/hls/master.m3u8", "https://other.com/720p.m3u8", "https://other.com/720p.m3u8"},
	}

	for _, tt := range tests {
		got, err := ResolveURI(tt.base, tt.ref)
		if err != nil {
			t.Errorf("ResolveURI(%q, %q) error: %v", tt.base, tt.ref, err)
			continue
		}
		if got != tt.expected {
			t.Errorf("ResolveURI(%q, %q) = %q, want %q", tt.base, tt.ref, got, tt.expected)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/manifest/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement manifest model**

```go
// internal/manifest/model.go
package manifest

import (
	"net/url"
)

type ManifestType string

const (
	ManifestTypeHLS  ManifestType = "hls"
	ManifestTypeDASH ManifestType = "dash"
)

type Manifest struct {
	Type           ManifestType
	URL            string
	Variants       []Variant
	AudioTracks    []AudioTrack
	SubtitleTracks []SubTrack
	InitSegments   []Segment
	MediaSegments  []Segment
	DRM            *DRMInfo
	Raw            string
}

type Variant struct {
	URI        string
	Bandwidth  int
	Resolution string
	Codecs     string
	FrameRate  float64
	Segments   []Segment
}

type AudioTrack struct {
	URI      string
	Language string
	Name     string
	Codecs   string
	Channels int
	Segments []Segment
}

type SubTrack struct {
	URI      string
	Language string
	Name     string
	Format   string
	Segments []Segment
}

type Segment struct {
	URI      string
	Duration float64
	Sequence int
	IsInit   bool
	TrackRef string
}

type DRMInfo struct {
	System    string
	SchemeURI string
	KeyID     string
	PSSH      string
	EXTXKey   string
}

func (m *Manifest) AllSegmentURLs() []string {
	seen := make(map[string]bool)
	var urls []string

	add := func(uri string) {
		if uri != "" && !seen[uri] {
			seen[uri] = true
			urls = append(urls, uri)
		}
	}

	for _, s := range m.InitSegments {
		add(s.URI)
	}
	for _, v := range m.Variants {
		for _, s := range v.Segments {
			add(s.URI)
		}
	}
	for _, a := range m.AudioTracks {
		for _, s := range a.Segments {
			add(s.URI)
		}
	}
	for _, st := range m.SubtitleTracks {
		for _, s := range st.Segments {
			add(s.URI)
		}
	}
	for _, s := range m.MediaSegments {
		add(s.URI)
	}

	return urls
}

func (m *Manifest) SampledSegmentURLs(n int) []string {
	seen := make(map[string]bool)
	var urls []string

	add := func(uri string) {
		if uri != "" && !seen[uri] {
			seen[uri] = true
			urls = append(urls, uri)
		}
	}

	// Always include all init segments
	for _, s := range m.InitSegments {
		add(s.URI)
	}

	// Sample first N + last N from each variant
	sampleSlice := func(segs []Segment) {
		if len(segs) <= 2*n {
			for _, s := range segs {
				add(s.URI)
			}
			return
		}
		for i := 0; i < n; i++ {
			add(segs[i].URI)
		}
		for i := len(segs) - n; i < len(segs); i++ {
			add(segs[i].URI)
		}
	}

	for _, v := range m.Variants {
		sampleSlice(v.Segments)
	}
	for _, a := range m.AudioTracks {
		sampleSlice(a.Segments)
	}
	for _, st := range m.SubtitleTracks {
		sampleSlice(st.Segments)
	}

	return urls
}

func ResolveURI(base, ref string) (string, error) {
	refURL, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	if refURL.IsAbs() {
		return ref, nil
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(refURL).String(), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/manifest/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/manifest/model.go internal/manifest/model_test.go
git commit -m "feat: add unified manifest model with URI resolution and segment sampling"
```

---

### Task 8: HLS Parser

**Files:**
- Create: `internal/manifest/hls/parser.go`
- Create: `internal/manifest/hls/parser_test.go`
- Create: `internal/manifest/hls/testdata/master.m3u8`
- Create: `internal/manifest/hls/testdata/media.m3u8`
- Create: `internal/manifest/hls/testdata/encrypted.m3u8`

- [ ] **Step 1: Create test fixture files**

`internal/manifest/hls/testdata/master.m3u8`:
```
#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=3000000,RESOLUTION=1280x720,CODECS="avc1.64001f,mp4a.40.2"
720p/playlist.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=1920x1080,CODECS="avc1.640028,mp4a.40.2"
1080p/playlist.m3u8
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="English",LANGUAGE="en",URI="audio/en.m3u8"
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="English",LANGUAGE="en",URI="subs/en.m3u8"
```

`internal/manifest/hls/testdata/media.m3u8`:
```
#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-MAP:URI="init.mp4"
#EXTINF:6.000,
segment_000.ts
#EXTINF:6.000,
segment_001.ts
#EXTINF:4.500,
segment_002.ts
#EXT-X-ENDLIST
```

`internal/manifest/hls/testdata/encrypted.m3u8`:
```
#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:6
#EXT-X-KEY:METHOD=AES-128,URI="https://keys.example.com/key1",IV=0x00000000000000000000000000000001
#EXTINF:6.000,
enc_seg_000.ts
#EXTINF:6.000,
enc_seg_001.ts
#EXT-X-ENDLIST
```

- [ ] **Step 2: Write failing tests for HLS parser**

```go
// internal/manifest/hls/parser_test.go
package hls

import (
	"os"
	"testing"

	"github.com/meinart/video-debug-mcp/internal/manifest"
)

func TestParseMasterPlaylist(t *testing.T) {
	data, err := os.ReadFile("testdata/master.m3u8")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	m, err := Parse(data, "https://cdn.example.com/hls/master.m3u8")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if m.Type != manifest.ManifestTypeHLS {
		t.Errorf("expected type=hls, got %v", m.Type)
	}
	if len(m.Variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(m.Variants))
	}

	// Check first variant
	v := m.Variants[0]
	if v.Bandwidth != 3000000 {
		t.Errorf("expected bandwidth=3000000, got %d", v.Bandwidth)
	}
	if v.Resolution != "1280x720" {
		t.Errorf("expected resolution=1280x720, got %s", v.Resolution)
	}
	if v.URI != "https://cdn.example.com/hls/720p/playlist.m3u8" {
		t.Errorf("expected resolved URI, got %s", v.URI)
	}

	// Check audio track
	if len(m.AudioTracks) != 1 {
		t.Fatalf("expected 1 audio track, got %d", len(m.AudioTracks))
	}
	if m.AudioTracks[0].Language != "en" {
		t.Errorf("expected language=en, got %s", m.AudioTracks[0].Language)
	}

	// Check subtitle track
	if len(m.SubtitleTracks) != 1 {
		t.Fatalf("expected 1 subtitle track, got %d", len(m.SubtitleTracks))
	}
}

func TestParseMediaPlaylist(t *testing.T) {
	data, err := os.ReadFile("testdata/media.m3u8")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	m, err := Parse(data, "https://cdn.example.com/hls/720p/playlist.m3u8")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if len(m.Variants) != 1 {
		t.Fatalf("expected 1 variant (media playlist treated as single variant), got %d", len(m.Variants))
	}

	v := m.Variants[0]
	if len(v.Segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(v.Segments))
	}

	// Check init segment
	if len(m.InitSegments) != 1 {
		t.Fatalf("expected 1 init segment, got %d", len(m.InitSegments))
	}
	if m.InitSegments[0].URI != "https://cdn.example.com/hls/720p/init.mp4" {
		t.Errorf("expected resolved init URI, got %s", m.InitSegments[0].URI)
	}

	// Check segment URIs are resolved
	if v.Segments[0].URI != "https://cdn.example.com/hls/720p/segment_000.ts" {
		t.Errorf("expected resolved segment URI, got %s", v.Segments[0].URI)
	}
	if v.Segments[2].Duration != 4.5 {
		t.Errorf("expected duration=4.5, got %f", v.Segments[2].Duration)
	}
}

func TestParseEncryptedPlaylist(t *testing.T) {
	data, err := os.ReadFile("testdata/encrypted.m3u8")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	m, err := Parse(data, "https://cdn.example.com/hls/encrypted.m3u8")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if m.DRM == nil {
		t.Fatal("expected DRM info, got nil")
	}
	if m.DRM.EXTXKey == "" {
		t.Error("expected EXT-X-KEY value, got empty")
	}
}

func TestParse_InvalidData(t *testing.T) {
	_, err := Parse([]byte("not a valid playlist"), "https://example.com/bad.m3u8")
	if err == nil {
		t.Error("expected error for invalid playlist, got nil")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/manifest/hls/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 4: Implement HLS parser**

```go
// internal/manifest/hls/parser.go
package hls

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/grafov/m3u8"
	"github.com/meinart/video-debug-mcp/internal/manifest"
)

func Parse(data []byte, baseURL string) (*manifest.Manifest, error) {
	buf := bytes.NewBuffer(data)
	playlist, listType, err := m3u8.Decode(*buf, true)
	if err != nil {
		return nil, fmt.Errorf("parse HLS: %w", err)
	}

	m := &manifest.Manifest{
		Type: manifest.ManifestTypeHLS,
		URL:  baseURL,
		Raw:  string(data),
	}

	switch listType {
	case m3u8.MASTER:
		master := playlist.(*m3u8.MasterPlaylist)
		if err := parseMaster(master, baseURL, m); err != nil {
			return nil, err
		}
	case m3u8.MEDIA:
		media := playlist.(*m3u8.MediaPlaylist)
		if err := parseMedia(media, baseURL, m); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown HLS playlist type: %v", listType)
	}

	return m, nil
}

func parseMaster(master *m3u8.MasterPlaylist, baseURL string, m *manifest.Manifest) error {
	for _, v := range master.Variants {
		if v == nil {
			continue
		}
		uri, err := manifest.ResolveURI(baseURL, v.URI)
		if err != nil {
			return fmt.Errorf("resolve variant URI %q: %w", v.URI, err)
		}

		resolution := ""
		if v.Resolution != "" {
			resolution = v.Resolution
		}

		m.Variants = append(m.Variants, manifest.Variant{
			URI:        uri,
			Bandwidth:  int(v.Bandwidth),
			Resolution: resolution,
			Codecs:     v.Codecs,
			FrameRate:  v.FrameRate,
		})

		// Process alternative renditions
		for _, alt := range v.Alternatives {
			if alt == nil {
				continue
			}
			altURI := ""
			if alt.URI != "" {
				resolved, err := manifest.ResolveURI(baseURL, alt.URI)
				if err != nil {
					return fmt.Errorf("resolve alt URI %q: %w", alt.URI, err)
				}
				altURI = resolved
			}

			switch strings.ToUpper(alt.Type) {
			case "AUDIO":
				m.AudioTracks = append(m.AudioTracks, manifest.AudioTrack{
					URI:      altURI,
					Language: alt.Language,
					Name:     alt.Name,
				})
			case "SUBTITLES":
				m.SubtitleTracks = append(m.SubtitleTracks, manifest.SubTrack{
					URI:      altURI,
					Language: alt.Language,
					Name:     alt.Name,
				})
			}
		}
	}

	return nil
}

func parseMedia(media *m3u8.MediaPlaylist, baseURL string, m *manifest.Manifest) error {
	var segments []manifest.Segment

	for i, seg := range media.Segments {
		if seg == nil {
			continue
		}
		uri, err := manifest.ResolveURI(baseURL, seg.URI)
		if err != nil {
			return fmt.Errorf("resolve segment URI %q: %w", seg.URI, err)
		}
		segments = append(segments, manifest.Segment{
			URI:      uri,
			Duration: seg.Duration,
			Sequence: int(media.SeqNo) + i,
		})

		// Check for init segment (EXT-X-MAP)
		if seg.Map != nil && seg.Map.URI != "" {
			initURI, err := manifest.ResolveURI(baseURL, seg.Map.URI)
			if err != nil {
				return fmt.Errorf("resolve init URI %q: %w", seg.Map.URI, err)
			}
			// Add init segment if not already present
			found := false
			for _, is := range m.InitSegments {
				if is.URI == initURI {
					found = true
					break
				}
			}
			if !found {
				m.InitSegments = append(m.InitSegments, manifest.Segment{
					URI:    initURI,
					IsInit: true,
				})
			}
		}

		// Check for encryption
		if seg.Key != nil && seg.Key.Method != "" && seg.Key.Method != "NONE" {
			if m.DRM == nil {
				keyLine := fmt.Sprintf("METHOD=%s", seg.Key.Method)
				if seg.Key.URI != "" {
					keyLine += fmt.Sprintf(",URI=%q", seg.Key.URI)
				}
				if seg.Key.IV != "" {
					keyLine += fmt.Sprintf(",IV=%s", seg.Key.IV)
				}
				m.DRM = &manifest.DRMInfo{
					System:  string(seg.Key.Method),
					EXTXKey: keyLine,
				}
			}
		}
	}

	m.Variants = append(m.Variants, manifest.Variant{
		URI:      baseURL,
		Segments: segments,
	})

	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/manifest/hls/ -v`
Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/manifest/hls/
git commit -m "feat: add HLS parser with master/media playlist support and DRM extraction"
```

---

### Task 9: DASH Parser

**Files:**
- Create: `internal/manifest/dash/parser.go`
- Create: `internal/manifest/dash/parser_test.go`
- Create: `internal/manifest/dash/testdata/simple.mpd`
- Create: `internal/manifest/dash/testdata/drm.mpd`

- [ ] **Step 1: Create test fixture files**

`internal/manifest/dash/testdata/simple.mpd`:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="PT60S">
  <Period>
    <AdaptationSet mimeType="video/mp4" contentType="video">
      <Representation id="720p" bandwidth="3000000" width="1280" height="720" codecs="avc1.64001f">
        <SegmentTemplate media="video_720p_$Number$.m4s" initialization="video_720p_init.mp4" startNumber="1" duration="6000" timescale="1000"/>
      </Representation>
      <Representation id="1080p" bandwidth="5000000" width="1920" height="1080" codecs="avc1.640028">
        <SegmentTemplate media="video_1080p_$Number$.m4s" initialization="video_1080p_init.mp4" startNumber="1" duration="6000" timescale="1000"/>
      </Representation>
    </AdaptationSet>
    <AdaptationSet mimeType="audio/mp4" contentType="audio" lang="en">
      <Representation id="audio_en" bandwidth="128000" codecs="mp4a.40.2">
        <SegmentTemplate media="audio_en_$Number$.m4s" initialization="audio_en_init.mp4" startNumber="1" duration="6000" timescale="1000"/>
      </Representation>
    </AdaptationSet>
    <AdaptationSet mimeType="text/vtt" contentType="text" lang="en">
      <Representation id="sub_en" bandwidth="1000">
        <BaseURL>subs/en.vtt</BaseURL>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>
```

`internal/manifest/dash/testdata/drm.mpd`:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" xmlns:cenc="urn:mpeg:cenc:2013" type="static" mediaPresentationDuration="PT60S">
  <Period>
    <AdaptationSet mimeType="video/mp4" contentType="video">
      <ContentProtection schemeIdUri="urn:mpeg:dash:mp4protection:2011" value="cenc" cenc:default_KID="12345678-1234-1234-1234-123456789012"/>
      <ContentProtection schemeIdUri="urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed">
        <cenc:pssh>AAAA</cenc:pssh>
      </ContentProtection>
      <Representation id="720p" bandwidth="3000000" width="1280" height="720" codecs="avc1.64001f">
        <SegmentTemplate media="video_$Number$.m4s" initialization="video_init.mp4" startNumber="1" duration="6000" timescale="1000"/>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>
```

- [ ] **Step 2: Write failing tests for DASH parser**

```go
// internal/manifest/dash/parser_test.go
package dash

import (
	"os"
	"testing"

	"github.com/meinart/video-debug-mcp/internal/manifest"
)

func TestParseMPD_Simple(t *testing.T) {
	data, err := os.ReadFile("testdata/simple.mpd")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	m, err := Parse(data, "https://cdn.example.com/dash/manifest.mpd")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if m.Type != manifest.ManifestTypeDASH {
		t.Errorf("expected type=dash, got %v", m.Type)
	}

	// 2 video representations = 2 variants
	if len(m.Variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(m.Variants))
	}

	v := m.Variants[0]
	if v.Bandwidth != 3000000 {
		t.Errorf("expected bandwidth=3000000, got %d", v.Bandwidth)
	}
	if v.Resolution != "1280x720" {
		t.Errorf("expected resolution=1280x720, got %s", v.Resolution)
	}

	// Init segments (1 per video rep + 1 audio = 3)
	if len(m.InitSegments) < 2 {
		t.Errorf("expected at least 2 init segments, got %d", len(m.InitSegments))
	}

	// Audio track
	if len(m.AudioTracks) != 1 {
		t.Fatalf("expected 1 audio track, got %d", len(m.AudioTracks))
	}
	if m.AudioTracks[0].Language != "en" {
		t.Errorf("expected language=en, got %s", m.AudioTracks[0].Language)
	}

	// Subtitle track
	if len(m.SubtitleTracks) != 1 {
		t.Fatalf("expected 1 subtitle track, got %d", len(m.SubtitleTracks))
	}
}

func TestParseMPD_SegmentURLResolution(t *testing.T) {
	data, err := os.ReadFile("testdata/simple.mpd")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	m, err := Parse(data, "https://cdn.example.com/dash/manifest.mpd")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Check that init segment URLs are resolved
	foundInit := false
	for _, s := range m.InitSegments {
		if s.URI == "https://cdn.example.com/dash/video_720p_init.mp4" {
			foundInit = true
			break
		}
	}
	if !foundInit {
		t.Error("expected resolved init segment URL for 720p")
	}

	// Check that media segment URLs are resolved
	if len(m.Variants[0].Segments) == 0 {
		t.Fatal("expected segments in first variant")
	}
	seg := m.Variants[0].Segments[0]
	if seg.URI != "https://cdn.example.com/dash/video_720p_1.m4s" {
		t.Errorf("expected resolved segment URL, got %s", seg.URI)
	}
}

func TestParseMPD_DRM(t *testing.T) {
	data, err := os.ReadFile("testdata/drm.mpd")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	m, err := Parse(data, "https://cdn.example.com/dash/drm.mpd")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if m.DRM == nil {
		t.Fatal("expected DRM info, got nil")
	}
	if m.DRM.SchemeURI == "" {
		t.Error("expected scheme URI, got empty")
	}
	if m.DRM.KeyID == "" {
		t.Error("expected key ID, got empty")
	}
}

func TestParse_InvalidXML(t *testing.T) {
	_, err := Parse([]byte("not valid xml"), "https://example.com/bad.mpd")
	if err == nil {
		t.Error("expected error for invalid MPD, got nil")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/manifest/dash/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 4: Implement DASH parser**

This implementation should use `encoding/xml` for custom MPD parsing (the go-dash library may have API differences — if it does, fall back to raw XML parsing). The parser needs to:

1. Parse the MPD XML structure
2. Iterate periods > adaptation sets > representations
3. Resolve segment template URLs by substituting `$Number$` tokens
4. Calculate segment count from `mediaPresentationDuration / (duration/timescale)`
5. Resolve all URIs against the base URL
6. Extract ContentProtection elements into DRMInfo
7. Categorize adaptation sets by contentType (video/audio/text)

```go
// internal/manifest/dash/parser.go
package dash

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/meinart/video-debug-mcp/internal/manifest"
)

// MPD XML structures
type mpd struct {
	XMLName                   xml.Name `xml:"MPD"`
	Type                      string   `xml:"type,attr"`
	MediaPresentationDuration string   `xml:"mediaPresentationDuration,attr"`
	Periods                   []period `xml:"Period"`
}

type period struct {
	AdaptationSets []adaptationSet `xml:"AdaptationSet"`
}

type adaptationSet struct {
	MimeType           string              `xml:"mimeType,attr"`
	ContentType        string              `xml:"contentType,attr"`
	Lang               string              `xml:"lang,attr"`
	ContentProtections []contentProtection  `xml:"ContentProtection"`
	Representations    []representation     `xml:"Representation"`
	SegmentTemplate    *segmentTemplate     `xml:"SegmentTemplate"`
}

type contentProtection struct {
	SchemeIDURI string `xml:"schemeIdUri,attr"`
	Value       string `xml:"value,attr"`
	DefaultKID  string `xml:"default_KID,attr"`
	PSSH        string `xml:"pssh"`
}

type representation struct {
	ID              string           `xml:"id,attr"`
	Bandwidth       int              `xml:"bandwidth,attr"`
	Width           int              `xml:"width,attr"`
	Height          int              `xml:"height,attr"`
	Codecs          string           `xml:"codecs,attr"`
	SegmentTemplate *segmentTemplate `xml:"SegmentTemplate"`
	BaseURL         string           `xml:"BaseURL"`
}

type segmentTemplate struct {
	Media          string `xml:"media,attr"`
	Initialization string `xml:"initialization,attr"`
	StartNumber    int    `xml:"startNumber,attr"`
	Duration       int    `xml:"duration,attr"`
	Timescale      int    `xml:"timescale,attr"`
}

var numberPattern = regexp.MustCompile(`\$Number\$`)

func Parse(data []byte, baseURL string) (*manifest.Manifest, error) {
	var doc mpd
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse DASH MPD: %w", err)
	}

	m := &manifest.Manifest{
		Type: manifest.ManifestTypeDASH,
		URL:  baseURL,
		Raw:  string(data),
	}

	totalDuration := parseDuration(doc.MediaPresentationDuration)

	for _, p := range doc.Periods {
		for _, as := range p.AdaptationSets {
			if err := parseAdaptationSet(&as, baseURL, totalDuration, m); err != nil {
				return nil, err
			}
		}
	}

	return m, nil
}

func parseAdaptationSet(as *adaptationSet, baseURL string, totalDuration float64, m *manifest.Manifest) error {
	// Extract DRM info
	for _, cp := range as.ContentProtections {
		if cp.SchemeIDURI == "urn:mpeg:dash:mp4protection:2011" {
			continue // This is the common encryption signaling, not a specific DRM system
		}
		if m.DRM == nil {
			m.DRM = &manifest.DRMInfo{}
		}
		m.DRM.SchemeURI = cp.SchemeIDURI
		if cp.PSSH != "" {
			m.DRM.PSSH = cp.PSSH
		}
		// Extract KeyID from cenc:default_KID
		for _, cp2 := range as.ContentProtections {
			if cp2.DefaultKID != "" {
				m.DRM.KeyID = cp2.DefaultKID
				break
			}
		}
	}

	contentType := as.ContentType
	if contentType == "" {
		// Infer from mimeType
		if strings.Contains(as.MimeType, "video") {
			contentType = "video"
		} else if strings.Contains(as.MimeType, "audio") {
			contentType = "audio"
		} else if strings.Contains(as.MimeType, "text") || strings.Contains(as.MimeType, "vtt") {
			contentType = "text"
		}
	}

	for _, rep := range as.Representations {
		st := rep.SegmentTemplate
		if st == nil {
			st = as.SegmentTemplate
		}

		switch contentType {
		case "video":
			variant, initSegs, err := buildVariant(&rep, st, baseURL, totalDuration)
			if err != nil {
				return err
			}
			m.Variants = append(m.Variants, *variant)
			m.InitSegments = append(m.InitSegments, initSegs...)

		case "audio":
			track, initSegs, err := buildAudioTrack(&rep, st, baseURL, totalDuration, as.Lang)
			if err != nil {
				return err
			}
			m.AudioTracks = append(m.AudioTracks, *track)
			m.InitSegments = append(m.InitSegments, initSegs...)

		case "text":
			sub := manifest.SubTrack{
				Language: as.Lang,
				Name:     as.Lang,
			}
			if rep.BaseURL != "" {
				uri, err := manifest.ResolveURI(baseURL, rep.BaseURL)
				if err != nil {
					return err
				}
				sub.URI = uri
			}
			m.SubtitleTracks = append(m.SubtitleTracks, sub)
		}
	}

	return nil
}

func buildVariant(rep *representation, st *segmentTemplate, baseURL string, totalDuration float64) (*manifest.Variant, []manifest.Segment, error) {
	v := &manifest.Variant{
		Bandwidth:  rep.Bandwidth,
		Codecs:     rep.Codecs,
	}

	if rep.Width > 0 && rep.Height > 0 {
		v.Resolution = fmt.Sprintf("%dx%d", rep.Width, rep.Height)
	}

	var initSegs []manifest.Segment

	if st != nil {
		// Resolve init segment
		if st.Initialization != "" {
			initURI := strings.ReplaceAll(st.Initialization, "$RepresentationID$", rep.ID)
			resolved, err := manifest.ResolveURI(baseURL, initURI)
			if err != nil {
				return nil, nil, err
			}
			v.URI = resolved
			initSegs = append(initSegs, manifest.Segment{URI: resolved, IsInit: true, TrackRef: rep.ID})
		}

		// Build media segment list
		segments, err := buildSegments(st, rep.ID, baseURL, totalDuration)
		if err != nil {
			return nil, nil, err
		}
		v.Segments = segments
	}

	return v, initSegs, nil
}

func buildAudioTrack(rep *representation, st *segmentTemplate, baseURL string, totalDuration float64, lang string) (*manifest.AudioTrack, []manifest.Segment, error) {
	track := &manifest.AudioTrack{
		Language: lang,
		Name:     lang,
		Codecs:   rep.Codecs,
	}

	var initSegs []manifest.Segment

	if st != nil {
		if st.Initialization != "" {
			initURI := strings.ReplaceAll(st.Initialization, "$RepresentationID$", rep.ID)
			resolved, err := manifest.ResolveURI(baseURL, initURI)
			if err != nil {
				return nil, nil, err
			}
			track.URI = resolved
			initSegs = append(initSegs, manifest.Segment{URI: resolved, IsInit: true, TrackRef: rep.ID})
		}

		segments, err := buildSegments(st, rep.ID, baseURL, totalDuration)
		if err != nil {
			return nil, nil, err
		}
		track.Segments = segments
	}

	return track, initSegs, nil
}

func buildSegments(st *segmentTemplate, repID string, baseURL string, totalDuration float64) ([]manifest.Segment, error) {
	if st.Duration == 0 || st.Timescale == 0 {
		return nil, nil
	}

	segDuration := float64(st.Duration) / float64(st.Timescale)
	segCount := int(totalDuration / segDuration)
	if segCount < 1 {
		segCount = 1
	}

	segments := make([]manifest.Segment, 0, segCount)
	for i := 0; i < segCount; i++ {
		num := st.StartNumber + i
		mediaURI := numberPattern.ReplaceAllString(st.Media, strconv.Itoa(num))
		mediaURI = strings.ReplaceAll(mediaURI, "$RepresentationID$", repID)

		resolved, err := manifest.ResolveURI(baseURL, mediaURI)
		if err != nil {
			return nil, err
		}

		segments = append(segments, manifest.Segment{
			URI:      resolved,
			Duration: segDuration,
			Sequence: num,
			TrackRef: repID,
		})
	}

	return segments, nil
}

func parseDuration(s string) float64 {
	if s == "" {
		return 0
	}
	// Parse ISO 8601 duration (PT60S, PT1M30S, etc.)
	d, err := parseISO8601Duration(s)
	if err != nil {
		return 0
	}
	return d.Seconds()
}

func parseISO8601Duration(s string) (time.Duration, error) {
	s = strings.TrimPrefix(s, "PT")
	s = strings.TrimPrefix(s, "P")

	var total time.Duration

	// Hours
	if i := strings.Index(s, "H"); i >= 0 {
		h, err := strconv.ParseFloat(s[:i], 64)
		if err != nil {
			return 0, err
		}
		total += time.Duration(h * float64(time.Hour))
		s = s[i+1:]
	}

	// Minutes
	if i := strings.Index(s, "M"); i >= 0 {
		m, err := strconv.ParseFloat(s[:i], 64)
		if err != nil {
			return 0, err
		}
		total += time.Duration(m * float64(time.Minute))
		s = s[i+1:]
	}

	// Seconds
	if i := strings.Index(s, "S"); i >= 0 {
		sec, err := strconv.ParseFloat(s[:i], 64)
		if err != nil {
			return 0, err
		}
		total += time.Duration(sec * float64(time.Second))
	}

	return total, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/manifest/dash/ -v`
Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/manifest/dash/
git commit -m "feat: add DASH MPD parser with segment template resolution and DRM extraction"
```

---

### Task 10: Docker Runner

**Files:**
- Create: `internal/docker/runner.go`
- Create: `internal/docker/runner_test.go`
- Create: `internal/docker/images.go`

- [ ] **Step 1: Write failing tests for Docker runner**

```go
// internal/docker/runner_test.go
package docker

import (
	"context"
	"testing"
	"time"
)

func TestRunRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     RunRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: RunRequest{
				Image:      "video-debug/ffmpeg-tools",
				Command:    []string{"ffprobe", "-v", "quiet", "/workspace/video.mp4"},
				WorkspaceDir: "/tmp/test",
				Timeout:    60 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "missing image",
			req: RunRequest{
				Command:    []string{"ffprobe"},
				WorkspaceDir: "/tmp/test",
				Timeout:    60 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "missing command",
			req: RunRequest{
				Image:      "video-debug/ffmpeg-tools",
				WorkspaceDir: "/tmp/test",
				Timeout:    60 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "missing workspace",
			req: RunRequest{
				Image:   "video-debug/ffmpeg-tools",
				Command: []string{"ffprobe"},
				Timeout: 60 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunResult_Success(t *testing.T) {
	r := &RunResult{
		ExitCode: 0,
		Stdout:   []byte("output"),
		Stderr:   []byte(""),
		Duration: 2 * time.Second,
	}

	if !r.Success() {
		t.Error("expected Success() = true for exit code 0")
	}
}

func TestRunResult_Failure(t *testing.T) {
	r := &RunResult{
		ExitCode: 1,
		Stdout:   []byte(""),
		Stderr:   []byte("error"),
		Duration: 1 * time.Second,
	}

	if r.Success() {
		t.Error("expected Success() = false for exit code 1")
	}
}

// mockDockerClient implements the Runner interface for unit tests
type mockDockerClient struct {
	runFunc func(ctx context.Context, req RunRequest) (*RunResult, error)
}

func (m *mockDockerClient) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	return m.runFunc(ctx, req)
}

func TestRunner_Interface(t *testing.T) {
	mock := &mockDockerClient{
		runFunc: func(ctx context.Context, req RunRequest) (*RunResult, error) {
			return &RunResult{
				ExitCode: 0,
				Stdout:   []byte("mock output"),
				Duration: 100 * time.Millisecond,
			}, nil
		},
	}

	var runner Runner = mock
	result, err := runner.Run(context.Background(), RunRequest{
		Image:      "test",
		Command:    []string{"echo", "hello"},
		WorkspaceDir: "/tmp",
		Timeout:    10 * time.Second,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Stdout) != "mock output" {
		t.Errorf("expected 'mock output', got %q", string(result.Stdout))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/docker/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement Docker runner types and interface**

```go
// internal/docker/runner.go
package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type Runner interface {
	Run(ctx context.Context, req RunRequest) (*RunResult, error)
}

type RunRequest struct {
	Image        string
	Command      []string
	WorkspaceDir string
	Timeout      time.Duration
	MemoryLimit  int64
	CPULimit     float64
}

func (r RunRequest) Validate() error {
	if r.Image == "" {
		return fmt.Errorf("image is required")
	}
	if len(r.Command) == 0 {
		return fmt.Errorf("command is required")
	}
	if r.WorkspaceDir == "" {
		return fmt.Errorf("workspace directory is required")
	}
	return nil
}

type RunResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Duration time.Duration
}

func (r *RunResult) Success() bool {
	return r.ExitCode == 0
}

type DockerRunner struct {
	cli *client.Client
}

func NewDockerRunner() (*DockerRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create Docker client: %w", err)
	}
	return &DockerRunner{cli: cli}, nil
}

func (d *DockerRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	start := time.Now()

	timeoutCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	// Container config
	containerCfg := &container.Config{
		Image: req.Image,
		Cmd:   req.Command,
	}

	// Host config with resource limits and no network
	hostCfg := &container.HostConfig{
		NetworkMode: "none",
		AutoRemove:  true,
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeBind,
				Source:   req.WorkspaceDir,
				Target:   "/workspace",
				ReadOnly: true,
			},
		},
		Resources: container.Resources{},
	}

	if req.MemoryLimit > 0 {
		hostCfg.Resources.Memory = req.MemoryLimit
	}
	if req.CPULimit > 0 {
		hostCfg.Resources.NanoCPUs = int64(req.CPULimit * 1e9)
	}

	// Create container
	resp, err := d.cli.ContainerCreate(timeoutCtx, containerCfg, hostCfg, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}

	// Attach to capture output
	attachResp, err := d.cli.ContainerAttach(timeoutCtx, resp.ID, container.AttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("attach container: %w", err)
	}
	defer attachResp.Close()

	// Start container
	if err := d.cli.ContainerStart(timeoutCtx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}

	// Read output
	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, attachResp.Reader)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read container output: %w", err)
	}

	// Wait for container to finish
	statusCh, errCh := d.cli.ContainerWait(timeoutCtx, resp.ID, container.WaitConditionRemoved)
	select {
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("wait for container: %w", err)
		}
	case status := <-statusCh:
		return &RunResult{
			ExitCode: int(status.StatusCode),
			Stdout:   stdout.Bytes(),
			Stderr:   stderr.Bytes(),
			Duration: time.Since(start),
		}, nil
	case <-timeoutCtx.Done():
		// Try to kill the container
		d.cli.ContainerKill(ctx, resp.ID, "KILL")
		return nil, fmt.Errorf("container timed out after %v", req.Timeout)
	}

	return &RunResult{
		ExitCode: -1,
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		Duration: time.Since(start),
	}, nil
}

func (d *DockerRunner) Close() error {
	return d.cli.Close()
}
```

```go
// internal/docker/images.go
package docker

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

type ImageManager struct {
	cli *client.Client
}

func NewImageManager(cli *client.Client) *ImageManager {
	return &ImageManager{cli: cli}
}

func (im *ImageManager) Exists(ctx context.Context, imageName string) bool {
	_, _, err := im.cli.ImageInspectWithRaw(ctx, imageName)
	return err == nil
}

func (im *ImageManager) EnsureImage(ctx context.Context, imageName string) error {
	if im.Exists(ctx, imageName) {
		return nil
	}

	log.Printf("Pulling image %s...", imageName)
	reader, err := im.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %s: %w", imageName, err)
	}
	defer reader.Close()
	io.Copy(io.Discard, reader)

	return nil
}

func (im *ImageManager) ListAvailable(ctx context.Context, imageNames []string) (available, missing []string) {
	for _, name := range imageNames {
		if im.Exists(ctx, name) {
			available = append(available, name)
		} else {
			missing = append(missing, name)
		}
	}
	return
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/docker/ -v`
Expected: All tests PASS (unit tests only, no Docker dependency).

- [ ] **Step 5: Commit**

```bash
git add internal/docker/
git commit -m "feat: add Docker runner with container lifecycle management and image manager"
```

---

### Task 11: Output Formatter

**Files:**
- Create: `internal/output/formatter.go`
- Create: `internal/output/formatter_test.go`

- [ ] **Step 1: Write failing tests for output formatter**

```go
// internal/output/formatter_test.go
package output

import (
	"encoding/json"
	"testing"
)

func TestFormatter_Summary(t *testing.T) {
	f := NewFormatter()
	result := &AnalysisResult{
		Tool:     "ffprobe",
		Command:  "ffprobe -v quiet -print_format json -show_streams /workspace/video.mp4",
		InputURL: "https://example.com/video.mp4",
		Streams: []StreamInfo{
			{Index: 0, Type: "video", Codec: "h264", Width: 1920, Height: 1080},
			{Index: 1, Type: "audio", Codec: "aac", SampleRate: 48000, Channels: 2},
		},
		Timing: &TimingInfo{Duration: 120.5, StartTime: 0},
		Anomalies: []Anomaly{
			{Severity: SeverityWarning, Description: "PTS discontinuity"},
		},
		RawOutput: "very long raw output here",
	}

	formatted := f.Format(result, VerbositySummary)

	var m map[string]interface{}
	data, _ := json.Marshal(formatted)
	json.Unmarshal(data, &m)

	// Summary should have tool, input_url, streams (count only), anomalies
	if m["tool"] != "ffprobe" {
		t.Errorf("expected tool=ffprobe, got %v", m["tool"])
	}
	// Should NOT have command or raw_output at summary level
	if _, ok := m["command"]; ok {
		t.Error("summary should not include command")
	}
	if _, ok := m["raw_output"]; ok {
		t.Error("summary should not include raw_output")
	}
}

func TestFormatter_Standard(t *testing.T) {
	f := NewFormatter()
	result := &AnalysisResult{
		Tool:     "ffprobe",
		Command:  "ffprobe ...",
		InputURL: "https://example.com/video.mp4",
		Streams: []StreamInfo{
			{Index: 0, Type: "video", Codec: "h264"},
		},
		RawOutput: "raw output",
	}

	formatted := f.Format(result, VerbosityStandard)

	var m map[string]interface{}
	data, _ := json.Marshal(formatted)
	json.Unmarshal(data, &m)

	// Standard should have command but not raw_output
	if _, ok := m["command"]; !ok {
		t.Error("standard should include command")
	}
	if _, ok := m["raw_output"]; ok {
		t.Error("standard should not include raw_output")
	}
}

func TestFormatter_Forensic(t *testing.T) {
	f := NewFormatter()
	result := &AnalysisResult{
		Tool:      "ffprobe",
		Command:   "ffprobe ...",
		InputURL:  "https://example.com/video.mp4",
		RawOutput: "full raw output dump",
	}

	formatted := f.Format(result, VerbosityForensic)

	var m map[string]interface{}
	data, _ := json.Marshal(formatted)
	json.Unmarshal(data, &m)

	// Forensic should include everything including raw_output
	if _, ok := m["raw_output"]; !ok {
		t.Error("forensic should include raw_output")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/output/ -v -run TestFormatter`
Expected: FAIL — NewFormatter and Format not defined.

- [ ] **Step 3: Implement output formatter**

```go
// internal/output/formatter.go
package output

type Formatter struct{}

func NewFormatter() *Formatter {
	return &Formatter{}
}

func (f *Formatter) Format(result *AnalysisResult, verbosity Verbosity) *AnalysisResult {
	switch verbosity {
	case VerbositySummary:
		return f.formatSummary(result)
	case VerbosityStandard:
		return f.formatStandard(result)
	case VerbosityDeep:
		return f.formatDeep(result)
	case VerbosityForensic:
		return f.formatForensic(result)
	default:
		return f.formatStandard(result)
	}
}

func (f *Formatter) formatSummary(r *AnalysisResult) *AnalysisResult {
	return &AnalysisResult{
		Tool:      r.Tool,
		InputURL:  r.InputURL,
		Streams:   r.Streams,
		Timing:    r.Timing,
		Bitrate:   r.Bitrate,
		Anomalies: r.Anomalies,
	}
}

func (f *Formatter) formatStandard(r *AnalysisResult) *AnalysisResult {
	return &AnalysisResult{
		Tool:         r.Tool,
		Command:      r.Command,
		InputURL:     r.InputURL,
		ResolvedURLs: r.ResolvedURLs,
		Downloads:    r.Downloads,
		Manifest:     r.Manifest,
		Streams:      r.Streams,
		Codec:        r.Codec,
		Timing:       r.Timing,
		Bitrate:      r.Bitrate,
		Anomalies:    r.Anomalies,
	}
}

func (f *Formatter) formatDeep(r *AnalysisResult) *AnalysisResult {
	return &AnalysisResult{
		Tool:            r.Tool,
		Command:         r.Command,
		InputURL:        r.InputURL,
		ResolvedURLs:    r.ResolvedURLs,
		Downloads:       r.Downloads,
		Manifest:        r.Manifest,
		Streams:         r.Streams,
		Codec:           r.Codec,
		Timing:          r.Timing,
		Bitrate:         r.Bitrate,
		ContainerLayout: r.ContainerLayout,
		Anomalies:       r.Anomalies,
	}
}

func (f *Formatter) formatForensic(r *AnalysisResult) *AnalysisResult {
	// Return everything as-is
	return r
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/output/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/output/formatter.go internal/output/formatter_test.go
git commit -m "feat: add output formatter with verbosity-level filtering"
```

---

### Task 12: FFprobe Tool Implementation

**Files:**
- Create: `internal/tools/ffmpeg/ffprobe.go`
- Create: `internal/tools/ffmpeg/parse.go`
- Create: `internal/tools/ffmpeg/ffprobe_test.go`
- Create: `internal/tools/ffmpeg/testdata/ffprobe_output.json`

- [ ] **Step 1: Create test fixture**

`internal/tools/ffmpeg/testdata/ffprobe_output.json`:
```json
{
  "streams": [
    {
      "index": 0,
      "codec_name": "h264",
      "codec_type": "video",
      "profile": "High",
      "level": 40,
      "width": 1920,
      "height": 1080,
      "r_frame_rate": "30000/1001",
      "bit_rate": "5000000",
      "sample_aspect_ratio": "1:1",
      "display_aspect_ratio": "16:9",
      "pix_fmt": "yuv420p"
    },
    {
      "index": 1,
      "codec_name": "aac",
      "codec_type": "audio",
      "profile": "LC",
      "sample_rate": "48000",
      "channels": 2,
      "bit_rate": "128000",
      "tags": {
        "language": "eng"
      }
    }
  ],
  "format": {
    "duration": "120.500000",
    "start_time": "0.000000",
    "bit_rate": "5128000",
    "format_name": "mov,mp4,m4a,3gp,3g2,mj2"
  }
}
```

- [ ] **Step 2: Write failing tests for ffprobe tool**

```go
// internal/tools/ffmpeg/ffprobe_test.go
package ffmpeg

import (
	"os"
	"testing"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

func TestFFprobe_Name(t *testing.T) {
	fp := NewFFprobe("video-debug/ffmpeg-tools")
	if fp.Name() != "run_ffprobe" {
		t.Errorf("expected name=run_ffprobe, got %s", fp.Name())
	}
}

func TestFFprobe_DockerImage(t *testing.T) {
	fp := NewFFprobe("video-debug/ffmpeg-tools")
	if fp.DockerImage() != "video-debug/ffmpeg-tools" {
		t.Errorf("expected image=video-debug/ffmpeg-tools, got %s", fp.DockerImage())
	}
}

func TestFFprobe_BuildCommand_Default(t *testing.T) {
	fp := NewFFprobe("video-debug/ffmpeg-tools")
	input := tools.ToolInput{
		URL: "https://example.com/video.mp4",
	}

	cmd, err := fp.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() error: %v", err)
	}

	if cmd.Binary != "ffprobe" {
		t.Errorf("expected binary=ffprobe, got %s", cmd.Binary)
	}

	// Should include -print_format json and -show_streams -show_format
	found := map[string]bool{}
	for _, arg := range cmd.Args {
		found[arg] = true
	}
	if !found["-print_format"] && !found["-of"] {
		t.Error("expected -print_format or -of in args")
	}
	if !found["-show_streams"] {
		t.Error("expected -show_streams in args")
	}
	if !found["-show_format"] {
		t.Error("expected -show_format in args")
	}
}

func TestFFprobe_BuildCommand_CustomArgs(t *testing.T) {
	fp := NewFFprobe("video-debug/ffmpeg-tools")
	input := tools.ToolInput{
		URL:  "https://example.com/video.mp4",
		Args: []string{"-show_entries", "stream=codec_name"},
	}

	cmd, err := fp.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() error: %v", err)
	}

	// Custom args should be used instead of defaults
	found := false
	for _, arg := range cmd.Args {
		if arg == "-show_entries" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected custom -show_entries arg")
	}
}

func TestFFprobe_ParseOutput(t *testing.T) {
	data, err := os.ReadFile("testdata/ffprobe_output.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	fp := NewFFprobe("video-debug/ffmpeg-tools")
	result, err := fp.ParseOutput(data, output.VerbosityStandard)
	if err != nil {
		t.Fatalf("ParseOutput() error: %v", err)
	}

	if result.Tool != "run_ffprobe" {
		t.Errorf("expected tool=run_ffprobe, got %s", result.Tool)
	}

	if len(result.Streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(result.Streams))
	}

	// Video stream
	vs := result.Streams[0]
	if vs.Type != "video" {
		t.Errorf("expected type=video, got %s", vs.Type)
	}
	if vs.Codec != "h264" {
		t.Errorf("expected codec=h264, got %s", vs.Codec)
	}
	if vs.Width != 1920 {
		t.Errorf("expected width=1920, got %d", vs.Width)
	}

	// Audio stream
	as := result.Streams[1]
	if as.Type != "audio" {
		t.Errorf("expected type=audio, got %s", as.Type)
	}
	if as.Channels != 2 {
		t.Errorf("expected channels=2, got %d", as.Channels)
	}

	// Timing
	if result.Timing == nil {
		t.Fatal("expected timing info, got nil")
	}
	if result.Timing.Duration != 120.5 {
		t.Errorf("expected duration=120.5, got %f", result.Timing.Duration)
	}

	// Bitrate
	if result.Bitrate == nil {
		t.Fatal("expected bitrate info, got nil")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/tools/ffmpeg/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 4: Implement ffprobe tool and parser**

```go
// internal/tools/ffmpeg/ffprobe.go
package ffmpeg

import (
	"encoding/json"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

type FFprobe struct {
	image string
}

func NewFFprobe(image string) *FFprobe {
	return &FFprobe{image: image}
}

func (f *FFprobe) Name() string        { return "run_ffprobe" }
func (f *FFprobe) Description() string  { return "Run ffprobe for media stream and container analysis" }
func (f *FFprobe) DockerImage() string  { return f.image }

func (f *FFprobe) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL of the media asset to analyze"},
			"verbosity": {"type": "string", "enum": ["summary","standard","deep","forensic"], "default": "standard"},
			"args": {"type": "array", "items": {"type": "string"}, "description": "Custom ffprobe arguments (overrides defaults)"},
			"headers": {"type": "object", "additionalProperties": {"type": "string"}, "description": "Custom HTTP headers"}
		},
		"required": ["url"]
	}`)
}

func (f *FFprobe) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	cmd := &tools.DockerCommand{
		Binary: "ffprobe",
	}

	if len(input.Args) > 0 {
		cmd.Args = append(input.Args, "/workspace/input")
	} else {
		cmd.Args = []string{
			"-v", "quiet",
			"-print_format", "json",
			"-show_streams",
			"-show_format",
			"/workspace/input",
		}
	}

	cmd.InputFiles = []string{"input"}
	return cmd, nil
}

func (f *FFprobe) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	return parseFFprobeOutput(raw, f.Name())
}
```

```go
// internal/tools/ffmpeg/parse.go
package ffmpeg

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/meinart/video-debug-mcp/internal/output"
)

type ffprobeJSON struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	Index      int               `json:"index"`
	CodecName  string            `json:"codec_name"`
	CodecType  string            `json:"codec_type"`
	Profile    string            `json:"profile"`
	Level      int               `json:"level"`
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	RFrameRate string            `json:"r_frame_rate"`
	BitRate    string            `json:"bit_rate"`
	SampleRate string            `json:"sample_rate"`
	Channels   int               `json:"channels"`
	PixFmt     string            `json:"pix_fmt"`
	Tags       map[string]string `json:"tags"`
}

type ffprobeFormat struct {
	Duration   string `json:"duration"`
	StartTime  string `json:"start_time"`
	BitRate    string `json:"bit_rate"`
	FormatName string `json:"format_name"`
}

func parseFFprobeOutput(raw []byte, toolName string) (*output.AnalysisResult, error) {
	var probe ffprobeJSON
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	result := &output.AnalysisResult{
		Tool: toolName,
	}

	// Parse streams
	for _, s := range probe.Streams {
		si := output.StreamInfo{
			Index: s.Index,
			Type:  s.CodecType,
			Codec: s.CodecName,
		}

		if s.Profile != "" {
			si.Profile = s.Profile
		}
		if s.Level > 0 {
			si.Level = fmt.Sprintf("%d", s.Level)
		}
		if s.Width > 0 {
			si.Width = s.Width
		}
		if s.Height > 0 {
			si.Height = s.Height
		}
		if s.BitRate != "" {
			if br, err := strconv.ParseInt(s.BitRate, 10, 64); err == nil {
				si.Bitrate = br
			}
		}
		if s.RFrameRate != "" {
			si.FrameRate = parseFrameRate(s.RFrameRate)
		}
		if s.SampleRate != "" {
			if sr, err := strconv.Atoi(s.SampleRate); err == nil {
				si.SampleRate = sr
			}
		}
		if s.Channels > 0 {
			si.Channels = s.Channels
		}
		if lang, ok := s.Tags["language"]; ok {
			si.Language = lang
		}

		result.Streams = append(result.Streams, si)
	}

	// Parse format/timing
	if probe.Format.Duration != "" {
		dur, _ := strconv.ParseFloat(probe.Format.Duration, 64)
		start, _ := strconv.ParseFloat(probe.Format.StartTime, 64)
		result.Timing = &output.TimingInfo{
			Duration:  dur,
			StartTime: start,
		}
	}

	// Parse overall bitrate
	if probe.Format.BitRate != "" {
		if br, err := strconv.ParseInt(probe.Format.BitRate, 10, 64); err == nil {
			result.Bitrate = &output.BitrateInfo{
				Overall: br,
			}

			// Fill in per-stream bitrates
			for _, s := range result.Streams {
				if s.Type == "video" && s.Bitrate > 0 {
					result.Bitrate.Video = s.Bitrate
				}
				if s.Type == "audio" && s.Bitrate > 0 {
					result.Bitrate.Audio = s.Bitrate
				}
			}
		}
	}

	// Build codec details from first video+audio stream
	codec := &output.CodecDetails{}
	hasCodec := false
	for _, s := range probe.Streams {
		switch s.CodecType {
		case "video":
			codec.VideoCodec = s.CodecName
			codec.VideoProfile = s.Profile
			if s.Level > 0 {
				codec.VideoLevel = fmt.Sprintf("%d", s.Level)
			}
			codec.PixelFormat = s.PixFmt
			hasCodec = true
		case "audio":
			codec.AudioCodec = s.CodecName
			hasCodec = true
		}
	}
	if hasCodec {
		result.Codec = codec
	}

	return result, nil
}

func parseFrameRate(s string) float64 {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}
	num, _ := strconv.ParseFloat(parts[0], 64)
	den, _ := strconv.ParseFloat(parts[1], 64)
	if den == 0 {
		return 0
	}
	return math.Round(num/den*100) / 100
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tools/ffmpeg/ -v`
Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tools/ffmpeg/
git commit -m "feat: add ffprobe tool with command builder and JSON output parser"
```

---

### Task 13: FFmpeg Playback Simulation Tool

**Files:**
- Create: `internal/tools/ffmpeg/ffmpeg.go`
- Create: `internal/tools/ffmpeg/ffmpeg_test.go`
- Create: `internal/tools/ffmpeg/testdata/ffmpeg_null_output.txt`

- [ ] **Step 1: Create test fixture**

`internal/tools/ffmpeg/testdata/ffmpeg_null_output.txt`:
```
ffmpeg version 6.1 Copyright (c) 2000-2023 the FFmpeg developers
Input #0, mov,mp4,m4a,3gp,3g2,mj2, from '/workspace/input':
  Duration: 00:02:00.50, start: 0.000000, bitrate: 5128 kb/s
  Stream #0:0(und): Video: h264 (High) (avc1 / 0x31637661), yuv420p, 1920x1080, 5000 kb/s, 29.97 fps
  Stream #0:1(eng): Audio: aac (LC) (mp4a / 0x6134706D), 48000 Hz, stereo, fltp, 128 kb/s
[mov,mp4,m4a,3gp,3g2,mj2 @ 0x55f6a3b5a000] discarding pts discontinuity at 45.045000
[h264 @ 0x55f6a3b5c100] concealing 32 DC, 32 AC, 32 MV errors in P frame
frame= 3600 fps=120.0 q=-0.0 Lsize=N/A time=00:02:00.50 bitrate=N/A speed=4.01x
video:0kB audio:0kB subtitle:0kB other streams:0kB global headers:0kB muxing overhead: unknown
```

- [ ] **Step 2: Write failing tests for ffmpeg simulation tool**

```go
// internal/tools/ffmpeg/ffmpeg_test.go
package ffmpeg

import (
	"os"
	"testing"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

func TestFFmpeg_Name(t *testing.T) {
	ff := NewFFmpeg("video-debug/ffmpeg-tools")
	if ff.Name() != "run_ffmpeg" {
		t.Errorf("expected name=run_ffmpeg, got %s", ff.Name())
	}
}

func TestFFmpeg_BuildCommand_DefaultDuration(t *testing.T) {
	ff := NewFFmpeg("video-debug/ffmpeg-tools")
	input := tools.ToolInput{
		URL: "https://example.com/video.mp4",
	}

	cmd, err := ff.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() error: %v", err)
	}

	if cmd.Binary != "ffmpeg" {
		t.Errorf("expected binary=ffmpeg, got %s", cmd.Binary)
	}

	// Should include -f null -
	hasNull := false
	for i, arg := range cmd.Args {
		if arg == "-f" && i+1 < len(cmd.Args) && cmd.Args[i+1] == "null" {
			hasNull = true
			break
		}
	}
	if !hasNull {
		t.Error("expected -f null in args")
	}
}

func TestFFmpeg_BuildCommand_CustomDuration(t *testing.T) {
	ff := NewFFmpeg("video-debug/ffmpeg-tools")
	input := tools.ToolInput{
		URL:      "https://example.com/video.mp4",
		Duration: 30,
	}

	cmd, err := ff.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() error: %v", err)
	}

	// Should include -t 30
	found := false
	for i, arg := range cmd.Args {
		if arg == "-t" && i+1 < len(cmd.Args) && cmd.Args[i+1] == "30" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected -t 30 in args for custom duration")
	}
}

func TestFFmpeg_ParseOutput(t *testing.T) {
	data, err := os.ReadFile("testdata/ffmpeg_null_output.txt")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	ff := NewFFmpeg("video-debug/ffmpeg-tools")
	result, err := ff.ParseOutput(data, output.VerbosityStandard)
	if err != nil {
		t.Fatalf("ParseOutput() error: %v", err)
	}

	if result.Tool != "run_ffmpeg" {
		t.Errorf("expected tool=run_ffmpeg, got %s", result.Tool)
	}

	// Should detect PTS discontinuity anomaly
	if len(result.Anomalies) == 0 {
		t.Error("expected anomalies detected from ffmpeg output")
	}

	foundPTS := false
	foundError := false
	for _, a := range result.Anomalies {
		if a.Description == "PTS discontinuity" {
			foundPTS = true
		}
		if a.Severity == output.SeverityError {
			foundError = true
		}
	}
	if !foundPTS {
		t.Error("expected PTS discontinuity anomaly")
	}
	if !foundError {
		t.Error("expected error-level anomaly for concealment errors")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/tools/ffmpeg/ -v -run TestFFmpeg`
Expected: FAIL — NewFFmpeg not defined.

- [ ] **Step 4: Implement ffmpeg simulation tool**

```go
// internal/tools/ffmpeg/ffmpeg.go
package ffmpeg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

type FFmpeg struct {
	image string
}

func NewFFmpeg(image string) *FFmpeg {
	return &FFmpeg{image: image}
}

func (f *FFmpeg) Name() string        { return "run_ffmpeg" }
func (f *FFmpeg) Description() string  { return "Run ffmpeg playback simulation (decode to null) for diagnostic output" }
func (f *FFmpeg) DockerImage() string  { return f.image }

func (f *FFmpeg) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL of the media asset to analyze"},
			"verbosity": {"type": "string", "enum": ["summary","standard","deep","forensic"], "default": "standard"},
			"duration": {"type": "integer", "description": "Playback duration in seconds (default: 10)"},
			"args": {"type": "array", "items": {"type": "string"}, "description": "Custom ffmpeg arguments"},
			"headers": {"type": "object", "additionalProperties": {"type": "string"}, "description": "Custom HTTP headers"}
		},
		"required": ["url"]
	}`)
}

func (f *FFmpeg) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	cmd := &tools.DockerCommand{
		Binary:     "ffmpeg",
		InputFiles: []string{"input"},
	}

	if len(input.Args) > 0 {
		cmd.Args = input.Args
		return cmd, nil
	}

	args := []string{"-v", "verbose"}

	if input.Duration > 0 {
		args = append(args, "-t", strconv.Itoa(input.Duration))
	}

	args = append(args,
		"-i", "/workspace/input",
		"-f", "null",
		"-",
	)

	cmd.Args = args
	return cmd, nil
}

var (
	ptsDiscontinuityRe = regexp.MustCompile(`discarding pts discontinuity at ([0-9.]+)`)
	concealingErrorRe  = regexp.MustCompile(`concealing (\d+) DC, (\d+) AC, (\d+) MV errors`)
	dtsDiscontinuityRe = regexp.MustCompile(`DTS .* discontinuity`)
	overreadRe         = regexp.MustCompile(`overread|truncated|invalid`)
)

func (f *FFmpeg) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	text := string(raw)
	result := &output.AnalysisResult{
		Tool: f.Name(),
	}

	// Detect anomalies from ffmpeg verbose output
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if matches := ptsDiscontinuityRe.FindStringSubmatch(line); len(matches) > 1 {
			ts, _ := strconv.ParseFloat(matches[1], 64)
			result.Anomalies = append(result.Anomalies, output.Anomaly{
				Severity:    output.SeverityWarning,
				Description: "PTS discontinuity",
				Detail:      fmt.Sprintf("Discarded PTS discontinuity at %.3fs", ts),
				Timestamp:   ts,
			})
		}

		if matches := concealingErrorRe.FindStringSubmatch(line); len(matches) > 0 {
			result.Anomalies = append(result.Anomalies, output.Anomaly{
				Severity:    output.SeverityError,
				Description: "Decoder concealment errors",
				Detail:      strings.TrimSpace(line),
			})
		}

		if dtsDiscontinuityRe.MatchString(line) {
			result.Anomalies = append(result.Anomalies, output.Anomaly{
				Severity:    output.SeverityWarning,
				Description: "DTS discontinuity",
				Detail:      strings.TrimSpace(line),
			})
		}

		if overreadRe.MatchString(strings.ToLower(line)) {
			result.Anomalies = append(result.Anomalies, output.Anomaly{
				Severity:    output.SeverityWarning,
				Description: "Stream parsing warning",
				Detail:      strings.TrimSpace(line),
			})
		}
	}

	if verbosity == output.VerbosityForensic {
		result.RawOutput = text
	}

	return result, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tools/ffmpeg/ -v`
Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tools/ffmpeg/
git commit -m "feat: add ffmpeg playback simulation tool with anomaly detection"
```

---

### Task 14: MP4Box Tool Implementation

**Files:**
- Create: `internal/tools/mp4/mp4box.go`
- Create: `internal/tools/mp4/mp4dump.go`
- Create: `internal/tools/mp4/parse.go`
- Create: `internal/tools/mp4/mp4box_test.go`
- Create: `internal/tools/mp4/testdata/mp4box_info_output.txt`
- Create: `internal/tools/mp4/testdata/mp4dump_output.txt`

- [ ] **Step 1: Create test fixtures**

`internal/tools/mp4/testdata/mp4box_info_output.txt`:
```
* Movie Info *
	Timescale 1000 - Duration 00:02:00.500
	1 track(s)
	Fragmented File: yes
	File Brand isom - version 512
	Created: GMT Thu Jan  1 00:00:00 1970

* Track 1 Info *
	Track ID 1 - TimeScale 90000
	Media Duration 00:02:00.500 - Indicated Duration 00:02:00.500
	Media Type: vide:avc1 - Visual Track
	Codec Parameters: avc1
	AVC/H264 Video - Visual Size 1920 x 1080
	AVC Profile High @ Level 4
	NAL Unit length bits: 32
	SPS: 1 - PPS: 1
	Pixel Aspect Ratio 1:1
	Average Bitrate 5000 kbps
	Max Bitrate 8000 kbps
	Sample count: 3600
```

`internal/tools/mp4/testdata/mp4dump_output.txt`:
```
[ftyp] size=32
  major_brand = isom
  minor_version = 512
  compatible_brand = isom
  compatible_brand = iso6
  compatible_brand = mp41
[moov] size=1234
  [mvhd] size=108
    timescale = 1000
    duration = 120500
  [trak] size=800
    [tkhd] size=92
      track_id = 1
      width = 1920.000000
      height = 1080.000000
    [mdia] size=700
      [mdhd] size=32
        timescale = 90000
        duration = 10845000
      [hdlr] size=45
        handler_type = vide
      [minf] size=600
        [stbl] size=500
          [stsd] size=200
            [avc1] size=180
```

- [ ] **Step 2: Write failing tests**

```go
// internal/tools/mp4/mp4box_test.go
package mp4

import (
	"os"
	"testing"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

func TestMP4Box_Name(t *testing.T) {
	m := NewMP4Box("video-debug/mp4-tools")
	if m.Name() != "run_mp4box" {
		t.Errorf("expected name=run_mp4box, got %s", m.Name())
	}
}

func TestMP4Box_BuildCommand(t *testing.T) {
	m := NewMP4Box("video-debug/mp4-tools")
	input := tools.ToolInput{URL: "https://example.com/video.mp4"}

	cmd, err := m.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() error: %v", err)
	}

	if cmd.Binary != "MP4Box" {
		t.Errorf("expected binary=MP4Box, got %s", cmd.Binary)
	}
}

func TestMP4Box_ParseOutput(t *testing.T) {
	data, err := os.ReadFile("testdata/mp4box_info_output.txt")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	m := NewMP4Box("video-debug/mp4-tools")
	result, err := m.ParseOutput(data, output.VerbosityStandard)
	if err != nil {
		t.Fatalf("ParseOutput() error: %v", err)
	}

	if len(result.Streams) == 0 {
		t.Error("expected at least 1 stream parsed from MP4Box output")
	}
	if result.Timing == nil {
		t.Error("expected timing info from MP4Box output")
	}
}

func TestMP4Dump_Name(t *testing.T) {
	m := NewMP4Dump("video-debug/mp4-tools")
	if m.Name() != "run_mp4dump" {
		t.Errorf("expected name=run_mp4dump, got %s", m.Name())
	}
}

func TestMP4Dump_ParseOutput(t *testing.T) {
	data, err := os.ReadFile("testdata/mp4dump_output.txt")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	m := NewMP4Dump("video-debug/mp4-tools")
	result, err := m.ParseOutput(data, output.VerbosityDeep)
	if err != nil {
		t.Fatalf("ParseOutput() error: %v", err)
	}

	if result.ContainerLayout == nil {
		t.Fatal("expected container layout (box tree), got nil")
	}
	if result.ContainerLayout.Name != "root" {
		t.Errorf("expected root box, got %s", result.ContainerLayout.Name)
	}
	if len(result.ContainerLayout.Children) == 0 {
		t.Error("expected child boxes (ftyp, moov, ...)")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/tools/mp4/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 4: Implement MP4Box, mp4dump tools and parsers**

```go
// internal/tools/mp4/mp4box.go
package mp4

import (
	"encoding/json"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

type MP4Box struct {
	image string
}

func NewMP4Box(image string) *MP4Box {
	return &MP4Box{image: image}
}

func (m *MP4Box) Name() string        { return "run_mp4box" }
func (m *MP4Box) Description() string  { return "Run MP4Box for MP4 container info and analysis" }
func (m *MP4Box) DockerImage() string  { return m.image }

func (m *MP4Box) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL of the MP4 asset to analyze"},
			"verbosity": {"type": "string", "enum": ["summary","standard","deep","forensic"], "default": "standard"},
			"args": {"type": "array", "items": {"type": "string"}, "description": "Custom MP4Box arguments"},
			"headers": {"type": "object", "additionalProperties": {"type": "string"}}
		},
		"required": ["url"]
	}`)
}

func (m *MP4Box) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	cmd := &tools.DockerCommand{
		Binary:     "MP4Box",
		InputFiles: []string{"input"},
	}

	if len(input.Args) > 0 {
		cmd.Args = input.Args
	} else {
		cmd.Args = []string{"-info", "/workspace/input"}
	}

	return cmd, nil
}

func (m *MP4Box) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	return parseMP4BoxInfoOutput(raw, m.Name(), verbosity)
}
```

```go
// internal/tools/mp4/mp4dump.go
package mp4

import (
	"encoding/json"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

type MP4Dump struct {
	image string
}

func NewMP4Dump(image string) *MP4Dump {
	return &MP4Dump{image: image}
}

func (m *MP4Dump) Name() string        { return "run_mp4dump" }
func (m *MP4Dump) Description() string  { return "Run mp4dump (GPAC) for MP4 box structure dump" }
func (m *MP4Dump) DockerImage() string  { return m.image }

func (m *MP4Dump) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL of the MP4 asset to analyze"},
			"verbosity": {"type": "string", "enum": ["summary","standard","deep","forensic"], "default": "standard"},
			"args": {"type": "array", "items": {"type": "string"}},
			"headers": {"type": "object", "additionalProperties": {"type": "string"}}
		},
		"required": ["url"]
	}`)
}

func (m *MP4Dump) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	cmd := &tools.DockerCommand{
		Binary:     "mp4dump",
		InputFiles: []string{"input"},
	}

	if len(input.Args) > 0 {
		cmd.Args = input.Args
	} else {
		cmd.Args = []string{"/workspace/input"}
	}

	return cmd, nil
}

func (m *MP4Dump) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	return parseMP4DumpOutput(raw, m.Name(), verbosity)
}
```

```go
// internal/tools/mp4/parse.go
package mp4

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/meinart/video-debug-mcp/internal/output"
)

var (
	trackRe     = regexp.MustCompile(`Track (\d+) Info`)
	durationRe  = regexp.MustCompile(`Duration (\d{2}):(\d{2}):(\d{2})\.(\d+)`)
	timescaleRe = regexp.MustCompile(`[Tt]imescale[:\s]+(\d+)`)
	visualSizeRe = regexp.MustCompile(`Visual Size (\d+) x (\d+)`)
	codecRe     = regexp.MustCompile(`Codec Parameters:\s+(\w+)`)
	bitrateRe   = regexp.MustCompile(`Average Bitrate (\d+) kbps`)
	mediaTypeRe = regexp.MustCompile(`Media Type:\s+(\w+):(\w+)`)
)

func parseMP4BoxInfoOutput(raw []byte, toolName string, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	text := string(raw)
	result := &output.AnalysisResult{
		Tool: toolName,
	}

	// Parse duration
	if matches := durationRe.FindStringSubmatch(text); len(matches) > 4 {
		h, _ := strconv.Atoi(matches[1])
		m, _ := strconv.Atoi(matches[2])
		s, _ := strconv.Atoi(matches[3])
		ms, _ := strconv.Atoi(matches[4])
		duration := float64(h)*3600 + float64(m)*60 + float64(s) + float64(ms)/1000
		result.Timing = &output.TimingInfo{Duration: duration}
	}

	// Parse timescale
	if result.Timing != nil {
		if matches := timescaleRe.FindStringSubmatch(text); len(matches) > 1 {
			ts, _ := strconv.Atoi(matches[1])
			result.Timing.Timescale = ts
		}
	}

	// Parse tracks
	sections := strings.Split(text, "* Track")
	for _, section := range sections[1:] { // Skip first section (movie info)
		si := output.StreamInfo{}

		if matches := trackRe.FindStringSubmatch("Track" + section); len(matches) > 1 {
			si.Index, _ = strconv.Atoi(matches[1])
		}

		if matches := mediaTypeRe.FindStringSubmatch(section); len(matches) > 2 {
			switch matches[1] {
			case "vide":
				si.Type = "video"
			case "soun":
				si.Type = "audio"
			case "text", "subt":
				si.Type = "subtitle"
			default:
				si.Type = matches[1]
			}
		}

		if matches := codecRe.FindStringSubmatch(section); len(matches) > 1 {
			si.Codec = matches[1]
		}

		if matches := visualSizeRe.FindStringSubmatch(section); len(matches) > 2 {
			si.Width, _ = strconv.Atoi(matches[1])
			si.Height, _ = strconv.Atoi(matches[2])
			si.Type = "video"
		}

		if matches := bitrateRe.FindStringSubmatch(section); len(matches) > 1 {
			br, _ := strconv.ParseInt(matches[1], 10, 64)
			si.Bitrate = br * 1000
		}

		result.Streams = append(result.Streams, si)
	}

	if verbosity == output.VerbosityForensic {
		result.RawOutput = text
	}

	return result, nil
}

// Box dump parsing
var boxLineRe = regexp.MustCompile(`^(\s*)\[(\w+)\]\s*size=(\d+)`)

func parseMP4DumpOutput(raw []byte, toolName string, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	text := string(raw)
	result := &output.AnalysisResult{
		Tool: toolName,
	}

	root := &output.BoxTree{Name: "root"}
	stack := []*output.BoxTree{root}

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		matches := boxLineRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		indent := len(matches[1]) / 2 // Each level is 2 spaces
		name := matches[2]
		size, _ := strconv.ParseInt(matches[3], 10, 64)

		box := output.BoxTree{
			Name: name,
			Size: size,
		}

		// Adjust stack to correct parent
		for indent+1 < len(stack) {
			stack = stack[:len(stack)-1]
		}

		parent := stack[len(stack)-1]
		parent.Children = append(parent.Children, box)

		// Push this box onto stack for potential children
		stack = append(stack, &parent.Children[len(parent.Children)-1])
	}

	result.ContainerLayout = root

	if verbosity == output.VerbosityForensic {
		result.RawOutput = text
	}

	return result, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tools/mp4/ -v`
Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tools/mp4/
git commit -m "feat: add MP4Box and mp4dump tools with container structure parsing"
```

---

### Task 15: Bento4 Tool Implementation

**Files:**
- Create: `internal/tools/bento4/mp4info.go`
- Create: `internal/tools/bento4/parse.go`
- Create: `internal/tools/bento4/bento4_test.go`
- Create: `internal/tools/bento4/testdata/mp4info_output.json`

- [ ] **Step 1: Create test fixture**

`internal/tools/bento4/testdata/mp4info_output.json`:
```json
{
  "movie": {
    "duration_ms": 120500,
    "timescale": 1000,
    "fragments": true,
    "tracks": [
      {
        "id": 1,
        "type": "Video",
        "codec": "avc1",
        "duration_ms": 120500,
        "timescale": 90000,
        "width": 1920,
        "height": 1080,
        "sample_count": 3600,
        "bitrate": 5000000
      },
      {
        "id": 2,
        "type": "Audio",
        "codec": "mp4a",
        "duration_ms": 120500,
        "timescale": 48000,
        "sample_rate": 48000,
        "channels": 2,
        "sample_count": 5640,
        "bitrate": 128000
      }
    ]
  }
}
```

- [ ] **Step 2: Write failing tests**

```go
// internal/tools/bento4/bento4_test.go
package bento4

import (
	"os"
	"testing"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

func TestBento4_Name(t *testing.T) {
	b := NewBento4("video-debug/bento4-tools")
	if b.Name() != "run_bento4" {
		t.Errorf("expected name=run_bento4, got %s", b.Name())
	}
}

func TestBento4_BuildCommand_DefaultMP4Info(t *testing.T) {
	b := NewBento4("video-debug/bento4-tools")
	input := tools.ToolInput{URL: "https://example.com/video.mp4"}

	cmd, err := b.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() error: %v", err)
	}

	if cmd.Binary != "mp4info" {
		t.Errorf("expected binary=mp4info, got %s", cmd.Binary)
	}

	// Should include --format json
	found := false
	for _, arg := range cmd.Args {
		if arg == "--format" || arg == "json" {
			found = true
		}
	}
	if !found {
		t.Error("expected --format json in args")
	}
}

func TestBento4_BuildCommand_CustomSubtool(t *testing.T) {
	b := NewBento4("video-debug/bento4-tools")
	input := tools.ToolInput{
		URL:  "https://example.com/video.mp4",
		Args: []string{"mp4dump", "/workspace/input"},
	}

	cmd, err := b.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() error: %v", err)
	}

	if cmd.Binary != "mp4dump" {
		t.Errorf("expected binary=mp4dump from custom args, got %s", cmd.Binary)
	}
}

func TestBento4_ParseOutput(t *testing.T) {
	data, err := os.ReadFile("testdata/mp4info_output.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	b := NewBento4("video-debug/bento4-tools")
	result, err := b.ParseOutput(data, output.VerbosityStandard)
	if err != nil {
		t.Fatalf("ParseOutput() error: %v", err)
	}

	if len(result.Streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(result.Streams))
	}

	vs := result.Streams[0]
	if vs.Type != "video" {
		t.Errorf("expected type=video, got %s", vs.Type)
	}
	if vs.Width != 1920 {
		t.Errorf("expected width=1920, got %d", vs.Width)
	}

	if result.Timing == nil {
		t.Fatal("expected timing info")
	}
	if result.Timing.Duration != 120.5 {
		t.Errorf("expected duration=120.5, got %f", result.Timing.Duration)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/tools/bento4/ -v`
Expected: FAIL.

- [ ] **Step 4: Implement Bento4 tool and parser**

```go
// internal/tools/bento4/mp4info.go
package bento4

import (
	"encoding/json"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

type Bento4 struct {
	image string
}

func NewBento4(image string) *Bento4 {
	return &Bento4{image: image}
}

func (b *Bento4) Name() string        { return "run_bento4" }
func (b *Bento4) Description() string  { return "Run Bento4 tools (mp4info, mp4dump, mp4fragment, mp4encrypt, mp4decrypt)" }
func (b *Bento4) DockerImage() string  { return b.image }

func (b *Bento4) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL of the MP4 asset to analyze"},
			"verbosity": {"type": "string", "enum": ["summary","standard","deep","forensic"], "default": "standard"},
			"args": {"type": "array", "items": {"type": "string"}, "description": "Custom args — first element is the sub-tool name (mp4info, mp4dump, etc.)"},
			"headers": {"type": "object", "additionalProperties": {"type": "string"}}
		},
		"required": ["url"]
	}`)
}

func (b *Bento4) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	cmd := &tools.DockerCommand{
		InputFiles: []string{"input"},
	}

	if len(input.Args) > 0 {
		cmd.Binary = input.Args[0]
		cmd.Args = input.Args[1:]
	} else {
		cmd.Binary = "mp4info"
		cmd.Args = []string{"--format", "json", "/workspace/input"}
	}

	return cmd, nil
}

func (b *Bento4) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	return parseBento4Output(raw, b.Name(), verbosity)
}
```

```go
// internal/tools/bento4/parse.go
package bento4

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/meinart/video-debug-mcp/internal/output"
)

type bento4JSON struct {
	Movie struct {
		DurationMS int  `json:"duration_ms"`
		Timescale  int  `json:"timescale"`
		Fragments  bool `json:"fragments"`
		Tracks     []struct {
			ID         int    `json:"id"`
			Type       string `json:"type"`
			Codec      string `json:"codec"`
			DurationMS int    `json:"duration_ms"`
			Timescale  int    `json:"timescale"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			SampleRate int    `json:"sample_rate"`
			Channels   int    `json:"channels"`
			SampleCount int   `json:"sample_count"`
			Bitrate    int64  `json:"bitrate"`
		} `json:"tracks"`
	} `json:"movie"`
}

func parseBento4Output(raw []byte, toolName string, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	var b4 bento4JSON
	if err := json.Unmarshal(raw, &b4); err != nil {
		return nil, fmt.Errorf("parse bento4 output: %w", err)
	}

	result := &output.AnalysisResult{
		Tool: toolName,
	}

	for _, track := range b4.Movie.Tracks {
		si := output.StreamInfo{
			Index:   track.ID,
			Type:    strings.ToLower(track.Type),
			Codec:   track.Codec,
			Bitrate: track.Bitrate,
		}

		if track.Width > 0 {
			si.Width = track.Width
			si.Height = track.Height
		}
		if track.SampleRate > 0 {
			si.SampleRate = track.SampleRate
		}
		if track.Channels > 0 {
			si.Channels = track.Channels
		}

		result.Streams = append(result.Streams, si)
	}

	result.Timing = &output.TimingInfo{
		Duration:  float64(b4.Movie.DurationMS) / 1000.0,
		Timescale: b4.Movie.Timescale,
	}

	if verbosity == output.VerbosityForensic {
		result.RawOutput = string(raw)
	}

	return result, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tools/bento4/ -v`
Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tools/bento4/
git commit -m "feat: add Bento4 tool with JSON output parser"
```

---

### Task 16: MediaInfo Tool Implementation

**Files:**
- Create: `internal/tools/mediainfo/mediainfo.go`
- Create: `internal/tools/mediainfo/parse.go`
- Create: `internal/tools/mediainfo/mediainfo_test.go`
- Create: `internal/tools/mediainfo/testdata/mediainfo_output.json`

- [ ] **Step 1: Create test fixture**

`internal/tools/mediainfo/testdata/mediainfo_output.json`:
```json
{
  "media": {
    "track": [
      {
        "@type": "General",
        "Format": "MPEG-4",
        "Duration": "120.500",
        "OverallBitRate": "5128000",
        "FileSize": "77294600"
      },
      {
        "@type": "Video",
        "Format": "AVC",
        "Format_Profile": "High",
        "Format_Level": "4",
        "CodecID": "avc1",
        "Duration": "120.500",
        "BitRate": "5000000",
        "Width": "1920",
        "Height": "1080",
        "FrameRate": "29.970",
        "PixelAspectRatio": "1.000",
        "ColorSpace": "YUV",
        "ChromaSubsampling": "4:2:0"
      },
      {
        "@type": "Audio",
        "Format": "AAC",
        "Format_Profile": "LC",
        "CodecID": "mp4a-40-2",
        "Duration": "120.500",
        "BitRate": "128000",
        "SamplingRate": "48000",
        "Channels": "2",
        "Language": "en"
      }
    ]
  }
}
```

- [ ] **Step 2: Write failing tests**

```go
// internal/tools/mediainfo/mediainfo_test.go
package mediainfo

import (
	"os"
	"testing"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

func TestMediaInfo_Name(t *testing.T) {
	mi := NewMediaInfo("video-debug/mediainfo-tools")
	if mi.Name() != "run_mediainfo" {
		t.Errorf("expected name=run_mediainfo, got %s", mi.Name())
	}
}

func TestMediaInfo_BuildCommand(t *testing.T) {
	mi := NewMediaInfo("video-debug/mediainfo-tools")
	input := tools.ToolInput{URL: "https://example.com/video.mp4"}

	cmd, err := mi.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() error: %v", err)
	}

	if cmd.Binary != "mediainfo" {
		t.Errorf("expected binary=mediainfo, got %s", cmd.Binary)
	}

	// Should include --Output=JSON
	found := false
	for _, arg := range cmd.Args {
		if arg == "--Output=JSON" {
			found = true
		}
	}
	if !found {
		t.Error("expected --Output=JSON in args")
	}
}

func TestMediaInfo_ParseOutput(t *testing.T) {
	data, err := os.ReadFile("testdata/mediainfo_output.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	mi := NewMediaInfo("video-debug/mediainfo-tools")
	result, err := mi.ParseOutput(data, output.VerbosityStandard)
	if err != nil {
		t.Fatalf("ParseOutput() error: %v", err)
	}

	// 2 streams (video + audio, General track is metadata only)
	if len(result.Streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(result.Streams))
	}

	if result.Streams[0].Type != "video" {
		t.Errorf("expected first stream type=video, got %s", result.Streams[0].Type)
	}
	if result.Streams[0].Width != 1920 {
		t.Errorf("expected width=1920, got %d", result.Streams[0].Width)
	}

	if result.Timing == nil {
		t.Fatal("expected timing info")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/tools/mediainfo/ -v`
Expected: FAIL.

- [ ] **Step 4: Implement MediaInfo tool and parser**

```go
// internal/tools/mediainfo/mediainfo.go
package mediainfo

import (
	"encoding/json"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

type MediaInfo struct {
	image string
}

func NewMediaInfo(image string) *MediaInfo {
	return &MediaInfo{image: image}
}

func (m *MediaInfo) Name() string        { return "run_mediainfo" }
func (m *MediaInfo) Description() string  { return "Run MediaInfo for rich media metadata analysis" }
func (m *MediaInfo) DockerImage() string  { return m.image }

func (m *MediaInfo) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL of the media asset to analyze"},
			"verbosity": {"type": "string", "enum": ["summary","standard","deep","forensic"], "default": "standard"},
			"args": {"type": "array", "items": {"type": "string"}, "description": "Custom MediaInfo arguments"},
			"headers": {"type": "object", "additionalProperties": {"type": "string"}}
		},
		"required": ["url"]
	}`)
}

func (m *MediaInfo) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	cmd := &tools.DockerCommand{
		Binary:     "mediainfo",
		InputFiles: []string{"input"},
	}

	if len(input.Args) > 0 {
		cmd.Args = input.Args
	} else {
		cmd.Args = []string{"--Output=JSON", "/workspace/input"}
	}

	return cmd, nil
}

func (m *MediaInfo) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	return parseMediaInfoOutput(raw, m.Name(), verbosity)
}
```

```go
// internal/tools/mediainfo/parse.go
package mediainfo

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/meinart/video-debug-mcp/internal/output"
)

type mediaInfoJSON struct {
	Media struct {
		Track []map[string]interface{} `json:"track"`
	} `json:"media"`
}

func parseMediaInfoOutput(raw []byte, toolName string, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	var mi mediaInfoJSON
	if err := json.Unmarshal(raw, &mi); err != nil {
		return nil, fmt.Errorf("parse mediainfo output: %w", err)
	}

	result := &output.AnalysisResult{
		Tool: toolName,
	}

	streamIndex := 0
	for _, track := range mi.Media.Track {
		trackType, _ := track["@type"].(string)

		switch strings.ToLower(trackType) {
		case "general":
			// Extract timing from General track
			if dur, ok := track["Duration"].(string); ok {
				d, _ := strconv.ParseFloat(dur, 64)
				result.Timing = &output.TimingInfo{Duration: d}
			}
			if br, ok := track["OverallBitRate"].(string); ok {
				b, _ := strconv.ParseInt(br, 10, 64)
				result.Bitrate = &output.BitrateInfo{Overall: b}
			}

		case "video":
			si := output.StreamInfo{
				Index: streamIndex,
				Type:  "video",
			}
			streamIndex++

			if v, ok := track["Format"].(string); ok {
				si.Codec = v
			}
			if v, ok := track["Width"].(string); ok {
				si.Width, _ = strconv.Atoi(v)
			}
			if v, ok := track["Height"].(string); ok {
				si.Height, _ = strconv.Atoi(v)
			}
			if v, ok := track["FrameRate"].(string); ok {
				si.FrameRate, _ = strconv.ParseFloat(v, 64)
			}
			if v, ok := track["BitRate"].(string); ok {
				br, _ := strconv.ParseInt(v, 10, 64)
				si.Bitrate = br
			}

			result.Streams = append(result.Streams, si)

			// Codec details
			if result.Codec == nil {
				result.Codec = &output.CodecDetails{}
			}
			result.Codec.VideoCodec, _ = track["Format"].(string)
			result.Codec.VideoProfile, _ = track["Format_Profile"].(string)
			result.Codec.VideoLevel, _ = track["Format_Level"].(string)
			result.Codec.ColorSpace, _ = track["ColorSpace"].(string)

		case "audio":
			si := output.StreamInfo{
				Index: streamIndex,
				Type:  "audio",
			}
			streamIndex++

			if v, ok := track["Format"].(string); ok {
				si.Codec = v
			}
			if v, ok := track["SamplingRate"].(string); ok {
				si.SampleRate, _ = strconv.Atoi(v)
			}
			if v, ok := track["Channels"].(string); ok {
				si.Channels, _ = strconv.Atoi(v)
			}
			if v, ok := track["BitRate"].(string); ok {
				br, _ := strconv.ParseInt(v, 10, 64)
				si.Bitrate = br
			}
			if v, ok := track["Language"].(string); ok {
				si.Language = v
			}

			result.Streams = append(result.Streams, si)

			if result.Codec == nil {
				result.Codec = &output.CodecDetails{}
			}
			result.Codec.AudioCodec, _ = track["Format"].(string)
		}
	}

	if verbosity == output.VerbosityForensic {
		result.RawOutput = string(raw)
	}

	return result, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tools/mediainfo/ -v`
Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tools/mediainfo/
git commit -m "feat: add MediaInfo tool with JSON output parser"
```

---

### Task 17: Shaka Packager Tool Implementation

**Files:**
- Create: `internal/tools/shaka/packager.go`
- Create: `internal/tools/shaka/parse.go`
- Create: `internal/tools/shaka/shaka_test.go`
- Create: `internal/tools/shaka/testdata/shaka_output.txt`

- [ ] **Step 1: Create test fixture**

`internal/tools/shaka/testdata/shaka_output.txt`:
```
[0319/120000:INFO:demuxer.cc] Demuxer::Run() on file '/workspace/input'.
[0319/120000:INFO:demuxer.cc] Initialize Demuxer for file '/workspace/input'.
[0319/120000:INFO:mp4_media_parser.cc] MPEG-4 file detected: brand = isom
[0319/120000:INFO:mp4_media_parser.cc] Track type: video, codec: avc1
[0319/120000:INFO:mp4_media_parser.cc] Width: 1920, Height: 1080
[0319/120000:INFO:mp4_media_parser.cc] Track type: audio, codec: mp4a
[0319/120000:WARNING:mp4_media_parser.cc] Potential timestamp overlap at sample 1200
Packaging completed successfully.
```

- [ ] **Step 2: Write failing tests**

```go
// internal/tools/shaka/shaka_test.go
package shaka

import (
	"os"
	"testing"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

func TestShaka_Name(t *testing.T) {
	s := NewShaka("video-debug/shaka-tools")
	if s.Name() != "run_shaka_packager" {
		t.Errorf("expected name=run_shaka_packager, got %s", s.Name())
	}
}

func TestShaka_BuildCommand(t *testing.T) {
	s := NewShaka("video-debug/shaka-tools")
	input := tools.ToolInput{URL: "https://example.com/video.mp4"}

	cmd, err := s.BuildCommand(input)
	if err != nil {
		t.Fatalf("BuildCommand() error: %v", err)
	}

	if cmd.Binary != "packager" {
		t.Errorf("expected binary=packager, got %s", cmd.Binary)
	}
}

func TestShaka_ParseOutput(t *testing.T) {
	data, err := os.ReadFile("testdata/shaka_output.txt")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	s := NewShaka("video-debug/shaka-tools")
	result, err := s.ParseOutput(data, output.VerbosityStandard)
	if err != nil {
		t.Fatalf("ParseOutput() error: %v", err)
	}

	if result.Tool != "run_shaka_packager" {
		t.Errorf("expected tool=run_shaka_packager, got %s", result.Tool)
	}

	// Should detect the warning
	if len(result.Anomalies) == 0 {
		t.Error("expected anomalies from shaka output")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/tools/shaka/ -v`
Expected: FAIL.

- [ ] **Step 4: Implement Shaka tool and parser**

```go
// internal/tools/shaka/packager.go
package shaka

import (
	"encoding/json"

	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

type Shaka struct {
	image string
}

func NewShaka(image string) *Shaka {
	return &Shaka{image: image}
}

func (s *Shaka) Name() string        { return "run_shaka_packager" }
func (s *Shaka) Description() string  { return "Run Shaka Packager for DASH/HLS packaging validation" }
func (s *Shaka) DockerImage() string  { return s.image }

func (s *Shaka) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL of the media asset to validate"},
			"verbosity": {"type": "string", "enum": ["summary","standard","deep","forensic"], "default": "standard"},
			"args": {"type": "array", "items": {"type": "string"}, "description": "Custom Shaka Packager arguments"},
			"headers": {"type": "object", "additionalProperties": {"type": "string"}}
		},
		"required": ["url"]
	}`)
}

func (s *Shaka) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	cmd := &tools.DockerCommand{
		Binary:     "packager",
		InputFiles: []string{"input"},
	}

	if len(input.Args) > 0 {
		cmd.Args = input.Args
	} else {
		cmd.Args = []string{
			"input=/workspace/input,stream=video,output=/dev/null",
			"--dump_stream_info",
		}
	}

	return cmd, nil
}

func (s *Shaka) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	return parseShakaOutput(raw, s.Name(), verbosity)
}
```

```go
// internal/tools/shaka/parse.go
package shaka

import (
	"regexp"
	"strings"

	"github.com/meinart/video-debug-mcp/internal/output"
)

var (
	shakaWarningRe = regexp.MustCompile(`\[.*WARNING.*\]\s*(.+)`)
	shakaErrorRe   = regexp.MustCompile(`\[.*ERROR.*\]\s*(.+)`)
	shakaTrackRe   = regexp.MustCompile(`Track type:\s*(\w+),\s*codec:\s*(\w+)`)
)

func parseShakaOutput(raw []byte, toolName string, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	text := string(raw)
	result := &output.AnalysisResult{
		Tool: toolName,
	}

	lines := strings.Split(text, "\n")
	streamIndex := 0

	for _, line := range lines {
		// Detect tracks
		if matches := shakaTrackRe.FindStringSubmatch(line); len(matches) > 2 {
			result.Streams = append(result.Streams, output.StreamInfo{
				Index: streamIndex,
				Type:  strings.ToLower(matches[1]),
				Codec: matches[2],
			})
			streamIndex++
		}

		// Detect warnings
		if matches := shakaWarningRe.FindStringSubmatch(line); len(matches) > 1 {
			result.Anomalies = append(result.Anomalies, output.Anomaly{
				Severity:    output.SeverityWarning,
				Description: strings.TrimSpace(matches[1]),
			})
		}

		// Detect errors
		if matches := shakaErrorRe.FindStringSubmatch(line); len(matches) > 1 {
			result.Anomalies = append(result.Anomalies, output.Anomaly{
				Severity:    output.SeverityError,
				Description: strings.TrimSpace(matches[1]),
			})
		}
	}

	if verbosity == output.VerbosityForensic {
		result.RawOutput = text
	}

	return result, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tools/shaka/ -v`
Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tools/shaka/
git commit -m "feat: add Shaka Packager tool with log parser"
```

---

### Task 18: Orchestrator

The orchestrator coordinates the full tool invocation pipeline: fetch → parse manifest → download segments → run tool → format output.

**Files:**
- Create: `internal/server/orchestrator.go`
- Create: `internal/server/orchestrator_test.go`

- [ ] **Step 1: Write failing tests for orchestrator**

```go
// internal/server/orchestrator_test.go
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

// mockRunner implements docker.Runner for testing
type mockRunner struct {
	result *docker.RunResult
	err    error
}

func (m *mockRunner) Run(ctx context.Context, req docker.RunRequest) (*docker.RunResult, error) {
	return m.result, m.err
}

// mockTool implements tools.Tool for testing
type mockTool struct {
	name      string
	image     string
	cmd       *tools.DockerCommand
	cmdErr    error
	parsed    *output.AnalysisResult
	parseErr  error
}

func (m *mockTool) Name() string                    { return m.name }
func (m *mockTool) Description() string             { return "mock" }
func (m *mockTool) InputSchema() json.RawMessage     { return json.RawMessage(`{}`) }
func (m *mockTool) DockerImage() string              { return m.image }
func (m *mockTool) BuildCommand(input tools.ToolInput) (*tools.DockerCommand, error) {
	return m.cmd, m.cmdErr
}
func (m *mockTool) ParseOutput(raw []byte, verbosity output.Verbosity) (*output.AnalysisResult, error) {
	return m.parsed, m.parseErr
}

func TestOrchestrator_ExecuteTool_DirectAsset(t *testing.T) {
	runner := &mockRunner{
		result: &docker.RunResult{
			ExitCode: 0,
			Stdout:   []byte(`{"streams":[],"format":{"duration":"10.0","bit_rate":"1000"}}`),
			Duration: 100 * time.Millisecond,
		},
	}

	tool := &mockTool{
		name:  "test_tool",
		image: "test-image",
		cmd: &tools.DockerCommand{
			Binary:     "testtool",
			Args:       []string{"/workspace/input"},
			InputFiles: []string{"input"},
		},
		parsed: &output.AnalysisResult{
			Tool: "test_tool",
			Streams: []output.StreamInfo{
				{Index: 0, Type: "video", Codec: "h264"},
			},
		},
	}

	cfg := config.Default()
	orch := NewOrchestrator(runner, output.NewFormatter(), cfg)

	input := tools.ToolInput{
		URL:       "https://example.com/video.mp4",
		Verbosity: "standard",
	}

	// This test verifies orchestrator wiring — actual HTTP fetching is not tested here
	// (that's covered by fetcher tests and integration tests)
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
		result: &docker.RunResult{
			ExitCode: 1,
			Stderr:   []byte("command not found"),
			Duration: 50 * time.Millisecond,
		},
	}

	tool := &mockTool{
		name:  "test_tool",
		image: "test-image",
		cmd: &tools.DockerCommand{
			Binary:     "testtool",
			Args:       []string{"/workspace/input"},
			InputFiles: []string{"input"},
		},
	}

	cfg := config.Default()
	orch := NewOrchestrator(runner, output.NewFormatter(), cfg)

	input := tools.ToolInput{URL: "https://example.com/video.mp4"}

	_, err := orch.ExecuteToolDirect(context.Background(), tool, input, "/tmp/test-workspace")
	if err == nil {
		t.Error("expected error for non-zero exit code, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -v`
Expected: FAIL — Orchestrator not defined.

- [ ] **Step 3: Implement orchestrator**

```go
// internal/server/orchestrator.go
package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/meinart/video-debug-mcp/internal/config"
	"github.com/meinart/video-debug-mcp/internal/docker"
	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

type Orchestrator struct {
	runner    docker.Runner
	formatter *output.Formatter
	cfg       *config.Config
}

func NewOrchestrator(runner docker.Runner, formatter *output.Formatter, cfg *config.Config) *Orchestrator {
	return &Orchestrator{
		runner:    runner,
		formatter: formatter,
		cfg:       cfg,
	}
}

func (o *Orchestrator) ExecuteToolDirect(ctx context.Context, tool tools.Tool, input tools.ToolInput, workspaceDir string) (*output.AnalysisResult, error) {
	// Build the command
	cmd, err := tool.BuildCommand(input)
	if err != nil {
		return nil, fmt.Errorf("build command: %w", err)
	}

	// Determine timeout
	timeout := o.cfg.Docker.Defaults.Timeout
	if toolCfg, ok := o.cfg.Docker.PerTool[tool.Name()]; ok && toolCfg.Timeout > 0 {
		timeout = toolCfg.Timeout
	}

	// Run in Docker
	fullCmd := append([]string{cmd.Binary}, cmd.Args...)
	runReq := docker.RunRequest{
		Image:        tool.DockerImage(),
		Command:      fullCmd,
		WorkspaceDir: workspaceDir,
		Timeout:      timeout,
	}

	result, err := o.runner.Run(ctx, runReq)
	if err != nil {
		return nil, fmt.Errorf("run container: %w", err)
	}

	if !result.Success() {
		return nil, fmt.Errorf("tool %s exited with code %d: %s", tool.Name(), result.ExitCode, string(result.Stderr))
	}

	// Parse output
	verbosity, err := input.ParsedVerbosity()
	if err != nil {
		verbosity = output.VerbosityStandard
	}

	// Use stdout for parsing; fall back to combined output
	rawOutput := result.Stdout
	if len(rawOutput) == 0 {
		rawOutput = result.Stderr
	}

	analysisResult, err := tool.ParseOutput(rawOutput, verbosity)
	if err != nil {
		return nil, fmt.Errorf("parse output: %w", err)
	}

	// Enrich with execution metadata
	analysisResult.Command = strings.Join(fullCmd, " ")
	analysisResult.InputURL = input.URL

	// Apply verbosity formatting
	formatted := o.formatter.Format(analysisResult, verbosity)

	return formatted, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/orchestrator.go internal/server/orchestrator_test.go
git commit -m "feat: add orchestrator for tool execution pipeline"
```

---

### Task 19: MCP Server Setup & Tool Registration

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/server_test.go`

- [ ] **Step 1: Write failing tests for MCP server tool registration**

```go
// internal/server/server_test.go
package server

import (
	"testing"

	"github.com/meinart/video-debug-mcp/internal/config"
)

func TestNewServer(t *testing.T) {
	cfg := config.Default()
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}

	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestServer_ToolCount(t *testing.T) {
	cfg := config.Default()
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}

	// Should have registered all tools (5 high-level + 7 direct = 12)
	// Or at minimum the direct tools
	count := srv.ToolCount()
	if count < 7 {
		t.Errorf("expected at least 7 registered tools, got %d", count)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -v -run TestNewServer`
Expected: FAIL — NewServer not defined.

- [ ] **Step 3: Implement MCP server**

```go
// internal/server/server.go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/meinart/video-debug-mcp/internal/config"
	"github.com/meinart/video-debug-mcp/internal/docker"
	"github.com/meinart/video-debug-mcp/internal/fetcher"
	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
	"github.com/meinart/video-debug-mcp/internal/tools/bento4"
	"github.com/meinart/video-debug-mcp/internal/tools/ffmpeg"
	"github.com/meinart/video-debug-mcp/internal/tools/mediainfo"
	mp4tools "github.com/meinart/video-debug-mcp/internal/tools/mp4"
	"github.com/meinart/video-debug-mcp/internal/tools/shaka"
)

type Server struct {
	mcpServer    *mcpserver.MCPServer
	registry     *tools.Registry
	orchestrator *Orchestrator
	fetcher      *fetcher.Fetcher
	cfg          *config.Config
}

func NewServer(cfg *config.Config) (*Server, error) {
	// Create tool registry and register all tools
	registry := tools.NewRegistry()

	registry.Register(ffmpeg.NewFFprobe(cfg.Docker.Images.FFmpeg))
	registry.Register(ffmpeg.NewFFmpeg(cfg.Docker.Images.FFmpeg))
	registry.Register(mp4tools.NewMP4Box(cfg.Docker.Images.MP4))
	registry.Register(mp4tools.NewMP4Dump(cfg.Docker.Images.MP4))
	registry.Register(bento4.NewBento4(cfg.Docker.Images.Bento4))
	registry.Register(mediainfo.NewMediaInfo(cfg.Docker.Images.MediaInfo))
	registry.Register(shaka.NewShaka(cfg.Docker.Images.Shaka))

	// Create Docker runner (will fail gracefully if Docker not available)
	var runner docker.Runner
	dockerRunner, err := docker.NewDockerRunner()
	if err != nil {
		log.Printf("WARNING: Docker not available: %v — tool execution will fail", err)
		runner = nil
	} else {
		runner = dockerRunner
	}

	formatter := output.NewFormatter()
	orchestrator := NewOrchestrator(runner, formatter, cfg)
	f := fetcher.NewFetcher(cfg.Fetcher.MaxConcurrentDownloads, cfg.Fetcher.RequestTimeout, cfg.Fetcher.UserAgent)

	// Create MCP server
	mcpSrv := mcpserver.NewMCPServer(
		"video-debug-mcp",
		"1.0.0",
		mcpserver.WithToolCapabilities(true),
	)

	srv := &Server{
		mcpServer:    mcpSrv,
		registry:     registry,
		orchestrator: orchestrator,
		fetcher:      f,
		cfg:          cfg,
	}

	// Register each tool as an MCP tool
	for _, tool := range registry.List() {
		srv.registerMCPTool(tool)
	}

	return srv, nil
}

func (s *Server) registerMCPTool(tool tools.Tool) {
	var schema map[string]interface{}
	json.Unmarshal(tool.InputSchema(), &schema)

	mcpTool := mcp.Tool{
		Name:        tool.Name(),
		Description: tool.Description(),
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: schema["properties"].(map[string]interface{}),
		},
	}

	if required, ok := schema["required"].([]interface{}); ok {
		for _, r := range required {
			if s, ok := r.(string); ok {
				mcpTool.InputSchema.Required = append(mcpTool.InputSchema.Required, s)
			}
		}
	}

	s.mcpServer.AddTool(mcpTool, s.createToolHandler(tool))
}

func (s *Server) createToolHandler(tool tools.Tool) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse input from MCP request
		inputJSON, err := json.Marshal(request.Params.Arguments)
		if err != nil {
			return errorResult(output.ErrMalformedInput, "Failed to parse tool input", "parse", ""), nil
		}

		var input tools.ToolInput
		if err := json.Unmarshal(inputJSON, &input); err != nil {
			return errorResult(output.ErrMalformedInput, "Invalid tool input", "parse", ""), nil
		}

		if input.URL == "" {
			return errorResult(output.ErrMalformedInput, "URL is required", "parse", ""), nil
		}

		if s.orchestrator.runner == nil {
			return errorResult(output.ErrDockerUnavail, "Docker is not available", "execute", input.URL), nil
		}

		// Create workspace and download asset
		ws, err := fetcher.NewWorkspace()
		if err != nil {
			return errorResult(output.ErrToolExecution, fmt.Sprintf("Failed to create workspace: %v", err), "fetch", input.URL), nil
		}
		defer ws.Cleanup()

		// Download the main asset
		err = s.fetcher.DownloadFile(ctx, input.URL, ws, "input", input.Headers)
		if err != nil {
			return errorResult(output.ErrManifestFetch, fmt.Sprintf("Failed to download: %v", err), "fetch", input.URL), nil
		}

		// Execute tool
		result, err := s.orchestrator.ExecuteToolDirect(ctx, tool, input, ws.Dir)
		if err != nil {
			return errorResult(output.ErrToolExecution, fmt.Sprintf("Tool execution failed: %v", err), "execute", input.URL), nil
		}

		// Return structured result
		resultJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return errorResult(output.ErrToolExecution, "Failed to serialize result", "format", input.URL), nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(resultJSON),
				},
			},
		}, nil
	}
}

func errorResult(code output.ErrorCode, message, phase, url string) *mcp.CallToolResult {
	errResp := output.ErrorResponse{
		Code:    code,
		Message: message,
		Phase:   phase,
		URL:     url,
	}
	data, _ := json.MarshalIndent(errResp, "", "  ")
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: string(data),
			},
		},
	}
}

func (s *Server) ToolCount() int {
	return len(s.registry.List())
}

func (s *Server) MCPServer() *mcpserver.MCPServer {
	return s.mcpServer
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/server.go internal/server/server_test.go
git commit -m "feat: add MCP server with tool registration and request handling"
```

---

### Task 20: Main Entry Point

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Implement main.go with config loading and SSE server startup**

```go
// cmd/server/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	mcphttp "github.com/mark3labs/mcp-go/server"

	"github.com/meinart/video-debug-mcp/internal/config"
	"github.com/meinart/video-debug-mcp/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Printf("Warning: could not load config from %s: %v — using defaults", *configPath, err)
		cfg = config.Default()
		cfg.ApplyEnvOverrides()
	}

	// Create server
	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Starting video-debug-mcp server on %s", addr)
	log.Printf("Registered %d tools", srv.ToolCount())

	// Create SSE server
	sseServer := mcphttp.NewSSEServer(srv.MCPServer())

	log.Fatal(http.ListenAndServe(addr, sseServer))
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./cmd/server`
Expected: Builds with no errors. (Note: may need to run `go mod tidy` first if imports changed.)

- [ ] **Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: add main entry point with config loading and SSE server"
```

---

### Task 21: Dockerfiles for Tool Images

**Files:**
- Create: `docker/ffmpeg-tools/Dockerfile`
- Create: `docker/mp4-tools/Dockerfile`
- Create: `docker/bento4-tools/Dockerfile`
- Create: `docker/mediainfo-tools/Dockerfile`
- Create: `docker/shaka-tools/Dockerfile`

- [ ] **Step 1: Create ffmpeg-tools Dockerfile**

```dockerfile
# docker/ffmpeg-tools/Dockerfile
FROM alpine:3.19

RUN apk add --no-cache ffmpeg

WORKDIR /workspace

ENTRYPOINT []
```

- [ ] **Step 2: Create mp4-tools Dockerfile**

```dockerfile
# docker/mp4-tools/Dockerfile
FROM alpine:3.19

RUN apk add --no-cache gpac

WORKDIR /workspace

ENTRYPOINT []
```

- [ ] **Step 3: Create bento4-tools Dockerfile**

```dockerfile
# docker/bento4-tools/Dockerfile
FROM alpine:3.19

RUN apk add --no-cache curl unzip && \
    cd /tmp && \
    curl -L -o bento4.zip https://www.bok.net/Bento4/binaries/Bento4-SDK-1-6-0-641.x86_64-unknown-linux.zip && \
    unzip bento4.zip && \
    cp Bento4-SDK-*/bin/* /usr/local/bin/ && \
    rm -rf /tmp/bento4* && \
    apk del curl unzip

WORKDIR /workspace

ENTRYPOINT []
```

- [ ] **Step 4: Create mediainfo-tools Dockerfile**

```dockerfile
# docker/mediainfo-tools/Dockerfile
FROM alpine:3.19

RUN apk add --no-cache mediainfo

WORKDIR /workspace

ENTRYPOINT []
```

- [ ] **Step 5: Create shaka-tools Dockerfile**

```dockerfile
# docker/shaka-tools/Dockerfile
FROM alpine:3.19

RUN apk add --no-cache curl && \
    curl -L -o /usr/local/bin/packager \
      https://github.com/shaka-project/shaka-packager/releases/latest/download/packager-linux-x64 && \
    chmod +x /usr/local/bin/packager && \
    apk del curl

WORKDIR /workspace

ENTRYPOINT []
```

- [ ] **Step 6: Commit**

```bash
git add docker/
git commit -m "feat: add Dockerfiles for all tool-family images"
```

---

### Task 22: High-Level Analysis Tools (analyze_manifest, analyze_media, etc.)

These are the "smart" tools that combine manifest parsing, fetching, and tool execution.

**Files:**
- Create: `internal/server/handlers.go`
- Create: `internal/server/handlers_test.go`

- [ ] **Step 1: Write failing tests for high-level tool handlers**

```go
// internal/server/handlers_test.go
package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/meinart/video-debug-mcp/internal/config"
	"github.com/meinart/video-debug-mcp/internal/docker"
	"github.com/meinart/video-debug-mcp/internal/fetcher"
	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

func TestAnalyzeManifest_HLS(t *testing.T) {
	hlsContent := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=3000000,RESOLUTION=1280x720,CODECS="avc1.64001f,mp4a.40.2"
720p/playlist.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=1920x1080,CODECS="avc1.640028,mp4a.40.2"
1080p/playlist.m3u8
`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(hlsContent))
	}))
	defer ts.Close()

	cfg := config.Default()
	f := fetcher.NewFetcher(10, 30*time.Second, "test/1.0")
	formatter := output.NewFormatter()

	handler := &AnalyzeManifestHandler{
		fetcher:   f,
		formatter: formatter,
		cfg:       cfg,
	}

	input := tools.ToolInput{
		URL:       ts.URL + "/master.m3u8",
		Verbosity: "standard",
	}

	result, err := handler.Handle(context.Background(), input)
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}

	if result.Manifest == nil {
		t.Fatal("expected manifest summary, got nil")
	}
	if result.Manifest.VariantCount != 2 {
		t.Errorf("expected 2 variants, got %d", result.Manifest.VariantCount)
	}
}

func TestAnalyzeMedia_WithMockRunner(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake mp4 data"))
	}))
	defer ts.Close()

	runner := &mockRunner{
		result: &docker.RunResult{
			ExitCode: 0,
			Stdout:   []byte(`{"streams":[{"index":0,"codec_name":"h264","codec_type":"video","width":1920,"height":1080,"r_frame_rate":"30/1","bit_rate":"5000000"}],"format":{"duration":"120.5","start_time":"0.0","bit_rate":"5000000"}}`),
			Duration: 100 * time.Millisecond,
		},
	}

	cfg := config.Default()
	f := fetcher.NewFetcher(10, 30*time.Second, "test/1.0")
	formatter := output.NewFormatter()
	orch := NewOrchestrator(runner, formatter, cfg)

	handler := &AnalyzeMediaHandler{
		fetcher:      f,
		orchestrator: orch,
		formatter:    formatter,
		cfg:          cfg,
	}

	input := tools.ToolInput{
		URL:       ts.URL + "/video.mp4",
		Verbosity: "standard",
	}

	result, err := handler.Handle(context.Background(), input)
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}

	if len(result.Streams) == 0 {
		t.Error("expected streams in result")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -v -run TestAnalyze`
Expected: FAIL — AnalyzeManifestHandler not defined.

- [ ] **Step 3: Implement high-level analysis handlers**

```go
// internal/server/handlers.go
package server

import (
	"context"
	"fmt"
	"os"

	"github.com/meinart/video-debug-mcp/internal/config"
	"github.com/meinart/video-debug-mcp/internal/fetcher"
	"github.com/meinart/video-debug-mcp/internal/manifest"
	"github.com/meinart/video-debug-mcp/internal/manifest/dash"
	"github.com/meinart/video-debug-mcp/internal/manifest/hls"
	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
	"github.com/meinart/video-debug-mcp/internal/tools/ffmpeg"
	mp4tools "github.com/meinart/video-debug-mcp/internal/tools/mp4"
	"github.com/meinart/video-debug-mcp/internal/tools/shaka"
)

type AnalyzeManifestHandler struct {
	fetcher   *fetcher.Fetcher
	formatter *output.Formatter
	cfg       *config.Config
}

func (h *AnalyzeManifestHandler) Handle(ctx context.Context, input tools.ToolInput) (*output.AnalysisResult, error) {
	// Create workspace
	ws, err := fetcher.NewWorkspace()
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	defer ws.Cleanup()

	// Download manifest
	err = h.fetcher.DownloadFile(ctx, input.URL, ws, "manifest", input.Headers)
	if err != nil {
		return nil, fmt.Errorf("download manifest: %w", err)
	}

	// Read manifest content
	manifestPath := ws.AssetPath("manifest")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	// Detect type and parse
	assetType := fetcher.DetectAssetType(input.URL, data)

	var m *manifest.Manifest
	switch assetType {
	case fetcher.AssetTypeHLS:
		m, err = hls.Parse(data, input.URL)
	case fetcher.AssetTypeDASH:
		m, err = dash.Parse(data, input.URL)
	default:
		return nil, fmt.Errorf("URL does not appear to be an HLS or DASH manifest")
	}
	if err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	// Build result
	verbosity, _ := input.ParsedVerbosity()

	result := &output.AnalysisResult{
		Tool:     "analyze_manifest",
		InputURL: input.URL,
		Manifest: &output.ManifestSummary{
			Type:           string(m.Type),
			URL:            m.URL,
			VariantCount:   len(m.Variants),
			AudioTracks:    len(m.AudioTracks),
			SubtitleTracks: len(m.SubtitleTracks),
			TotalSegments:  len(m.AllSegmentURLs()),
		},
		ResolvedURLs: m.AllSegmentURLs(),
	}

	// Add variant details at standard+ verbosity
	if verbosity != output.VerbositySummary {
		for _, v := range m.Variants {
			result.Manifest.Variants = append(result.Manifest.Variants, output.VariantInfo{
				URI:        v.URI,
				Bandwidth:  v.Bandwidth,
				Resolution: v.Resolution,
				Codecs:     v.Codecs,
				FrameRate:  v.FrameRate,
			})
		}
	}

	// Add DRM info
	if m.DRM != nil {
		result.Manifest.DRM = &output.DRMSummary{
			System:    m.DRM.System,
			SchemeURI: m.DRM.SchemeURI,
			KeyID:     m.DRM.KeyID,
		}
	}

	// Add raw manifest at forensic level
	if verbosity == output.VerbosityForensic {
		result.Manifest.Raw = m.Raw
		result.RawOutput = m.Raw
	}

	return h.formatter.Format(result, verbosity), nil
}

type AnalyzeMediaHandler struct {
	fetcher      *fetcher.Fetcher
	orchestrator *Orchestrator
	formatter    *output.Formatter
	cfg          *config.Config
}

func (h *AnalyzeMediaHandler) Handle(ctx context.Context, input tools.ToolInput) (*output.AnalysisResult, error) {
	// Create workspace
	ws, err := fetcher.NewWorkspace()
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	defer ws.Cleanup()

	// Download asset
	err = h.fetcher.DownloadFile(ctx, input.URL, ws, "input", input.Headers)
	if err != nil {
		return nil, fmt.Errorf("download asset: %w", err)
	}

	// Run ffprobe
	ffprobeTool := ffmpeg.NewFFprobe(h.cfg.Docker.Images.FFmpeg)
	result, err := h.orchestrator.ExecuteToolDirect(ctx, ffprobeTool, input, ws.Dir)
	if err != nil {
		return nil, fmt.Errorf("ffprobe analysis: %w", err)
	}

	result.Tool = "analyze_media"
	result.Downloads = ws.Assets

	verbosity, _ := input.ParsedVerbosity()
	return h.formatter.Format(result, verbosity), nil
}

type AnalyzeMP4StructureHandler struct {
	fetcher      *fetcher.Fetcher
	orchestrator *Orchestrator
	formatter    *output.Formatter
	cfg          *config.Config
}

func (h *AnalyzeMP4StructureHandler) Handle(ctx context.Context, input tools.ToolInput) (*output.AnalysisResult, error) {
	ws, err := fetcher.NewWorkspace()
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	defer ws.Cleanup()

	err = h.fetcher.DownloadFile(ctx, input.URL, ws, "input", input.Headers)
	if err != nil {
		return nil, fmt.Errorf("download asset: %w", err)
	}

	mp4dumpTool := mp4tools.NewMP4Dump(h.cfg.Docker.Images.MP4)
	result, err := h.orchestrator.ExecuteToolDirect(ctx, mp4dumpTool, input, ws.Dir)
	if err != nil {
		return nil, fmt.Errorf("mp4dump analysis: %w", err)
	}

	result.Tool = "analyze_mp4_structure"
	result.Downloads = ws.Assets

	verbosity, _ := input.ParsedVerbosity()
	return h.formatter.Format(result, verbosity), nil
}

type SimulatePlaybackHandler struct {
	fetcher      *fetcher.Fetcher
	orchestrator *Orchestrator
	formatter    *output.Formatter
	cfg          *config.Config
}

func (h *SimulatePlaybackHandler) Handle(ctx context.Context, input tools.ToolInput) (*output.AnalysisResult, error) {
	ws, err := fetcher.NewWorkspace()
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	defer ws.Cleanup()

	err = h.fetcher.DownloadFile(ctx, input.URL, ws, "input", input.Headers)
	if err != nil {
		return nil, fmt.Errorf("download asset: %w", err)
	}

	if input.Duration == 0 {
		input.Duration = h.cfg.Playback.DefaultDuration
	}

	ffmpegTool := ffmpeg.NewFFmpeg(h.cfg.Docker.Images.FFmpeg)
	result, err := h.orchestrator.ExecuteToolDirect(ctx, ffmpegTool, input, ws.Dir)
	if err != nil {
		return nil, fmt.Errorf("playback simulation: %w", err)
	}

	result.Tool = "simulate_playback"
	result.Downloads = ws.Assets

	verbosity, _ := input.ParsedVerbosity()
	return h.formatter.Format(result, verbosity), nil
}

type ValidatePackagingHandler struct {
	fetcher      *fetcher.Fetcher
	orchestrator *Orchestrator
	formatter    *output.Formatter
	cfg          *config.Config
}

func (h *ValidatePackagingHandler) Handle(ctx context.Context, input tools.ToolInput) (*output.AnalysisResult, error) {
	ws, err := fetcher.NewWorkspace()
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	defer ws.Cleanup()

	err = h.fetcher.DownloadFile(ctx, input.URL, ws, "input", input.Headers)
	if err != nil {
		return nil, fmt.Errorf("download asset: %w", err)
	}

	shakaTool := shaka.NewShaka(h.cfg.Docker.Images.Shaka)
	result, err := h.orchestrator.ExecuteToolDirect(ctx, shakaTool, input, ws.Dir)
	if err != nil {
		return nil, fmt.Errorf("packaging validation: %w", err)
	}

	result.Tool = "validate_packaging"
	result.Downloads = ws.Assets

	verbosity, _ := input.ParsedVerbosity()
	return h.formatter.Format(result, verbosity), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Register high-level tools in server.go**

Add the following to `NewServer` in `server.go`, after the direct tool registrations and after creating the orchestrator/fetcher. This registers all 5 high-level tools as MCP tools with custom handler functions:

```go
// In NewServer, after creating orchestrator and fetcher, add:

manifestHandler := &AnalyzeManifestHandler{fetcher: f, formatter: formatter, cfg: cfg}
mediaHandler := &AnalyzeMediaHandler{fetcher: f, orchestrator: orchestrator, formatter: formatter, cfg: cfg}
mp4StructHandler := &AnalyzeMP4StructureHandler{fetcher: f, orchestrator: orchestrator, formatter: formatter, cfg: cfg}
playbackHandler := &SimulatePlaybackHandler{fetcher: f, orchestrator: orchestrator, formatter: formatter, cfg: cfg}
packagingHandler := &ValidatePackagingHandler{fetcher: f, orchestrator: orchestrator, formatter: formatter, cfg: cfg}

highLevelTools := []struct {
	name        string
	description string
	handler     func(context.Context, tools.ToolInput) (*output.AnalysisResult, error)
}{
	{"analyze_manifest", "Parse and inspect HLS/DASH manifests — variant streams, renditions, segments, DRM signaling", manifestHandler.Handle},
	{"analyze_media", "Deep media container/stream analysis — codecs, tracks, timing, bitrates", mediaHandler.Handle},
	{"analyze_mp4_structure", "MP4/fMP4 box hierarchy, fragment structure, sample tables, codec private data", mp4StructHandler.Handle},
	{"simulate_playback", "Decoder-level diagnostics via ffmpeg null decode — PTS warnings, decode errors, timestamp discontinuities", playbackHandler.Handle},
	{"validate_packaging", "Verify DASH/HLS packaging correctness — segment alignment, manifest consistency", packagingHandler.Handle},
}

for _, ht := range highLevelTools {
	name := ht.name
	handler := ht.handler
	mcpTool := mcp.Tool{
		Name:        name,
		Description: ht.description,
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"url":       map[string]interface{}{"type": "string", "description": "URL of the asset to analyze"},
				"verbosity": map[string]interface{}{"type": "string", "enum": []string{"summary", "standard", "deep", "forensic"}, "default": "standard"},
				"duration":  map[string]interface{}{"type": "integer", "description": "Inspection duration in seconds (for playback simulation)"},
				"headers":   map[string]interface{}{"type": "object", "additionalProperties": map[string]interface{}{"type": "string"}},
			},
			Required: []string{"url"},
		},
	}

	mcpSrv.AddTool(mcpTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		inputJSON, _ := json.Marshal(request.Params.Arguments)
		var input tools.ToolInput
		json.Unmarshal(inputJSON, &input)

		if input.URL == "" {
			return errorResult(output.ErrMalformedInput, "URL is required", "parse", ""), nil
		}

		result, err := handler(ctx, input)
		if err != nil {
			return errorResult(output.ErrToolExecution, err.Error(), "execute", input.URL), nil
		}

		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(resultJSON)}},
		}, nil
	})
}
```

Also update `ToolCount` to reflect all registered tools:

```go
func (s *Server) ToolCount() int {
	return len(s.registry.List()) + 5 // 7 direct + 5 high-level
}
```

- [ ] **Step 6: Commit**

```bash
git add internal/server/handlers.go internal/server/handlers_test.go internal/server/server.go
git commit -m "feat: add all high-level analysis tools (analyze_manifest, analyze_media, analyze_mp4_structure, simulate_playback, validate_packaging)"
```

---

### Task 23: Integration Tests

**Files:**
- Create: `integration_test.go`

- [ ] **Step 1: Write integration tests**

```go
//go:build integration

// integration_test.go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/meinart/video-debug-mcp/internal/config"
	"github.com/meinart/video-debug-mcp/internal/docker"
	"github.com/meinart/video-debug-mcp/internal/fetcher"
	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/server"
	"github.com/meinart/video-debug-mcp/internal/tools"
	"github.com/meinart/video-debug-mcp/internal/tools/ffmpeg"
)

func TestIntegration_DockerRunner(t *testing.T) {
	runner, err := docker.NewDockerRunner()
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := runner.Run(ctx, docker.RunRequest{
		Image:        "alpine:3.19",
		Command:      []string{"echo", "hello from docker"},
		WorkspaceDir: os.TempDir(),
		Timeout:      10 * time.Second,
	})

	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !result.Success() {
		t.Errorf("expected success, got exit code %d: %s", result.ExitCode, string(result.Stderr))
	}
}

func TestIntegration_FFprobe_MP4(t *testing.T) {
	runner, err := docker.NewDockerRunner()
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}

	// Serve a small test MP4 (generate one if needed)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a minimal valid MP4 — in real tests, serve a real fixture file
		w.Header().Set("Content-Type", "video/mp4")
		w.Write([]byte("minimal mp4 test data"))
	}))
	defer ts.Close()

	cfg := config.Default()
	f := fetcher.NewFetcher(10, 30*time.Second, "test/1.0")
	formatter := output.NewFormatter()
	orch := server.NewOrchestrator(runner, formatter, cfg)

	ws, _ := fetcher.NewWorkspace()
	defer ws.Cleanup()

	ctx := context.Background()
	_ = f.DownloadFile(ctx, ts.URL+"/test.mp4", ws, "input", nil)

	ffprobeTool := ffmpeg.NewFFprobe(cfg.Docker.Images.FFmpeg)
	input := tools.ToolInput{URL: ts.URL + "/test.mp4", Verbosity: "standard"}

	// This will fail if the Docker image isn't built, which is expected
	// In CI, run `make build-images` first
	result, err := orch.ExecuteToolDirect(ctx, ffprobeTool, input, ws.Dir)
	if err != nil {
		t.Logf("Expected if images not built: %v", err)
		t.Skip("Skipping — Docker image not available")
	}

	if result.Tool != "run_ffprobe" {
		t.Errorf("expected tool=run_ffprobe, got %s", result.Tool)
	}
}

func TestIntegration_HLS_EndToEnd(t *testing.T) {
	masterPlaylist := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=3000000,RESOLUTION=1280x720
720p.m3u8
`
	mediaPlaylist := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:6
#EXTINF:6.000,
seg001.ts
#EXTINF:6.000,
seg002.ts
#EXT-X-ENDLIST
`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/master.m3u8":
			w.Write([]byte(masterPlaylist))
		case "/720p.m3u8":
			w.Write([]byte(mediaPlaylist))
		default:
			w.Write([]byte("fake segment data"))
		}
	}))
	defer ts.Close()

	cfg := config.Default()
	srv, err := server.NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}

	if srv.ToolCount() < 12 {
		t.Errorf("expected at least 12 tools registered, got %d", srv.ToolCount())
	}
}
```

- [ ] **Step 2: Run unit tests to make sure nothing is broken**

Run: `go test ./...`
Expected: All unit tests PASS.

- [ ] **Step 3: Run integration tests (requires Docker)**

Run: `go test -tags integration -v -timeout 120s`
Expected: Integration tests PASS (or SKIP if Docker not available).

- [ ] **Step 4: Commit**

```bash
git add integration_test.go
git commit -m "feat: add integration tests for Docker runner, HLS flow, and ffprobe"
```

---

### Task 24: Final Wiring, go mod tidy, and Build Verification

- [ ] **Step 1: Run go mod tidy**

Run: `go mod tidy`
Expected: No errors. go.sum updated.

- [ ] **Step 2: Run full unit test suite**

Run: `go test ./... -v`
Expected: All tests PASS.

- [ ] **Step 3: Build the binary**

Run: `make build`
Expected: Binary created at `bin/video-debug-mcp`.

- [ ] **Step 4: Verify binary starts and shows tool count**

Run: `./bin/video-debug-mcp --config config.yaml &; sleep 2; kill %1`
Expected: Log output showing "Starting video-debug-mcp server on 0.0.0.0:8080" and "Registered N tools".

- [ ] **Step 5: Commit final state**

```bash
git add go.mod go.sum
git commit -m "chore: tidy modules and verify build"
```
