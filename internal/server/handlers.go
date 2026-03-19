package server

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/meinart/video-debug-mcp/internal/config"
	"github.com/meinart/video-debug-mcp/internal/fetcher"
	manifestpkg "github.com/meinart/video-debug-mcp/internal/manifest"
	"github.com/meinart/video-debug-mcp/internal/manifest/dash"
	"github.com/meinart/video-debug-mcp/internal/manifest/hls"
	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

// AnalyzeManifestHandler downloads a manifest and parses it as HLS or DASH.
type AnalyzeManifestHandler struct {
	fetcher   *fetcher.Fetcher
	formatter *output.Formatter
	cfg       *config.Config
}

// Handle processes an analyze_manifest request.
func (h *AnalyzeManifestHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	input, err := requestToToolInput(req)
	if err != nil {
		return errorResult(output.ErrInvalidInput, err.Error()), nil
	}
	if input.URL == "" {
		return errorResult(output.ErrInvalidInput, "url is required"), nil
	}

	// Download manifest content directly.
	body, err := httpGet(ctx, input.URL)
	if err != nil {
		return errorResult(output.ErrManifestFetch, fmt.Sprintf("fetch manifest: %v", err)), nil
	}

	assetType := fetcher.DetectAssetType(input.URL, body)

	var result *output.AnalysisResult

	switch assetType {
	case fetcher.AssetTypeHLS:
		m, err := hls.Parse(body, input.URL)
		if err != nil {
			return errorResult(output.ErrManifestFetch, fmt.Sprintf("parse HLS: %v", err)), nil
		}
		result = manifestToResult(m, "hls", input.URL)

	case fetcher.AssetTypeDASH:
		m, err := dash.Parse(body, input.URL)
		if err != nil {
			return errorResult(output.ErrManifestFetch, fmt.Sprintf("parse DASH: %v", err)), nil
		}
		result = manifestToResult(m, "dash", input.URL)

	default:
		return errorResult(output.ErrInvalidInput, "URL does not appear to be an HLS or DASH manifest"), nil
	}

	verbosity, err := input.ParsedVerbosity()
	if err != nil {
		verbosity = output.VerbosityStandard
	}
	result = h.formatter.Format(result, verbosity)

	return analysisResultToMCP(result)
}

// manifestToResult converts a parsed manifest.Manifest to an AnalysisResult.
func manifestToResult(m *manifestpkg.Manifest, mtype, url string) *output.AnalysisResult {
	summary := &output.ManifestSummary{
		Type:         mtype,
		URL:          url,
		VariantCount: len(m.Variants),
	}

	for _, v := range m.Variants {
		summary.Variants = append(summary.Variants, output.VariantInfo{
			URL:        v.URI,
			Bandwidth:  int64(v.Bandwidth),
			Resolution: v.Resolution,
			Codecs:     v.Codecs,
		})
	}

	// Count total segments.
	totalSegments := 0
	for _, v := range m.Variants {
		totalSegments += len(v.Segments)
	}
	summary.SegmentCount = totalSegments

	if m.DRM != nil {
		summary.DRM = &output.DRMSummary{
			System:    m.DRM.System,
			KeyID:     m.DRM.KeyID,
			Encrypted: m.DRM.KeyID != "" || m.DRM.PSSH != "" || m.DRM.EXTXKey != "",
		}
	}

	return &output.AnalysisResult{
		Tool:     "analyze_manifest",
		InputURL: url,
		Manifest: summary,
	}
}

// AnalyzeMediaHandler downloads an asset and runs ffprobe.
type AnalyzeMediaHandler struct {
	fetcher      *fetcher.Fetcher
	orchestrator *Orchestrator
	registry     *tools.Registry
	cfg          *config.Config
}

// Handle processes an analyze_media request.
func (h *AnalyzeMediaHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	input, err := requestToToolInput(req)
	if err != nil {
		return errorResult(output.ErrInvalidInput, err.Error()), nil
	}
	if input.URL == "" {
		return errorResult(output.ErrInvalidInput, "url is required"), nil
	}

	tool, ok := h.registry.Get("run_ffprobe")
	if !ok {
		return errorResult(output.ErrToolNotFound, "ffprobe tool not registered"), nil
	}

	ws, err := fetcher.NewWorkspace()
	if err != nil {
		return errorResult(output.ErrInternalFailure, fmt.Sprintf("create workspace: %v", err)), nil
	}
	defer ws.Cleanup()

	if err := h.fetcher.DownloadFile(ctx, input.URL, ws, "input", input.Headers); err != nil {
		return errorResult(output.ErrSegmentFetch, fmt.Sprintf("download: %v", err)), nil
	}

	result, err := h.orchestrator.ExecuteToolDirect(ctx, tool, input, ws.Dir)
	if err != nil {
		return errorResult(output.ErrInternalFailure, err.Error()), nil
	}

	return analysisResultToMCP(result)
}

// AnalyzeMP4StructureHandler downloads an asset and runs mp4dump.
type AnalyzeMP4StructureHandler struct {
	fetcher      *fetcher.Fetcher
	orchestrator *Orchestrator
	registry     *tools.Registry
	cfg          *config.Config
}

// Handle processes an analyze_mp4_structure request.
func (h *AnalyzeMP4StructureHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	input, err := requestToToolInput(req)
	if err != nil {
		return errorResult(output.ErrInvalidInput, err.Error()), nil
	}
	if input.URL == "" {
		return errorResult(output.ErrInvalidInput, "url is required"), nil
	}

	tool, ok := h.registry.Get("run_mp4dump")
	if !ok {
		return errorResult(output.ErrToolNotFound, "mp4dump tool not registered"), nil
	}

	ws, err := fetcher.NewWorkspace()
	if err != nil {
		return errorResult(output.ErrInternalFailure, fmt.Sprintf("create workspace: %v", err)), nil
	}
	defer ws.Cleanup()

	if err := h.fetcher.DownloadFile(ctx, input.URL, ws, "input", input.Headers); err != nil {
		return errorResult(output.ErrSegmentFetch, fmt.Sprintf("download: %v", err)), nil
	}

	result, err := h.orchestrator.ExecuteToolDirect(ctx, tool, input, ws.Dir)
	if err != nil {
		return errorResult(output.ErrInternalFailure, err.Error()), nil
	}

	return analysisResultToMCP(result)
}

// SimulatePlaybackHandler downloads an asset and runs ffmpeg -f null.
type SimulatePlaybackHandler struct {
	fetcher      *fetcher.Fetcher
	orchestrator *Orchestrator
	registry     *tools.Registry
	cfg          *config.Config
}

// Handle processes a simulate_playback request.
func (h *SimulatePlaybackHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	input, err := requestToToolInput(req)
	if err != nil {
		return errorResult(output.ErrInvalidInput, err.Error()), nil
	}
	if input.URL == "" {
		return errorResult(output.ErrInvalidInput, "url is required"), nil
	}

	tool, ok := h.registry.Get("run_ffmpeg")
	if !ok {
		return errorResult(output.ErrToolNotFound, "ffmpeg tool not registered"), nil
	}

	if input.Duration == 0 {
		input.Duration = h.cfg.Playback.DefaultDuration
	}

	ws, err := fetcher.NewWorkspace()
	if err != nil {
		return errorResult(output.ErrInternalFailure, fmt.Sprintf("create workspace: %v", err)), nil
	}
	defer ws.Cleanup()

	if err := h.fetcher.DownloadFile(ctx, input.URL, ws, "input", input.Headers); err != nil {
		return errorResult(output.ErrSegmentFetch, fmt.Sprintf("download: %v", err)), nil
	}

	result, err := h.orchestrator.ExecuteToolDirect(ctx, tool, input, ws.Dir)
	if err != nil {
		return errorResult(output.ErrInternalFailure, err.Error()), nil
	}

	return analysisResultToMCP(result)
}

// ValidatePackagingHandler downloads an asset and runs Shaka Packager.
type ValidatePackagingHandler struct {
	fetcher      *fetcher.Fetcher
	orchestrator *Orchestrator
	registry     *tools.Registry
	cfg          *config.Config
}

// Handle processes a validate_packaging request.
func (h *ValidatePackagingHandler) Handle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	input, err := requestToToolInput(req)
	if err != nil {
		return errorResult(output.ErrInvalidInput, err.Error()), nil
	}
	if input.URL == "" {
		return errorResult(output.ErrInvalidInput, "url is required"), nil
	}

	tool, ok := h.registry.Get("run_shaka_packager")
	if !ok {
		return errorResult(output.ErrToolNotFound, "shaka packager tool not registered"), nil
	}

	ws, err := fetcher.NewWorkspace()
	if err != nil {
		return errorResult(output.ErrInternalFailure, fmt.Sprintf("create workspace: %v", err)), nil
	}
	defer ws.Cleanup()

	if err := h.fetcher.DownloadFile(ctx, input.URL, ws, "input", input.Headers); err != nil {
		return errorResult(output.ErrSegmentFetch, fmt.Sprintf("download: %v", err)), nil
	}

	result, err := h.orchestrator.ExecuteToolDirect(ctx, tool, input, ws.Dir)
	if err != nil {
		return errorResult(output.ErrInternalFailure, err.Error()), nil
	}

	return analysisResultToMCP(result)
}

// httpGet performs a simple HTTP GET and returns the response body bytes.
func httpGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
