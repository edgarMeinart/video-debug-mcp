package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration that can be unmarshaled from a YAML string like "30s".
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = dur
	return nil
}

// Config is the top-level configuration structure.
type Config struct {
	Server   ServerConfig  `yaml:"server"`
	Docker   DockerConfig  `yaml:"docker"`
	Fetcher  FetcherConfig `yaml:"fetcher"`
	Output   OutputConfig  `yaml:"output"`
	Playback PlaybackConfig `yaml:"playback"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

// DockerConfig groups all Docker-related settings.
type DockerConfig struct {
	Images   DockerImages          `yaml:"images"`
	Defaults DockerDefaults        `yaml:"defaults"`
	PerTool  map[string]ToolConfig `yaml:"per_tool"`
}

// DockerImages holds the Docker image references for each tool.
type DockerImages struct {
	FFmpeg    string `yaml:"ffmpeg"`
	MP4       string `yaml:"mp4"`
	Bento4    string `yaml:"bento4"`
	MediaInfo string `yaml:"mediainfo"`
	Shaka     string `yaml:"shaka"`
}

// DockerDefaults holds default resource limits for Docker containers.
type DockerDefaults struct {
	Timeout     time.Duration
	MemoryLimit string `yaml:"memory_limit"`
	CPULimit    string `yaml:"cpu_limit"`
}

type dockerDefaultsYAML struct {
	Timeout     Duration `yaml:"timeout"`
	MemoryLimit string   `yaml:"memory_limit"`
	CPULimit    string   `yaml:"cpu_limit"`
}

func (d *DockerDefaults) UnmarshalYAML(value *yaml.Node) error {
	var raw dockerDefaultsYAML
	if err := value.Decode(&raw); err != nil {
		return err
	}
	d.Timeout = raw.Timeout.Duration
	d.MemoryLimit = raw.MemoryLimit
	d.CPULimit = raw.CPULimit
	return nil
}

// ToolConfig holds per-tool Docker overrides.
type ToolConfig struct {
	Timeout     time.Duration
	MemoryLimit string `yaml:"memory_limit"`
}

type toolConfigYAML struct {
	Timeout     Duration `yaml:"timeout"`
	MemoryLimit string   `yaml:"memory_limit"`
}

func (tc *ToolConfig) UnmarshalYAML(value *yaml.Node) error {
	var raw toolConfigYAML
	if err := value.Decode(&raw); err != nil {
		return err
	}
	tc.Timeout = raw.Timeout.Duration
	tc.MemoryLimit = raw.MemoryLimit
	return nil
}

// FetcherConfig holds settings for the asset fetcher.
type FetcherConfig struct {
	MaxConcurrentDownloads int           `yaml:"max_concurrent_downloads"`
	SegmentSampleCount     int           `yaml:"segment_sample_count"`
	RequestTimeout         time.Duration
	MaxAssetSize           string `yaml:"max_asset_size"`
	UserAgent              string `yaml:"user_agent"`
}

type fetcherConfigYAML struct {
	MaxConcurrentDownloads int      `yaml:"max_concurrent_downloads"`
	SegmentSampleCount     int      `yaml:"segment_sample_count"`
	RequestTimeout         Duration `yaml:"request_timeout"`
	MaxAssetSize           string   `yaml:"max_asset_size"`
	UserAgent              string   `yaml:"user_agent"`
}

func (f *FetcherConfig) UnmarshalYAML(value *yaml.Node) error {
	var raw fetcherConfigYAML
	if err := value.Decode(&raw); err != nil {
		return err
	}
	f.MaxConcurrentDownloads = raw.MaxConcurrentDownloads
	f.SegmentSampleCount = raw.SegmentSampleCount
	f.RequestTimeout = raw.RequestTimeout.Duration
	f.MaxAssetSize = raw.MaxAssetSize
	f.UserAgent = raw.UserAgent
	return nil
}

// OutputConfig controls output formatting.
type OutputConfig struct {
	DefaultVerbosity string `yaml:"default_verbosity"`
}

// PlaybackConfig holds playback analysis settings.
type PlaybackConfig struct {
	DefaultDuration int `yaml:"default_duration"`
}

// Default returns a Config populated with sensible defaults.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8080,
			Host: "0.0.0.0",
		},
		Docker: DockerConfig{
			Images: DockerImages{
				FFmpeg:    "jrottenberg/ffmpeg:4.4-alpine",
				MP4:       "alfg/mp4box",
				Bento4:    "axiomatic/bento4",
				MediaInfo: "jrottenberg/mediainfo",
				Shaka:     "google/shaka-packager",
			},
			Defaults: DockerDefaults{
				Timeout:     60 * time.Second,
				MemoryLimit: "512m",
				CPULimit:    "1.0",
			},
			PerTool: map[string]ToolConfig{},
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

// Load reads a YAML config file from path and returns a Config.
// Fields not present in the file retain their zero values; callers should
// merge with Default() if fallback values are needed.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}
	return cfg, nil
}

// ApplyEnvOverrides reads well-known environment variables and overwrites the
// corresponding fields on the receiver.  Only variables that are set (non-empty)
// are applied.
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
