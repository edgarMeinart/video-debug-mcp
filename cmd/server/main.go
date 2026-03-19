package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	mcphttp "github.com/mark3labs/mcp-go/server"

	"github.com/meinart/video-debug-mcp/internal/config"
	"github.com/meinart/video-debug-mcp/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
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

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Starting video-debug-mcp server on %s", addr)
	log.Printf("Registered %d tools", srv.ToolCount())

	sseServer := mcphttp.NewSSEServer(srv.MCPServer())
	log.Fatal(http.ListenAndServe(addr, sseServer))
}
