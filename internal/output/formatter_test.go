package output

import (
	"encoding/json"
	"testing"
)

func TestFormatter_Summary(t *testing.T) {
	f := NewFormatter()
	result := &AnalysisResult{
		Tool: "ffprobe", Command: "ffprobe ...", InputURL: "https://example.com/video.mp4",
		Streams: []StreamInfo{{Index: 0, Type: "video", Codec: "h264", Width: 1920, Height: 1080}, {Index: 1, Type: "audio", Codec: "aac", SampleRate: 48000, Channels: 2}},
		Timing: &TimingInfo{Duration: 120.5, StartTime: 0},
		Anomalies: []Anomaly{{Severity: SeverityWarning, Description: "PTS discontinuity"}},
		RawOutput: "very long raw output here",
	}
	formatted := f.Format(result, VerbositySummary)
	var m map[string]interface{}
	data, _ := json.Marshal(formatted)
	json.Unmarshal(data, &m)
	if m["tool"] != "ffprobe" { t.Errorf("expected tool=ffprobe, got %v", m["tool"]) }
	if _, ok := m["command"]; ok { t.Error("summary should not include command") }
	if _, ok := m["raw_output"]; ok { t.Error("summary should not include raw_output") }
}

func TestFormatter_Standard(t *testing.T) {
	f := NewFormatter()
	result := &AnalysisResult{
		Tool: "ffprobe", Command: "ffprobe ...", InputURL: "https://example.com/video.mp4",
		Streams: []StreamInfo{{Index: 0, Type: "video", Codec: "h264"}},
		RawOutput: "raw output",
	}
	formatted := f.Format(result, VerbosityStandard)
	var m map[string]interface{}
	data, _ := json.Marshal(formatted)
	json.Unmarshal(data, &m)
	if _, ok := m["command"]; !ok { t.Error("standard should include command") }
	if _, ok := m["raw_output"]; ok { t.Error("standard should not include raw_output") }
}

func TestFormatter_Forensic(t *testing.T) {
	f := NewFormatter()
	result := &AnalysisResult{
		Tool: "ffprobe", Command: "ffprobe ...", InputURL: "https://example.com/video.mp4",
		RawOutput: "full raw output dump",
	}
	formatted := f.Format(result, VerbosityForensic)
	var m map[string]interface{}
	data, _ := json.Marshal(formatted)
	json.Unmarshal(data, &m)
	if _, ok := m["raw_output"]; !ok { t.Error("forensic should include raw_output") }
}
