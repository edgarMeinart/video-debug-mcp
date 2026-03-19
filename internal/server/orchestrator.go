package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/meinart/video-debug-mcp/internal/config"
	"github.com/meinart/video-debug-mcp/internal/docker"
	"github.com/meinart/video-debug-mcp/internal/output"
	"github.com/meinart/video-debug-mcp/internal/tools"
)

type Orchestrator struct {
	Runner    docker.Runner
	formatter *output.Formatter
	cfg       *config.Config
}

func NewOrchestrator(runner docker.Runner, formatter *output.Formatter, cfg *config.Config) *Orchestrator {
	return &Orchestrator{Runner: runner, formatter: formatter, cfg: cfg}
}

func (o *Orchestrator) ExecuteToolDirect(ctx context.Context, tool tools.Tool, input tools.ToolInput, workspaceDir string) (*output.AnalysisResult, error) {
	cmd, err := tool.BuildCommand(input)
	if err != nil {
		return nil, fmt.Errorf("build command: %w", err)
	}

	timeout := o.cfg.Docker.Defaults.Timeout
	if toolCfg, ok := o.cfg.Docker.PerTool[tool.Name()]; ok && toolCfg.Timeout > 0 {
		timeout = toolCfg.Timeout
	}

	fullCmd := append([]string{cmd.Binary}, cmd.Args...)
	runReq := docker.RunRequest{
		Image:        tool.DockerImage(),
		Command:      fullCmd,
		WorkspaceDir: workspaceDir,
		Timeout:      timeout,
	}

	result, err := o.Runner.Run(ctx, runReq)
	if err != nil {
		return nil, fmt.Errorf("run container: %w", err)
	}

	if !result.Success() {
		return nil, fmt.Errorf("tool %s exited with code %d: %s", tool.Name(), result.ExitCode, string(result.Stderr))
	}

	verbosity, err := input.ParsedVerbosity()
	if err != nil {
		verbosity = output.VerbosityStandard
	}

	rawOutput := result.Stdout
	if len(rawOutput) == 0 {
		rawOutput = result.Stderr
	}

	analysisResult, err := tool.ParseOutput(rawOutput, verbosity)
	if err != nil {
		return nil, fmt.Errorf("parse output: %w", err)
	}

	analysisResult.Command = strings.Join(fullCmd, " ")
	analysisResult.InputURL = input.URL

	return o.formatter.Format(analysisResult, verbosity), nil
}
