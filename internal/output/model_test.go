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
			{Index: 0, Type: "video", Codec: "h264", Width: 1920, Height: 1080, Bitrate: 5000000, FrameRate: 29.97},
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
	if m["tool"] != "ffprobe" {
		t.Errorf("expected tool=ffprobe, got %v", m["tool"])
	}
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
	a := Anomaly{Severity: SeverityWarning, Description: "PTS discontinuity detected", Detail: "Jump of 5.2s at segment boundary"}
	if a.Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %v", a.Severity)
	}
}

func TestErrorResponse_JSON(t *testing.T) {
	e := &ErrorResponse{Code: ErrManifestFetch, Message: "Failed to download manifest", Phase: "fetch", URL: "https://example.com/master.m3u8"}
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
	if _, ok := m["details"]; ok {
		t.Error("expected details to be omitted")
	}
}
