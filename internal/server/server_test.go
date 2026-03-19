package server

import (
	"testing"

	"github.com/meinart/video-debug-mcp/internal/config"
)

func TestNewServer(t *testing.T) {
	cfg := config.Default()
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestServer_ToolCount(t *testing.T) {
	cfg := config.Default()
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	count := srv.ToolCount()
	if count < 12 {
		t.Errorf("expected at least 12 registered tools, got %d", count)
	}
}

func TestServer_MCPServer(t *testing.T) {
	cfg := config.Default()
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	if srv.MCPServer() == nil {
		t.Fatal("expected non-nil MCPServer")
	}
}
