package output

type Formatter struct{}

func NewFormatter() *Formatter { return &Formatter{} }

func (f *Formatter) Format(result *AnalysisResult, verbosity Verbosity) *AnalysisResult {
	switch verbosity {
	case VerbositySummary:
		return f.formatSummary(result)
	case VerbosityStandard:
		return f.formatStandard(result)
	case VerbosityDeep:
		return f.formatDeep(result)
	case VerbosityForensic:
		return f.formatForensic(result)
	default:
		return f.formatStandard(result)
	}
}

func (f *Formatter) formatSummary(r *AnalysisResult) *AnalysisResult {
	return &AnalysisResult{Tool: r.Tool, InputURL: r.InputURL, Streams: r.Streams, Timing: r.Timing, Bitrate: r.Bitrate, Anomalies: r.Anomalies}
}

func (f *Formatter) formatStandard(r *AnalysisResult) *AnalysisResult {
	return &AnalysisResult{Tool: r.Tool, Command: r.Command, InputURL: r.InputURL, ResolvedURLs: r.ResolvedURLs, Downloads: r.Downloads, Manifest: r.Manifest, Streams: r.Streams, Codec: r.Codec, Timing: r.Timing, Bitrate: r.Bitrate, Anomalies: r.Anomalies}
}

func (f *Formatter) formatDeep(r *AnalysisResult) *AnalysisResult {
	return &AnalysisResult{Tool: r.Tool, Command: r.Command, InputURL: r.InputURL, ResolvedURLs: r.ResolvedURLs, Downloads: r.Downloads, Manifest: r.Manifest, Streams: r.Streams, Codec: r.Codec, Timing: r.Timing, Bitrate: r.Bitrate, ContainerLayout: r.ContainerLayout, Anomalies: r.Anomalies}
}

func (f *Formatter) formatForensic(r *AnalysisResult) *AnalysisResult { return r }
