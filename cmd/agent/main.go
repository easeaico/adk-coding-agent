// Package main is the entry point for the Legacy Code Hunter agent.
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"text/template"

	"github.com/easeaico/adk-memory-agent/internal/memory"
	"github.com/easeaico/adk-memory-agent/internal/tools"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

// Config holds the application configuration.
type Config struct {
	DatabaseURL string
	APIKey      string
	WorkDir     string
}

// Embedder wraps the genai client for embedding generation.
type Embedder struct {
	client *genai.Client
}

// Embed generates an embedding for the given text.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := e.client.Models.EmbedContent(ctx, "text-embedding-004", genai.Text(text), nil)
	if err != nil {
		return nil, err
	}
	return resp.Embeddings[0].Values, nil
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
	llmAgent, cleanup, err := initializeAgent(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}
	defer cleanup()

	// Run interactive loop using adk-go runtime (launcher)
	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(llmAgent),
	}
	l := full.NewLauncher()
	if err := l.Execute(ctx, config, os.Args[1:]); err != nil {
		log.Fatalf("Failed to run agent: %v\n\n%s", err, l.CommandLineSyntax())
	}
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
func initializeAgent(ctx context.Context, cfg Config) (agent.Agent, func(), error) {
	// Create GenAI client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	// Create embedder
	embedder := &Embedder{client: client}

	// Connect to database with embedder for memory.Service support
	store, err := memory.NewPostgresStore(ctx, cfg.DatabaseURL, embedder)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Load project rules for system prompt
	rules, err := store.GetProjectRules(ctx)
	if err != nil {
		log.Printf("Warning: failed to load project rules: %v", err)
	}

	// Build system instruction
	systemPrompt := buildSystemPrompt(rules)

	// Create tools
	agentTools, err := tools.BuildTools(tools.ToolsConfig{
		Store:    store,
		Embedder: embedder,
		WorkDir:  cfg.WorkDir,
	})
	if err != nil {
		store.Close()
		return nil, nil, fmt.Errorf("failed to build tools: %w", err)
	}

	// Create LLM model using ADK's gemini wrapper
	llmModel, err := gemini.NewModel(ctx, "gemini-2.0-flash", &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		store.Close()
		return nil, nil, fmt.Errorf("failed to create LLM model: %w", err)
	}

	// Create LLM agent
	llmAgent, err := llmagent.New(llmagent.Config{
		Name:        "legacy_code_hunter",
		Description: "帮助开发者理解、调试和修复代码问题的智能助手",
		Model:       llmModel,
		Instruction: systemPrompt,
		Tools:       agentTools,
	})
	if err != nil {
		store.Close()
		return nil, nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Create cleanup function
	cleanup := func() {
		store.Close()
	}

	log.Printf("Agent initialized with %d project rules loaded", len(rules))
	return llmAgent, cleanup, nil
}

var systemPromptTmpl = template.Must(template.New("systemPrompt").Parse(`
你是一个资深的 Go 工程师，名为"遗留代码猎手"(Legacy Code Hunter)。
你的任务是帮助开发者理解、调试和修复代码问题。

你具备以下能力：
1. 可以读取文件内容来理解代码
2. 可以搜索历史问题库来查找相似问题的解决方案
3. 可以保存新的问题解决经验供将来参考

{{- if .HasRules }}

你必须严格遵守以下项目规范：
{{- range $idx, $rule := .Rules }}
{{$add := inc $idx}}{{printf "%d. %s" $add $rule}}
{{end}}
{{end}}

在回答问题时：
- 首先考虑是否需要搜索历史问题库
- 如果需要查看代码，使用 read_file_content 工具
- 解决问题后，使用 save_experience 工具保存经验
- 始终提供清晰、可操作的建议
`))

// inc is a small helper for incrementing index
func inc(i int) int { return i + 1 }

// buildSystemPrompt constructs the system prompt with project rules.
func buildSystemPrompt(rules []string) string {
	data := struct {
		Rules    []string
		HasRules bool
	}{
		Rules:    rules,
		HasRules: len(rules) > 0,
	}

	// Add funcMap for inc
	tmpl := systemPromptTmpl.Funcs(template.FuncMap{"inc": inc})

	var buf bytes.Buffer
	_ = tmpl.Execute(&buf, data)
	return buf.String()
}
