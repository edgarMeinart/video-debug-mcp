package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/meinart/video-debug-mcp/internal/config"
	"github.com/meinart/video-debug-mcp/internal/fetcher"
	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

func TestAnalyzeManifest_HLS(t *testing.T) {
	hlsContent := "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-STREAM-INF:BANDWIDTH=3000000,RESOLUTION=1280x720\n720p/playlist.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=1920x1080\n1080p/playlist.m3u8\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(hlsContent))
	}))
	defer ts.Close()

	cfg := config.Default()
	f := fetcher.NewFetcher(10, 30*time.Second, "test/1.0")
	formatter := output.NewFormatter()
	handler := &AnalyzeManifestHandler{fetcher: f, formatter: formatter, cfg: cfg}

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"url":       ts.URL + "/master.m3u8",
		"verbosity": "standard",
	}

	result, err := handler.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Handle() returned error result: %+v", result)
	}

	// Parse the text content back into an AnalysisResult.
	var analysisResult output.AnalysisResult
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	if err := json.Unmarshal([]byte(tc.Text), &analysisResult); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if analysisResult.Manifest == nil {
		t.Fatal("expected manifest summary, got nil")
	}
	if analysisResult.Manifest.VariantCount != 2 {
		t.Errorf("expected 2 variants, got %d", analysisResult.Manifest.VariantCount)
	}
	if analysisResult.Manifest.Type != "hls" {
		t.Errorf("expected type hls, got %s", analysisResult.Manifest.Type)
	}
}

func TestAnalyzeManifest_DASH(t *testing.T) {
	dashContent := `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="PT60S">
  <Period>
    <AdaptationSet mimeType="video/mp4" contentType="video">
      <SegmentTemplate media="video_$Number$.m4s" initialization="video_init.mp4" startNumber="1" duration="4000" timescale="1000"/>
      <Representation id="1" bandwidth="3000000" width="1280" height="720" codecs="avc1.64001f"/>
    </AdaptationSet>
  </Period>
</MPD>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(dashContent))
	}))
	defer ts.Close()

	cfg := config.Default()
	f := fetcher.NewFetcher(10, 30*time.Second, "test/1.0")
	formatter := output.NewFormatter()
	handler := &AnalyzeManifestHandler{fetcher: f, formatter: formatter, cfg: cfg}

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"url":       ts.URL + "/manifest.mpd",
		"verbosity": "standard",
	}

	result, err := handler.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Handle() returned error result: %+v", result)
	}

	var analysisResult output.AnalysisResult
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	if err := json.Unmarshal([]byte(tc.Text), &analysisResult); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if analysisResult.Manifest == nil {
		t.Fatal("expected manifest summary, got nil")
	}
	if analysisResult.Manifest.Type != "dash" {
		t.Errorf("expected type dash, got %s", analysisResult.Manifest.Type)
	}
	if analysisResult.Manifest.VariantCount != 1 {
		t.Errorf("expected 1 variant, got %d", analysisResult.Manifest.VariantCount)
	}
}

func TestAnalyzeManifest_InvalidURL(t *testing.T) {
	cfg := config.Default()
	f := fetcher.NewFetcher(10, 30*time.Second, "test/1.0")
	formatter := output.NewFormatter()
	handler := &AnalyzeManifestHandler{fetcher: f, formatter: formatter, cfg: cfg}

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"url": "",
	}

	result, err := handler.Handle(context.Background(), req)
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for empty URL")
	}
}

func TestRequestToToolInput(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"url":       "https://example.com/video.mp4",
		"verbosity": "deep",
		"duration":  float64(15),
	}

	input, err := requestToToolInput(req)
	if err != nil {
		t.Fatalf("requestToToolInput() error: %v", err)
	}
	if input.URL != "https://example.com/video.mp4" {
		t.Errorf("expected URL https://example.com/video.mp4, got %s", input.URL)
	}
	if input.Verbosity != "deep" {
		t.Errorf("expected verbosity deep, got %s", input.Verbosity)
	}
	if input.Duration != 15 {
		t.Errorf("expected duration 15, got %d", input.Duration)
	}
}

func TestErrorResult(t *testing.T) {
	result := errorResult(output.ErrInvalidInput, "test error")
	if !result.IsError {
		t.Error("expected IsError to be true")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	var errResp output.ErrorResponse
	if err := json.Unmarshal([]byte(tc.Text), &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if errResp.Code != output.ErrInvalidInput {
		t.Errorf("expected code %s, got %s", output.ErrInvalidInput, errResp.Code)
	}
}

// Ensure unused imports don't cause issues.
var _ = tools.ToolInput{}
