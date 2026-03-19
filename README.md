# video-debug-mcp

A Go-based MCP (Model Context Protocol) server that gives AI agents deep inspection capabilities for video delivery workflows — HLS, DASH, MP4, audio, video, and subtitle assets.

It wraps industry-standard media tools (ffmpeg, MP4Box, Bento4, MediaInfo, Shaka Packager) behind a structured interface, running each in an isolated Docker container. You point it at a URL, it downloads the asset, runs the right tools, and returns structured, machine-readable analysis.

## Why this exists

Debugging video delivery is painful. A typical investigation involves:

- Downloading manifests and segments by hand
- Running ffprobe, MP4Box, mediainfo with different flags
- Eyeballing raw CLI output for anomalies
- Mentally correlating findings across tools
- Repeating all of this for every variant, rendition, and segment

This MCP server automates that entire workflow. An AI agent can call `analyze_manifest` with a URL and get back a structured JSON response with variant streams, DRM signaling, segment inventory, and detected anomalies — no manual tool invocation needed.

It's particularly useful for:

- **Incident response** — quickly inspect a broken stream without SSH'ing into anything
- **QA validation** — verify packaging correctness across HLS/DASH variants
- **Debugging playback issues** — detect PTS discontinuities, decode errors, timestamp problems
- **DRM verification** — inspect content protection signaling in manifests and containers
- **Container forensics** — dive into MP4 box structure, fragment layout, codec parameters

## What's honest about this

- **It's a v1.** The output parsers use regex and string matching on CLI output. They cover common cases well but may miss edge cases with unusual tool output formats.
- **Docker is required.** Every tool runs in a container. No Docker, no analysis (manifest parsing still works).
- **It downloads things.** The fetcher pulls manifests and segments to a temp directory. For large manifests, it samples segments (first N + last N) rather than downloading everything.
- **No DRM decryption.** It inspects DRM signaling (PSSH boxes, EXT-X-KEY tags, ContentProtection elements) but cannot decrypt protected content.
- **ffplay is replaced.** Instead of actual playback simulation, it uses `ffmpeg -f null -` which provides the same decoder diagnostics without needing a display.

## Quick start

### Prerequisites

- Go 1.21+
- Docker running locally

### Build and run

```bash
# Build the tool images
make build-images

# Build the server
make build

# Run
./bin/video-debug-mcp
```

The server starts on `http://0.0.0.0:8080` (SSE transport).

### Connect from Claude Desktop

Add to your Claude Desktop MCP config:

```json
{
  "mcpServers": {
    "video-debug": {
      "url": "http://localhost:8080/sse"
    }
  }
}
```

## Tools

### High-level analysis tools

These are the "smart" tools — they handle downloading, parsing, and analysis automatically.

| Tool | What it does |
|---|---|
| `analyze_manifest` | Downloads and parses HLS/DASH manifests. Returns variant streams, audio/subtitle renditions, DRM signaling, segment inventory. |
| `analyze_media` | Downloads a media file and runs ffprobe. Returns streams, codecs, timing, bitrates. |
| `analyze_mp4_structure` | Downloads an MP4 and runs mp4dump. Returns the ISO BMFF box tree hierarchy. |
| `simulate_playback` | Downloads a media file and decodes it via `ffmpeg -f null`. Detects PTS discontinuities, decode errors, timestamp anomalies. |
| `validate_packaging` | Downloads a media file and runs Shaka Packager validation. Checks packaging correctness and reports warnings. |

### Direct tool access

For when you need to run a specific tool with custom arguments.

| Tool | Underlying CLI |
|---|---|
| `run_ffprobe` | ffprobe with JSON output |
| `run_ffmpeg` | ffmpeg null muxer decode |
| `run_mp4box` | MP4Box -info |
| `run_mp4dump` | mp4dump box structure |
| `run_bento4` | Any Bento4 sub-tool (mp4info, mp4dump, mp4fragment, mp4encrypt, mp4decrypt) |
| `run_mediainfo` | MediaInfo JSON output |
| `run_shaka_packager` | Shaka Packager stream info |

### Common input parameters

All tools accept:

| Parameter | Type | Required | Description |
|---|---|---|---|
| `url` | string | yes | URL of the manifest or media asset |
| `verbosity` | string | no | `summary`, `standard` (default), `deep`, or `forensic` |
| `duration` | integer | no | Playback simulation duration in seconds (for simulate_playback) |
| `args` | string[] | no | Custom CLI arguments (overrides defaults, direct tools only) |
| `headers` | object | no | Custom HTTP headers for asset fetching (e.g., auth tokens) |

## Verbosity levels

| Level | What's included |
|---|---|
| `summary` | Key findings only — tool name, streams, timing, anomalies |
| `standard` | Default. Adds command executed, resolved URLs, downloads, manifest details, codec info |
| `deep` | Adds MP4 box tree structure, all timing details |
| `forensic` | Everything, including raw CLI output dumps |

## Configuration

### Config file

The server reads `config.yaml` (or specify with `--config path/to/config.yaml`):

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

If the config file is missing, defaults are used. Environment variables override both.

### Environment variables

| Variable | Overrides | Example |
|---|---|---|
| `VIDEO_DEBUG_SERVER_PORT` | server.port | `9090` |
| `VIDEO_DEBUG_SERVER_HOST` | server.host | `127.0.0.1` |
| `VIDEO_DEBUG_FETCHER_MAX_CONCURRENT_DOWNLOADS` | fetcher.max_concurrent_downloads | `20` |
| `VIDEO_DEBUG_DOCKER_DEFAULTS_TIMEOUT` | docker.defaults.timeout | `120s` |
| `VIDEO_DEBUG_OUTPUT_DEFAULT_VERBOSITY` | output.default_verbosity | `deep` |

## Architecture

```
AI Agent (Claude, etc.)
  |
  | MCP protocol (SSE/HTTP)
  v
+---------------------------------------------------+
|  MCP Server (mcp-go)                              |
|  - Tool registration                              |
|  - Request routing                                |
+---------------------------------------------------+
  |                         |
  v                         v
+------------------+  +------------------+
| High-Level Tools |  | Direct Tools     |
| (Go handlers)    |  | (Tool interface) |
+------------------+  +------------------+
  |                         |
  v                         v
+---------------------------------------------------+
|  Orchestrator                                     |
|  1. Fetcher downloads asset to temp dir           |
|  2. Manifest parser extracts structure (if HLS/DASH)|
|  3. Tool.BuildCommand() constructs CLI invocation |
|  4. Docker Runner executes in isolated container  |
|  5. Tool.ParseOutput() structures the results     |
|  6. Formatter applies verbosity filtering         |
+---------------------------------------------------+
  |
  v
+---------------------------------------------------+
|  Docker Containers (one per tool family)           |
|  - No network access                              |
|  - Read-only /workspace mount                     |
|  - Memory + CPU limits                            |
|  - Auto-removed after execution                   |
+---------------------------------------------------+
  |           |          |          |          |
  v           v          v          v          v
ffmpeg    MP4Box     Bento4    MediaInfo   Shaka
ffprobe   mp4dump    mp4info              Packager
```

### Key design decisions

**Docker isolation.** Each tool runs in its own container with no network access. Assets are pre-downloaded by the fetcher and bind-mounted read-only into `/workspace`. This makes execution deterministic and prevents tools from making unexpected network calls.

**Tool-family images.** Related tools share an image (e.g., ffmpeg and ffprobe are in the same `ffmpeg-tools` image). This balances isolation with image count — 5 images instead of 10+.

**Segment sampling.** For manifests with hundreds of segments, downloading all of them is wasteful for most analyses. By default, the first 3 and last 3 segments per track are downloaded. This covers initialization, steady-state, and end-of-stream behavior. Use `download_all: true` for full downloads.

**Manifest parsing is native Go.** HLS and DASH manifests are parsed in-process (not via Docker tools) using `grafov/m3u8` for HLS and `encoding/xml` for DASH. This is faster and doesn't require Docker for manifest-only analysis.

**Unified manifest model.** Both HLS and DASH are parsed into the same `Manifest` struct. The `analyze_manifest` tool doesn't care which protocol was used — it returns the same structured output either way.

### Adding a new tool

1. Create a package in `internal/tools/yourtool/`
2. Implement the `tools.Tool` interface:
   - `Name()` — MCP tool name
   - `Description()` — what it does
   - `InputSchema()` — JSON schema for parameters
   - `DockerImage()` — which container to run in
   - `BuildCommand(input)` — construct the CLI invocation
   - `ParseOutput(raw, verbosity)` — parse raw output into `AnalysisResult`
3. Create a Dockerfile in `docker/yourtool/`
4. Register it in `internal/server/server.go`
5. Add the image name to `config.yaml`

### Error handling

All errors are returned as structured JSON with a machine-readable code:

```json
{
  "code": "MANIFEST_FETCH_FAILED",
  "message": "Failed to download manifest: HTTP 404",
  "phase": "download",
  "url": "https://example.com/master.m3u8"
}
```

Errors include which phase of the pipeline failed (download, parse, execute, format) so the caller knows where things broke.

## Nuances and known limitations

- **`grafov/m3u8` quirks.** The HLS parser library only associates `EXT-X-MEDIA` renditions with variants when the `AUDIO`/`SUBTITLES` group-id attributes are present on `EXT-X-STREAM-INF` tags. Manifests that declare `EXT-X-MEDIA` without referencing the group from a stream tag won't have their audio/subtitle tracks extracted.
- **DASH parser is XML-only.** It uses `encoding/xml` directly rather than the `go-dash` library. This handles SegmentTemplate with `$Number$` and `$RepresentationID$` substitution, but SegmentTimeline (used by some live DASH streams) is not yet implemented.
- **MP4Box and mp4dump images.** The `mp4` tool family structs currently have hardcoded image names rather than reading from config. The config values (`docker.images.mp4`) are defined but not wired to these tools — the images set in the Dockerfile build are what matter.
- **Bento4 binary downloads.** The Bento4 Dockerfile downloads pre-built binaries from `bok.net`. These are only available for x86_64 and aarch64. Other architectures aren't supported.
- **Output parsers are regex-based.** ffmpeg, MP4Box, and Shaka output is parsed via regular expressions. Unusual output formatting, different tool versions, or localized messages may not parse correctly.
- **No streaming analysis.** The server downloads complete assets before analysis. It doesn't support analyzing live streams in real-time or connecting to running playback sessions.
- **Per-tool Docker images must be pre-built.** The server doesn't auto-build images. Run `make build-images` before first use.

## Development

```bash
# Run all unit tests
make test

# Run integration tests (requires Docker)
make test-integration

# Build all Docker images
make build-images

# Build the server binary
make build
```

## License

MIT
