// Package main is the entry point for the Legacy Code Hunter agent.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	internal "github.com/easeaico/adk-memory-agent/internal/agent"
	"github.com/easeaico/adk-memory-agent/internal/config"
	"github.com/easeaico/adk-memory-agent/internal/memory"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
)

// main is the entry point for the Legacy Code Hunter agent application.
// It initializes all components (database, memory service, agent) and starts
// the interactive agent runtime using the ADK launcher.
func main() {
	// Load configuration from environment variables
	cfg := config.Load()

	// Create context with cancellation support for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown (SIGINT, SIGTERM)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n正在关闭...")
		cancel()
	}()

	// Initialize database connection pool
	// The store provides access to both semantic memory (project rules) and
	// episodic memory (past experiences) stored in PostgreSQL with pgvector.
	store, err := memory.NewPostgresStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer store.Close()

	// Create memory service that implements ADK's memory.Service interface
	// This service handles vector similarity search and session ingestion
	// for long-term knowledge storage.
	memoryService, err := memory.NewService(ctx, store, cfg)
	if err != nil {
		log.Fatalf("failed to create memory service: %v", err)
	}

	// Initialize the LLM agent with tools and system prompt
	// The agent is configured with project rules and has access to tools
	// for file operations and experience management.
	llmAgent, err := internal.NewCodingAgent(ctx, store, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}

	// Configure and start the ADK launcher
	// The launcher provides the interactive runtime environment for the agent,
	// handling command-line arguments and managing the agent lifecycle.
	launcherConfig := &launcher.Config{
		MemoryService: memoryService,
		AgentLoader:   agent.NewSingleLoader(llmAgent),
	}
	l := full.NewLauncher()
	if err := l.Execute(ctx, launcherConfig, os.Args[1:]); err != nil {
		log.Fatalf("Failed to run agent: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
