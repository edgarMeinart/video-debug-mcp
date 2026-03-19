package shaka

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/meinart/video-debug-mcp/internal/output"
)

var (
	// trackTypeRe matches lines like: "Track type: video, codec: avc1"
	trackTypeRe = regexp.MustCompile(`(?i)Track type:\s*(\w+)(?:,\s*codec:\s*(\S+))?`)

	// warningRe matches Shaka WARNING log lines.
	warningRe = regexp.MustCompile(`\[[\d/]+:WARNING:[^\]]+\]\s*(.+)`)

	// errorRe matches Shaka ERROR log lines.
	errorRe = regexp.MustCompile(`\[[\d/]+:ERROR:[^\]]+\]\s*(.+)`)
)

// parseShakaOutput parses Shaka Packager log output into an AnalysisResult.
func parseShakaOutput(raw []byte, toolName string) (*output.AnalysisResult, error) {
	if raw == nil {
		return nil, fmt.Errorf("shaka: nil output")
	}

	result := &output.AnalysisResult{
		Tool:      toolName,
		RawOutput: string(raw),
	}

	scanner := bufio.NewScanner(bytes.NewReader(raw))
	streamIndex := 0

	for scanner.Scan() {
		line := scanner.Text()

		// Detect track/stream lines.
		if m := trackTypeRe.FindStringSubmatch(line); m != nil {
			trackType := strings.ToLower(m[1])
			codec := ""
			if len(m) > 2 {
				codec = strings.TrimSuffix(m[2], ",")
			}
			si := output.StreamInfo{
				Index: streamIndex,
				Type:  trackType,
				Codec: codec,
			}
			result.Streams = append(result.Streams, si)
			streamIndex++
			continue
		}

		// Detect WARNING anomalies.
		if m := warningRe.FindStringSubmatch(line); m != nil {
			result.Anomalies = append(result.Anomalies, output.Anomaly{
				Severity:    output.SeverityWarning,
				Description: strings.TrimSpace(m[1]),
			})
			continue
		}

		// Detect ERROR anomalies.
		if m := errorRe.FindStringSubmatch(line); m != nil {
			result.Anomalies = append(result.Anomalies, output.Anomaly{
				Severity:    output.SeverityError,
				Description: strings.TrimSpace(m[1]),
			})
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("shaka: scanning output: %w", err)
	}

	return result, nil
}
