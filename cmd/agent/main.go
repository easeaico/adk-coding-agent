// Package main is the entry point for the Legacy Code Hunter agent.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/easeaico/adk-memory-agent/internal/llm"
	"github.com/easeaico/adk-memory-agent/internal/memory"
	"github.com/easeaico/adk-memory-agent/internal/service"
)

// Config holds the application configuration.
type Config struct {
	DatabaseURL string
	APIKey      string
	WorkDir     string
}

func main() {
	// Load configuration from environment
	cfg := loadConfig()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n正在关闭...")
		cancel()
	}()

	// Initialize components
	agent, cleanup, err := initializeAgent(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}
	defer cleanup()

	// Run interactive loop
	runInteractiveLoop(ctx, agent)
}

// loadConfig loads configuration from environment variables.
func loadConfig() Config {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		APIKey:      os.Getenv("GOOGLE_API_KEY"),
		WorkDir:     os.Getenv("WORK_DIR"),
	}

	// Set defaults
	if cfg.WorkDir == "" {
		cfg.WorkDir, _ = os.Getwd()
	}

	// Validate required config
	if cfg.APIKey == "" {
		log.Fatal("GOOGLE_API_KEY environment variable is required")
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required (e.g., postgres://user:pass@localhost:5432/dbname)")
	}

	return cfg
}

// initializeAgent creates and initializes all components.
func initializeAgent(ctx context.Context, cfg Config) (*service.Agent, func(), error) {
	// Connect to database
	store, err := memory.NewPostgresStore(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create LLM client
	llmClient, err := llm.NewClient(ctx, cfg.APIKey)
	if err != nil {
		store.Close()
		return nil, nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Create agent
	agent := service.NewAgent(llmClient, store, cfg.WorkDir)

	// Start session
	if err := agent.StartSession(ctx); err != nil {
		llmClient.Close()
		store.Close()
		return nil, nil, fmt.Errorf("failed to start session: %w", err)
	}

	// Create cleanup function
	cleanup := func() {
		agent.Close(ctx, true) // Consolidate memory on close
		llmClient.Close()
		store.Close()
	}

	return agent, cleanup, nil
}

// runInteractiveLoop runs the interactive chat loop.
func runInteractiveLoop(ctx context.Context, agent *service.Agent) {
	fmt.Println("========================================")
	fmt.Println("   遗留代码猎手 (Legacy Code Hunter)")
	fmt.Println("   基于 ADK 的分层记忆智能体")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("输入你的问题，我会帮你分析代码问题。")
	fmt.Println("输入 'exit' 或按 Ctrl+C 退出。")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("你: ")

		select {
		case <-ctx.Done():
			return
		default:
		}

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if strings.ToLower(input) == "exit" {
			fmt.Println("再见！")
			return
		}

		// Special commands
		if strings.HasPrefix(input, "/") {
			handleCommand(input)
			continue
		}

		// Send to agent
		response, err := agent.Chat(ctx, input)
		if err != nil {
			fmt.Printf("错误: %v\n", err)
			continue
		}

		fmt.Printf("\n助手: %s\n\n", response)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v", err)
	}
}

// handleCommand handles special commands.
func handleCommand(cmd string) {
	switch strings.ToLower(cmd) {
	case "/help":
		fmt.Print(`
可用命令:
  /help    - 显示帮助信息
  /clear   - 清除会话历史
  exit     - 退出程序

功能说明:
  - 我可以读取和分析代码文件
  - 我可以搜索历史问题库寻找类似问题
  - 解决问题后，经验会自动保存
`)
	case "/clear":
		fmt.Println("会话历史已清除。")
	default:
		fmt.Printf("未知命令: %s\n", cmd)
	}
}
