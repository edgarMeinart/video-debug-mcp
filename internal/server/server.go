package server

import (
	"context"
	"encoding/json"
	"fmt"

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
	"github.com/meinart/video-debug-mcp/internal/tools/mp4"
	"github.com/meinart/video-debug-mcp/internal/tools/shaka"
)

// Server wraps an MCP server with registered video debug tools.
type Server struct {
	mcp          *mcpserver.MCPServer
	orchestrator *Orchestrator
	registry     *tools.Registry
	fetcher      *fetcher.Fetcher
	formatter    *output.Formatter
	cfg          *config.Config
	toolCount    int
}

// NewServer creates a Server with all tools registered. Docker unavailability is
// handled gracefully — the server is still created but direct tool execution
// will fail at runtime if Docker is needed.
func NewServer(cfg *config.Config) (*Server, error) {
	// Create Docker runner (may fail if Docker is not available).
	var runner docker.Runner
	dr, err := docker.NewDockerRunner()
	if err != nil {
		// Docker not available — use a stub that returns errors.
		runner = &noopRunner{}
	} else {
		runner = dr
	}

	formatter := output.NewFormatter()
	orch := NewOrchestrator(runner, formatter, cfg)
	f := fetcher.NewFetcher(
		cfg.Fetcher.MaxConcurrentDownloads,
		cfg.Fetcher.RequestTimeout,
		cfg.Fetcher.UserAgent,
	)

	registry := tools.NewRegistry()

	// Register all 7 direct tools.
	directTools := []tools.Tool{
		ffmpeg.NewFFprobe(cfg.Docker.Images.FFmpeg),
		ffmpeg.NewFFmpeg(cfg.Docker.Images.FFmpeg),
		mp4.NewMP4Box(cfg.Docker.Images.MP4),
		mp4.NewMP4Dump(cfg.Docker.Images.MP4),
		bento4.NewBento4(cfg.Docker.Images.Bento4),
		mediainfo.NewMediaInfo(cfg.Docker.Images.MediaInfo),
		shaka.NewShaka(cfg.Docker.Images.Shaka),
	}
	for _, t := range directTools {
		registry.Register(t)
	}

	// Create the MCP server.
	mcpSrv := mcpserver.NewMCPServer(
		"video-debug-mcp",
		"1.0.0",
	)

	srv := &Server{
		mcp:          mcpSrv,
		orchestrator: orch,
		registry:     registry,
		fetcher:      f,
		formatter:    formatter,
		cfg:          cfg,
	}

	// Register direct tools as MCP tools.
	for _, t := range directTools {
		srv.registerDirectTool(t)
	}

	// Register high-level analysis tools.
	srv.registerHighLevelTools()

	return srv, nil
}

// ToolCount returns the total number of registered MCP tools.
func (s *Server) ToolCount() int {
	return s.toolCount
}

// MCPServer returns the underlying mcp-go MCPServer.
func (s *Server) MCPServer() *mcpserver.MCPServer {
	return s.mcp
}

// registerDirectTool registers a single tools.Tool as an MCP tool with a handler
// that converts the request arguments to ToolInput and executes via the orchestrator.
func (s *Server) registerDirectTool(t tools.Tool) {
	mcpTool := mcp.NewTool(t.Name(),
		mcp.WithDescription(t.Description()),
		mcp.WithString("url", mcp.Description("URL or path to the media file"), mcp.Required()),
		mcp.WithString("verbosity", mcp.Description("Output verbosity: summary, standard, deep, forensic")),
	)

	toolRef := t // capture for closure
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		input, err := requestToToolInput(req)
		if err != nil {
			return errorResult(output.ErrInvalidInput, err.Error()), nil
		}

		ws, err := fetcher.NewWorkspace()
		if err != nil {
			return errorResult(output.ErrInternalFailure, fmt.Sprintf("create workspace: %v", err)), nil
		}
		defer ws.Cleanup()

		// Download asset if URL is provided.
		if input.URL != "" {
			if err := s.fetcher.DownloadFile(ctx, input.URL, ws, "input", input.Headers); err != nil {
				return errorResult(output.ErrSegmentFetch, fmt.Sprintf("download: %v", err)), nil
			}
		}

		result, err := s.orchestrator.ExecuteToolDirect(ctx, toolRef, input, ws.Dir)
		if err != nil {
			return errorResult(output.ErrInternalFailure, err.Error()), nil
		}

		return analysisResultToMCP(result)
	}

	s.mcp.AddTool(mcpTool, handler)
	s.toolCount++
}

// registerHighLevelTools registers the 5 high-level analysis tools.
func (s *Server) registerHighLevelTools() {
	handlers := []struct {
		name        string
		description string
		handler     mcpserver.ToolHandlerFunc
	}{
		{
			name:        "analyze_manifest",
			description: "Download and parse an HLS or DASH manifest, returning structured variant, DRM, and segment information.",
			handler:     (&AnalyzeManifestHandler{fetcher: s.fetcher, formatter: s.formatter, cfg: s.cfg}).Handle,
		},
		{
			name:        "analyze_media",
			description: "Download a media asset and run ffprobe to return detailed stream, codec, and timing information.",
			handler:     (&AnalyzeMediaHandler{fetcher: s.fetcher, orchestrator: s.orchestrator, registry: s.registry, cfg: s.cfg}).Handle,
		},
		{
			name:        "analyze_mp4_structure",
			description: "Download an MP4 asset and run mp4dump to return the ISO BMFF box tree structure.",
			handler:     (&AnalyzeMP4StructureHandler{fetcher: s.fetcher, orchestrator: s.orchestrator, registry: s.registry, cfg: s.cfg}).Handle,
		},
		{
			name:        "simulate_playback",
			description: "Download a media asset and run ffmpeg with the null muxer to detect decode errors and timing anomalies.",
			handler:     (&SimulatePlaybackHandler{fetcher: s.fetcher, orchestrator: s.orchestrator, registry: s.registry, cfg: s.cfg}).Handle,
		},
		{
			name:        "validate_packaging",
			description: "Download a media asset and run Shaka Packager to validate packaging and detect anomalies.",
			handler:     (&ValidatePackagingHandler{fetcher: s.fetcher, orchestrator: s.orchestrator, registry: s.registry, cfg: s.cfg}).Handle,
		},
	}

	for _, h := range handlers {
		tool := mcp.NewTool(h.name,
			mcp.WithDescription(h.description),
			mcp.WithString("url", mcp.Description("URL to the media asset or manifest"), mcp.Required()),
			mcp.WithString("verbosity", mcp.Description("Output verbosity: summary, standard, deep, forensic")),
		)
		s.mcp.AddTool(tool, h.handler)
		s.toolCount++
	}
}

// requestToToolInput converts MCP request arguments to a ToolInput.
func requestToToolInput(req mcp.CallToolRequest) (tools.ToolInput, error) {
	args := req.Params.Arguments
	data, err := json.Marshal(args)
	if err != nil {
		return tools.ToolInput{}, fmt.Errorf("marshal arguments: %w", err)
	}
	var input tools.ToolInput
	if err := json.Unmarshal(data, &input); err != nil {
		return tools.ToolInput{}, fmt.Errorf("unmarshal arguments: %w", err)
	}
	return input, nil
}

// analysisResultToMCP converts an AnalysisResult to a CallToolResult with JSON text content.
func analysisResultToMCP(result *output.AnalysisResult) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errorResult(output.ErrInternalFailure, fmt.Sprintf("marshal result: %v", err)), nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: string(data),
			},
		},
	}, nil
}

// errorResult creates a CallToolResult indicating an error.
func errorResult(code output.ErrorCode, message string) *mcp.CallToolResult {
	errResp := output.ErrorResponse{
		Code:    code,
		Message: message,
	}
	data, _ := json.Marshal(errResp)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: string(data),
			},
		},
		IsError: true,
	}
}

// noopRunner is a Docker runner stub used when Docker is not available.
type noopRunner struct{}

func (n *noopRunner) Run(_ context.Context, _ docker.RunRequest) (*docker.RunResult, error) {
	return nil, fmt.Errorf("docker is not available")
}

// Ensure noopRunner satisfies docker.Runner at compile time.
var _ docker.Runner = (*noopRunner)(nil)
