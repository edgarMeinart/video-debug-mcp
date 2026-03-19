package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/meinart/video-debug-mcp/internal/config"
	"github.com/meinart/video-debug-mcp/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	transport := flag.String("transport", "sse", "transport type: stdio or sse")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Printf("Warning: could not load config from %s: %v — using defaults", *configPath, err)
		cfg = config.Default()
		cfg.ApplyEnvOverrides()
	}

	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	log.Printf("Registered %d tools", srv.ToolCount())

	switch *transport {
	case "stdio":
		log.Printf("Starting video-debug-mcp server on stdio")
		if err := mcpserver.ServeStdio(srv.MCPServer()); err != nil {
			log.Fatalf("Stdio server error: %v", err)
		}
	case "sse":
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		log.Printf("Starting video-debug-mcp server on %s", addr)
		sseServer := mcpserver.NewSSEServer(srv.MCPServer())
		log.Fatal(http.ListenAndServe(addr, sseServer))
	default:
		log.Fatalf("Unknown transport %q: use 'stdio' or 'sse'", *transport)
	}
}
