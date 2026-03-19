package docker

import (
	"context"
	"testing"
	"time"
)

func TestRunRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     RunRequest
		wantErr bool
	}{
		{"valid request", RunRequest{Image: "video-debug/ffmpeg-tools", Command: []string{"ffprobe", "-v", "quiet"}, WorkspaceDir: "/tmp/test", Timeout: 60 * time.Second}, false},
		{"missing image", RunRequest{Command: []string{"ffprobe"}, WorkspaceDir: "/tmp/test", Timeout: 60 * time.Second}, true},
		{"missing command", RunRequest{Image: "video-debug/ffmpeg-tools", WorkspaceDir: "/tmp/test", Timeout: 60 * time.Second}, true},
		{"missing workspace", RunRequest{Image: "video-debug/ffmpeg-tools", Command: []string{"ffprobe"}, Timeout: 60 * time.Second}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunResult_Success(t *testing.T) {
	r := &RunResult{ExitCode: 0, Stdout: []byte("output"), Duration: 2 * time.Second}
	if !r.Success() {
		t.Error("expected Success() = true for exit code 0")
	}
}

func TestRunResult_Failure(t *testing.T) {
	r := &RunResult{ExitCode: 1, Stderr: []byte("error"), Duration: 1 * time.Second}
	if r.Success() {
		t.Error("expected Success() = false for exit code 1")
	}
}

type mockDockerClient struct {
	runFunc func(ctx context.Context, req RunRequest) (*RunResult, error)
}

func (m *mockDockerClient) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	return m.runFunc(ctx, req)
}

func TestRunner_Interface(t *testing.T) {
	mock := &mockDockerClient{
		runFunc: func(ctx context.Context, req RunRequest) (*RunResult, error) {
			return &RunResult{ExitCode: 0, Stdout: []byte("mock output"), Duration: 100 * time.Millisecond}, nil
		},
	}
	var runner Runner = mock
	result, err := runner.Run(context.Background(), RunRequest{Image: "test", Command: []string{"echo"}, WorkspaceDir: "/tmp", Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Stdout) != "mock output" {
		t.Errorf("expected 'mock output', got %q", string(result.Stdout))
	}
}
