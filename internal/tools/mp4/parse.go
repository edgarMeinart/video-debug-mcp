package mp4

import (
	"bufio"
	"bytes"
	"regexp"
	"strconv"
	"strings"

	"github.com/meinart/video-debug-mcp/internal/output"
)

// Regexes for MP4Box -info output.
var (
	reMovieDuration  = regexp.MustCompile(`Timescale\s+(\d+)\s+-\s+Duration\s+([\d:\.]+)`)
	reTrackHeader    = regexp.MustCompile(`\*\s+Track\s+(\d+)\s+Info\s+\*`)
	reTrackTimeScale = regexp.MustCompile(`Track ID\s+\d+\s+-\s+TimeScale\s+(\d+)`)
	reMediaType      = regexp.MustCompile(`Media Type:\s+(\S+)\s+-\s+(\S+)`)
	reVisualSize     = regexp.MustCompile(`Visual Size\s+(\d+)\s+x\s+(\d+)`)
	reAVCProfile     = regexp.MustCompile(`AVC Profile\s+(\S+)\s+@\s+Level\s+(\d+)`)
	reAvgBitrate     = regexp.MustCompile(`Average Bitrate\s+(\d+)\s+kbps`)
	reMaxBitrate     = regexp.MustCompile(`Max Bitrate\s+(\d+)\s+kbps`)
	reCodecParams    = regexp.MustCompile(`Codec Parameters:\s+(\S+)`)
)

// parseMP4BoxOutput parses MP4Box -info text output into an AnalysisResult.
func parseMP4BoxOutput(raw []byte, toolName string) (*output.AnalysisResult, error) {
	result := &output.AnalysisResult{
		Tool: toolName,
	}

	scanner := bufio.NewScanner(bytes.NewReader(raw))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Parse movie-level info.
	timing := &output.TimingInfo{}
	for _, line := range lines {
		if m := reMovieDuration.FindStringSubmatch(line); m != nil {
			if ts, err := strconv.Atoi(m[1]); err == nil {
				timing.TimeBase = strconv.Itoa(ts)
			}
			timing.Duration = parseDurationHHMMSS(m[2])
		}
	}
	result.Timing = timing

	// Parse tracks — collect lines per track section.
	type trackSection struct {
		lines []string
	}
	var tracks []trackSection
	var current *trackSection

	for _, line := range lines {
		if reTrackHeader.MatchString(line) {
			tracks = append(tracks, trackSection{})
			current = &tracks[len(tracks)-1]
			continue
		}
		if current != nil {
			current.lines = append(current.lines, line)
		}
	}

	for i, track := range tracks {
		si := output.StreamInfo{Index: i}
		var avgBitrate, maxBitrate int64
		var codecProfile, codecLevel string

		for _, line := range track.lines {
			if m := reTrackTimeScale.FindStringSubmatch(line); m != nil {
				// timescale noted but not directly mapped to StreamInfo
				_ = m[1]
			}
			if m := reMediaType.FindStringSubmatch(line); m != nil {
				// m[1] like "vide:avc1", m[2] like "Visual"
				parts := strings.SplitN(m[1], ":", 2)
				handlerType := parts[0]
				switch handlerType {
				case "vide":
					si.Type = "video"
				case "soun":
					si.Type = "audio"
				default:
					si.Type = handlerType
				}
				if len(parts) == 2 {
					si.Codec = parts[1]
				}
			}
			if m := reVisualSize.FindStringSubmatch(line); m != nil {
				si.Width, _ = strconv.Atoi(m[1])
				si.Height, _ = strconv.Atoi(m[2])
			}
			if m := reAVCProfile.FindStringSubmatch(line); m != nil {
				codecProfile = m[1]
				codecLevel = m[2]
			}
			if m := reAvgBitrate.FindStringSubmatch(line); m != nil {
				if v, err := strconv.ParseInt(m[1], 10, 64); err == nil {
					avgBitrate = v * 1000 // kbps → bps
				}
			}
			if m := reMaxBitrate.FindStringSubmatch(line); m != nil {
				if v, err := strconv.ParseInt(m[1], 10, 64); err == nil {
					maxBitrate = v * 1000
				}
			}
			if m := reCodecParams.FindStringSubmatch(line); m != nil {
				if si.Codec == "" {
					si.Codec = m[1]
				}
			}
		}

		si.Bitrate = avgBitrate
		result.Streams = append(result.Streams, si)

		// Set codec details from the first video stream.
		if si.Type == "video" && result.Codec == nil && (codecProfile != "" || codecLevel != "") {
			result.Codec = &output.CodecDetails{
				Profile: codecProfile,
				Level:   codecLevel,
			}
		}

		// Set bitrate info.
		if avgBitrate > 0 || maxBitrate > 0 {
			result.Bitrate = &output.BitrateInfo{
				Average: avgBitrate,
				Max:     maxBitrate,
				Unit:    "bps",
			}
		}
	}

	return result, nil
}

// parseDurationHHMMSS converts "HH:MM:SS.mmm" to seconds as float64.
func parseDurationHHMMSS(s string) float64 {
	// Format: 00:02:00.500
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0
	}
	h, _ := strconv.ParseFloat(parts[0], 64)
	m, _ := strconv.ParseFloat(parts[1], 64)
	secParts := strings.Split(parts[2], ".")
	sec, _ := strconv.ParseFloat(secParts[0], 64)
	var ms float64
	if len(secParts) == 2 {
		ms, _ = strconv.ParseFloat(secParts[1], 64)
		// ms digits could be 1-3, normalize to milliseconds
		for ms >= 1000 {
			ms /= 10
		}
		ms /= 1000
	}
	return h*3600 + m*60 + sec + ms
}

// Regex for mp4dump box lines: optional leading spaces, [boxtype] size=N
var reBoxLine = regexp.MustCompile(`^(\s*)\[([^\]]+)\]\s+size=(\d+)`)

// parseMP4DumpOutput parses mp4dump text output into a BoxTree slice.
func parseMP4DumpOutput(raw []byte) []output.BoxTree {
	scanner := bufio.NewScanner(bytes.NewReader(raw))

	type stackEntry struct {
		depth int
		node  *output.BoxTree
	}

	var roots []output.BoxTree
	var stack []stackEntry

	for scanner.Scan() {
		line := scanner.Text()
		m := reBoxLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		indent := len(m[1])
		boxType := m[2]
		size, _ := strconv.ParseInt(m[3], 10, 64)

		node := output.BoxTree{
			Type: boxType,
			Size: size,
		}

		// Pop stack entries that are deeper than or equal to current indent.
		for len(stack) > 0 && stack[len(stack)-1].depth >= indent {
			stack = stack[:len(stack)-1]
		}

		if len(stack) == 0 {
			// Root-level box.
			roots = append(roots, node)
			// Push a pointer to the last root.
			stack = append(stack, stackEntry{depth: indent, node: &roots[len(roots)-1]})
		} else {
			// Child of the top of stack.
			parent := stack[len(stack)-1].node
			parent.Children = append(parent.Children, node)
			// Push pointer to the newly appended child.
			child := &parent.Children[len(parent.Children)-1]
			stack = append(stack, stackEntry{depth: indent, node: child})
		}
	}

	return roots
}
