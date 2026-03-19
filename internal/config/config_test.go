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
		t.Errorf("expected max_concurrent=5, got %d", cfg.Fetcher.MaxConcurrentDownloads)
	}
	if cfg.Output.DefaultVerbosity != "deep" {
		t.Errorf("expected verbosity=deep, got %s", cfg.Output.DefaultVerbosity)
	}
	if cfg.Playback.DefaultDuration != 20 {
		t.Errorf("expected duration=20, got %d", cfg.Playback.DefaultDuration)
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
		t.Errorf("expected default max_concurrent=10, got %d", cfg.Fetcher.MaxConcurrentDownloads)
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
		t.Errorf("expected port=3000, got %d", cfg.Server.Port)
	}
	if cfg.Fetcher.MaxConcurrentDownloads != 20 {
		t.Errorf("expected max_concurrent=20, got %d", cfg.Fetcher.MaxConcurrentDownloads)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
