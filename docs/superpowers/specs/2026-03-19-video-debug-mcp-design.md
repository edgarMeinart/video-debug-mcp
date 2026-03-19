# Video Debug MCP Server — Design Spec

## Overview

A Go-based MCP server that provides deep technical analysis and debugging capabilities for video delivery workflows. It acts as a unified debugging and inspection layer, enabling AI agents to analyze streaming manifests (HLS, DASH), media segments, and container metadata through a consistent, structured interface backed by isolated dockerized tools.

## Decisions Summary

| Decision | Choice |
|---|---|
| MCP transport | HTTP/SSE |
| MCP SDK | mcp-go (`github.com/mark3labs/mcp-go`) |
| Docker orchestration | Docker SDK for Go (`github.com/docker/docker/client`) |
| Container strategy | Tool-family images (ffmpeg, mp4, bento4, mediainfo, shaka) |
| Asset storage | Temp directory per request, cleaned up after response |
| Additional CLI tools | Bento4, MediaInfo, Shaka Packager (in addition to ffmpeg, MP4Box, mp4dump) |
| Manifest parsing | Hybrid — Go libraries (grafov/m3u8, go-dash) wrapped in a unified model |
| Concurrency | Concurrent downloads, sequential analysis |
| Playback simulation | `ffmpeg -f null -` with verbose logging (replaces ffplay) |
| Configuration | Config file (YAML) with environment variable overrides |
| Architecture | Layered with Tool plugin interface |

## Architecture

### Layered Design

1. **MCP Transport Layer** — mcp-go handles SSE/HTTP, tool registration
2. **Tool Orchestrator** — receives tool calls, coordinates the workflow (download -> analyze -> format)
3. **Manifest Parser** — unified model wrapping HLS/DASH libraries
4. **Asset Fetcher** — downloads manifests/segments with concurrent segment fetching, temp dir lifecycle
5. **Docker Runner** — generic container execution engine (Docker SDK), manages image pulls, bind mounts, output capture
6. **Tool Definitions** — each tool family is a self-contained module implementing the Tool interface
7. **Output Normalizer** — transforms raw tool output into structured responses at the requested verbosity level

### Project Structure

```
video-debug-mcp/
├── cmd/
│   └── server/
│       └── main.go              # Entry point, config loading, MCP server startup
├── internal/
│   ├── config/
│   │   └── config.go            # Config file + env var loading
│   ├── server/
│   │   └── server.go            # MCP server setup, tool registration with mcp-go
│   ├── tools/
│   │   ├── registry.go          # Tool registry — maps MCP tool names to Tool implementations
│   │   ├── tool.go              # Tool interface definition
│   │   ├── ffmpeg/
│   │   │   ├── ffprobe.go       # ffprobe tool (stream/container analysis)
│   │   │   ├── ffmpeg.go        # ffmpeg -f null tool (playback simulation)
│   │   │   └── parse.go         # ffmpeg/ffprobe output parsers
│   │   ├── mp4/
│   │   │   ├── mp4box.go        # MP4Box tool
│   │   │   ├── mp4dump.go       # mp4dump tool
│   │   │   └── parse.go         # MP4 tool output parsers
│   │   ├── bento4/
│   │   │   ├── mp4info.go       # mp4info, mp4dump, mp4fragment, etc.
│   │   │   └── parse.go         # Bento4 output parsers
│   │   ├── mediainfo/
│   │   │   ├── mediainfo.go     # MediaInfo tool
│   │   │   └── parse.go         # MediaInfo output parser
│   │   └── shaka/
│   │       ├── packager.go      # Shaka Packager validation
│   │       └── parse.go         # Shaka output parser
│   ├── manifest/
│   │   ├── model.go             # Unified manifest model (protocol-agnostic)
│   │   ├── hls/
│   │   │   └── parser.go        # HLS parser (wraps grafov/m3u8)
│   │   └── dash/
│   │       └── parser.go        # DASH parser (wraps go-dash)
│   ├── fetcher/
│   │   ├── fetcher.go           # Asset downloader — manifest + concurrent segment fetch
│   │   └── workspace.go         # Temp directory lifecycle management
│   ├── docker/
│   │   ├── runner.go            # Docker container execution engine (Docker SDK)
│   │   └── images.go            # Image registry — names, pull logic, health checks
│   └── output/
│       ├── formatter.go         # Output normalizer — verbosity levels
│       └── model.go             # Structured output types (AnalysisResult, etc.)
├── docker/
│   ├── ffmpeg-tools/
│   │   └── Dockerfile           # ffmpeg, ffprobe
│   ├── mp4-tools/
│   │   └── Dockerfile           # MP4Box, mp4dump (GPAC)
│   ├── bento4-tools/
│   │   └── Dockerfile           # Bento4 suite
│   ├── mediainfo-tools/
│   │   └── Dockerfile           # MediaInfo
│   └── shaka-tools/
│       └── Dockerfile           # Shaka Packager
├── config.yaml                  # Default configuration
├── go.mod
└── go.sum
```

## Core Interfaces

### Tool Interface

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() json.RawMessage
    DockerImage() string
    BuildCommand(input ToolInput) (*DockerCommand, error)
    ParseOutput(raw []byte, verbosity Verbosity) (*AnalysisResult, error)
}
```

### Request Data Flow

```
AI Agent
  |
  v
MCP Server (mcp-go, SSE/HTTP)
  |
  v
Tool Registry  --lookup-->  Tool Implementation
  |
  v
Orchestrator
  ├── 1. Fetcher: download manifest/asset -> temp dir
  ├── 2. Manifest Parser: parse HLS/DASH -> unified model (if manifest URL)
  ├── 3. Fetcher: concurrent segment downloads (if needed)
  ├── 4. Tool.BuildCommand(): construct CLI invocation
  ├── 5. Docker Runner: execute container (bind-mount temp dir, capture output)
  ├── 6. Tool.ParseOutput(): raw -> structured
  └── 7. Output Formatter: apply verbosity level -> response
  |
  v
Structured JSON response back to AI Agent
```

### Verbosity Levels

```go
type Verbosity string

const (
    VerbositySummary  Verbosity = "summary"   // Key findings only
    VerbosityStandard Verbosity = "standard"  // Default — findings + stream inventory + metadata
    VerbosityDeep     Verbosity = "deep"      // Above + full box structure, all tracks, timing details
    VerbosityForensic Verbosity = "forensic"  // Everything — raw output included, full dumps
)
```

### Structured Output Model

```go
type AnalysisResult struct {
    Tool            string
    Command         string
    InputURL        string
    ResolvedURLs    []string
    Downloads       []DownloadedAsset
    Manifest        *ManifestSummary
    Streams         []StreamInfo
    Codec           *CodecDetails
    Timing          *TimingInfo
    Bitrate         *BitrateInfo
    ContainerLayout *BoxTree
    Anomalies       []Anomaly
    RawOutput       string            // Forensic only
}
```

## MCP Tools

### High-Level Analysis Tools

| MCP Tool | Purpose | Underlying CLI |
|---|---|---|
| `analyze_manifest` | Parse and inspect HLS/DASH manifests — variant streams, renditions, segments, DRM signaling | Go-native parsing (grafov/m3u8, go-dash) |
| `analyze_media` | Deep media container/stream analysis — codecs, tracks, timing, bitrates | ffprobe + mediainfo |
| `analyze_mp4_structure` | MP4/fMP4 box hierarchy, fragment structure, sample tables, codec private data | MP4Box + bento4 mp4dump |
| `simulate_playback` | Decoder-level diagnostics — PTS warnings, decode errors, timestamp discontinuities | ffmpeg -f null - |
| `validate_packaging` | Verify DASH/HLS packaging correctness — segment alignment, manifest consistency | Shaka Packager |

### Direct Tool Access

| MCP Tool | Purpose |
|---|---|
| `run_ffprobe` | Run ffprobe with caller-specified arguments |
| `run_ffmpeg` | Run ffmpeg with caller-specified arguments |
| `run_mp4box` | Run MP4Box with caller-specified arguments |
| `run_mp4dump` | Run mp4dump (GPAC) with caller-specified arguments |
| `run_bento4` | Run any Bento4 tool (mp4info, mp4dump, mp4fragment, mp4encrypt, mp4decrypt) |
| `run_mediainfo` | Run MediaInfo with caller-specified arguments |
| `run_shaka_packager` | Run Shaka Packager with caller-specified arguments |

### Common Input Schema

All tools share these base fields:

```json
{
  "url": "string (required) — manifest or media asset URL",
  "verbosity": "string (optional) — summary|standard|deep|forensic, default: standard",
  "duration": "number (optional) — inspection duration in seconds for playback simulation",
  "headers": "object (optional) — custom HTTP headers for asset fetching"
}
```

Direct tool access tools add an `args` field for custom CLI arguments.

### Orchestrator Behavior

High-level tools are "smart" — `analyze_manifest` for instance:
1. Detects whether the URL is HLS or DASH
2. Downloads and parses the manifest
3. Resolves all segment/init URLs
4. Downloads representative segments (not necessarily all — configurable)
5. Runs the appropriate analysis
6. Returns a unified structured result

Direct tools are "thin" — download the asset, run the exact command, parse and return.

## Docker Container Architecture

### Tool-Family Images

| Image | Base | Contents | Size Estimate |
|---|---|---|---|
| `video-debug/ffmpeg-tools` | Alpine | ffmpeg, ffprobe | ~80MB |
| `video-debug/mp4-tools` | Alpine | GPAC (MP4Box, mp4dump) | ~50MB |
| `video-debug/bento4-tools` | Alpine | Bento4 suite | ~30MB |
| `video-debug/mediainfo-tools` | Alpine | MediaInfo CLI | ~20MB |
| `video-debug/shaka-tools` | Alpine | Shaka Packager | ~40MB |

### Container Execution Model

1. Ensure image exists (pull/build if missing — lazy on first invocation)
2. Create container with:
   - Read-only bind mount of temp workspace to `/workspace`
   - No network access (assets already downloaded)
   - Memory + CPU limits from config
   - Auto-remove on exit
3. Start container, stream stdout+stderr
4. Wait for exit with timeout (context deadline)
5. Return: exit code, stdout, stderr, duration

### Key Design Choices

- **No network inside containers** — assets are pre-downloaded and bind-mounted for deterministic execution
- **Read-only mounts** — containers read assets, output captured from stdout/stderr
- **Auto-remove** — ephemeral containers, no cleanup needed
- **Resource limits** — configurable per tool family
- **Timeout enforcement** — container killed on timeout, structured error returned

### Image Lifecycle

- On startup: check which images are present, log warnings for missing ones (don't block)
- On first tool invocation: build image if missing (lazy build)
- `make build-images` target for building all images

## Manifest Parsing

### Unified Manifest Model

```go
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
```

### HLS Parser (wraps grafov/m3u8)

- Detects master vs media playlist
- If master: parses all variant streams, audio/subtitle renditions, fetches and parses each media playlist
- Resolves relative URIs against manifest base URL
- Extracts EXT-X-KEY, EXT-X-MAP, EXT-X-MEDIA
- Maps to unified Manifest model

### DASH Parser (wraps go-dash)

- Parses MPD: periods, adaptation sets, representations
- Resolves segment templates / segment timelines into concrete segment URLs
- Extracts ContentProtection elements
- Handles SegmentTemplate and SegmentList addressing
- Maps to unified Manifest model

## Asset Fetching

### Fetcher

- Creates a temp directory per request
- Downloads manifest, detects type (HLS/DASH/direct media by URL extension and content)
- If manifest: parses, resolves all segment URLs
- Downloads segments concurrently (bounded by `max_concurrent_downloads` config)
- Returns a Workspace with all file paths mapped

### Segment Sampling Strategy

For manifests with many segments:
- **Init segments**: always download all
- **Media segments**: download first N + last N per track (default N=3, configurable)
- **Full download**: available via `download_all` flag in tool input

## Configuration

### config.yaml

```yaml
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

### Environment Variable Overrides

Pattern: `VIDEO_DEBUG_<SECTION>_<KEY>`, e.g.:
- `VIDEO_DEBUG_SERVER_PORT=9090`
- `VIDEO_DEBUG_DOCKER_DEFAULTS_TIMEOUT=120s`
- `VIDEO_DEBUG_FETCHER_MAX_CONCURRENT_DOWNLOADS=20`

## Error Handling

### Structured Errors

```go
type ErrorResponse struct {
    Code    ErrorCode
    Message string
    Details string
    Phase   string   // "fetch", "parse", "execute", "format"
    URL     string
    Command string
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
```

### Key Behaviors

- **Phase tagging** — every error reports which pipeline phase it occurred in
- **Partial results** — if some segments fail to download, analysis runs on what was fetched, with anomalies noting the gaps
- **Docker health** — actionable errors for Docker daemon issues vs image issues
- **Timeout clarity** — reports configured timeout and suggests increasing it

## Testing Strategy

### Unit Tests (no external dependencies)

| Package | What's Tested | Approach |
|---|---|---|
| `internal/manifest/hls/` | HLS parsing, URI resolution, variant extraction, DRM tags | Sample m3u8 strings -> assert unified model |
| `internal/manifest/dash/` | MPD parsing, segment template resolution, ContentProtection | Sample MPD XML -> assert unified model |
| `internal/tools/ffmpeg/` | Command construction, output parsing at all verbosity levels | Sample outputs -> assert structured results |
| `internal/tools/mp4/` | Command construction, output parsing | Same pattern |
| `internal/tools/bento4/` | Command construction, output parsing | Same pattern |
| `internal/tools/mediainfo/` | Command construction, output parsing | Sample MediaInfo XML output |
| `internal/tools/shaka/` | Command construction, output parsing | Same pattern |
| `internal/fetcher/` | URL resolution, segment sampling, workspace lifecycle | httptest.Server with sample manifests |
| `internal/docker/` | Command assembly, timeout enforcement, resource limits | Mock Docker client interface |
| `internal/output/` | Verbosity filtering, field omission, envelope structure | AnalysisResult -> assert JSON at each level |
| `internal/config/` | Config loading, env var overrides, defaults | Temp config files + env vars |

### Integration Tests (require Docker, build tag `//go:build integration`)

| Test | Coverage |
|---|---|
| HLS flow | httptest manifest -> analyze_manifest -> end-to-end structured output |
| DASH flow | httptest MPD -> same pattern |
| Direct MP4 flow | httptest MP4 -> analyze_media -> container execution -> output |
| Docker interaction | Container lifecycle: create, start, wait, output capture, auto-remove, timeout kill |
| Error scenarios | 404 manifest, malformed manifest, missing image, tool timeout -> structured errors |

### Test Fixtures

`testdata/` directories alongside packages with sample m3u8, MPD, ffprobe JSON, MP4Box output, small valid MP4/fMP4 files.

### TDD Workflow

Every feature: write failing test -> implement -> pass -> refactor.
