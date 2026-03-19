package docker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

// Runner is the interface for running commands in Docker containers.
type Runner interface {
	Run(ctx context.Context, req RunRequest) (*RunResult, error)
}

// RunRequest describes how to run a command in a container.
type RunRequest struct {
	Image        string
	Command      []string
	WorkspaceDir string
	Timeout      time.Duration
	MemoryLimit  int64
	CPULimit     float64
}

// Validate checks that all required fields are present.
func (r RunRequest) Validate() error {
	if r.Image == "" {
		return errors.New("docker: image is required")
	}
	if len(r.Command) == 0 {
		return errors.New("docker: command is required")
	}
	if r.WorkspaceDir == "" {
		return errors.New("docker: workspace directory is required")
	}
	return nil
}

// RunResult holds the output of a completed container run.
type RunResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Duration time.Duration
}

// Success returns true when the container exited with code 0.
func (r *RunResult) Success() bool {
	return r.ExitCode == 0
}

// DockerRunner implements Runner using the Docker SDK.
type DockerRunner struct {
	cli *client.Client
}

// NewDockerRunner creates a DockerRunner connected to the local Docker daemon.
func NewDockerRunner() (*DockerRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerRunner{cli: cli}, nil
}

// Run executes a command inside a fresh container and returns the result.
func (dr *DockerRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Apply timeout if specified.
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	// Build resource constraints.
	resources := container.Resources{}
	if req.MemoryLimit > 0 {
		resources.Memory = req.MemoryLimit
	}
	if req.CPULimit > 0 {
		// CPUQuota in microseconds per CPUPeriod (default 100ms).
		resources.CPUPeriod = 100000
		resources.CPUQuota = int64(req.CPULimit * 100000)
	}

	hostCfg := &container.HostConfig{
		AutoRemove:  true,
		NetworkMode: "none",
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeBind,
				Source:   req.WorkspaceDir,
				Target:   "/workspace",
				ReadOnly: true,
			},
		},
		Resources: resources,
	}

	resp, err := dr.cli.ContainerCreate(ctx, &container.Config{
		Image: req.Image,
		Cmd:   req.Command,
	}, hostCfg, nil, nil, "")
	if err != nil {
		return nil, err
	}

	start := time.Now()

	if err := dr.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, err
	}

	// Wait for the container to finish.
	statusCh, errCh := dr.cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	var exitCode int
	select {
	case err := <-errCh:
		if err != nil {
			return nil, err
		}
	case status := <-statusCh:
		exitCode = int(status.StatusCode)
	}

	duration := time.Since(start)

	// Collect logs.
	logReader, err := dr.cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return nil, err
	}
	defer logReader.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	// Docker multiplexes stdout/stderr with an 8-byte header per frame.
	// Use io.Copy as a simple fallback — for demuxed output use dockerstdcopy.
	if _, err := io.Copy(&stdoutBuf, logReader); err != nil {
		return nil, err
	}

	return &RunResult{
		ExitCode: exitCode,
		Stdout:   stdoutBuf.Bytes(),
		Stderr:   stderrBuf.Bytes(),
		Duration: duration,
	}, nil
}
