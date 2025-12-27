package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/easeaico/adk-memory-agent/internal/llm"
	"github.com/easeaico/adk-memory-agent/internal/memory"
	"github.com/easeaico/adk-memory-agent/internal/tools"
	"github.com/google/generative-ai-go/genai"
)

// Agent represents the Legacy Code Hunter agent with tiered memory.
type Agent struct {
	llmClient    *llm.Client
	memoryStore  memory.Store
	toolHandler  *tools.Handler
	agentContext *AgentContext
	chatSession  *genai.ChatSession
}

// NewAgent creates a new agent with the given dependencies.
func NewAgent(llmClient *llm.Client, memoryStore memory.Store, workDir string) *Agent {
	return &Agent{
		llmClient:    llmClient,
		memoryStore:  memoryStore,
		toolHandler:  tools.NewHandler(memoryStore, llmClient, workDir),
		agentContext: NewAgentContext(),
	}
}

// StartSession initializes a new chat session with loaded project rules.
func (a *Agent) StartSession(ctx context.Context) error {
	// Load semantic memory (project rules)
	rules, err := a.memoryStore.GetProjectRules(ctx)
	if err != nil {
		return fmt.Errorf("failed to load project rules: %w", err)
	}
	a.agentContext.GlobalRules = rules

	// Build system instruction with rules
	sysPrompt := a.buildSystemPrompt()

	// Configure the model
	a.llmClient.ConfigureModel(sysPrompt, tools.AllTools())

	// Start chat session
	a.chatSession = a.llmClient.ChatModel().StartChat()
	a.chatSession.History = a.agentContext.SessionHistory

	log.Printf("Session started with %d project rules loaded", len(rules))
	return nil
}

// buildSystemPrompt constructs the system prompt with project rules.
func (a *Agent) buildSystemPrompt() string {
	var sb strings.Builder

	sb.WriteString(`你是一个资深的 Go 工程师，名为"遗留代码猎手"(Legacy Code Hunter)。
你的任务是帮助开发者理解、调试和修复代码问题。

你具备以下能力：
1. 可以读取文件内容来理解代码
2. 可以搜索历史问题库来查找相似问题的解决方案
3. 可以保存新的问题解决经验供将来参考

`)

	if len(a.agentContext.GlobalRules) > 0 {
		sb.WriteString("你必须严格遵守以下项目规范：\n")
		for i, rule := range a.agentContext.GlobalRules {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, rule))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`在回答问题时：
- 首先考虑是否需要搜索历史问题库
- 如果需要查看代码，使用 read_file_content 工具
- 解决问题后，使用 save_experience 工具保存经验
- 始终提供清晰、可操作的建议
`)

	return sb.String()
}

// Chat sends a user message and returns the agent's response.
func (a *Agent) Chat(ctx context.Context, userMessage string) (string, error) {
	if a.chatSession == nil {
		if err := a.StartSession(ctx); err != nil {
			return "", err
		}
	}

	// Send user message
	resp, err := a.chatSession.SendMessage(ctx, genai.Text(userMessage))
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	// Process response and handle tool calls
	return a.processResponse(ctx, resp)
}

// processResponse handles the model response and any tool calls.
func (a *Agent) processResponse(ctx context.Context, resp *genai.GenerateContentResponse) (string, error) {
	var result strings.Builder

	for _, candidate := range resp.Candidates {
		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			switch p := part.(type) {
			case genai.Text:
				result.WriteString(string(p))

			case genai.FunctionCall:
				// Handle tool call
				toolResult, err := a.handleToolCall(ctx, p)
				if err != nil {
					log.Printf("Tool call error: %v", err)
					continue
				}

				// Send tool result back to model
				funcResp, err := a.chatSession.SendMessage(ctx,
					genai.FunctionResponse{
						Name:     p.Name,
						Response: map[string]interface{}{"result": toolResult},
					})
				if err != nil {
					return "", fmt.Errorf("failed to send tool response: %w", err)
				}

				// Recursively process the new response
				followUp, err := a.processResponse(ctx, funcResp)
				if err != nil {
					return "", err
				}
				result.WriteString(followUp)
			}
		}
	}

	return result.String(), nil
}

// handleToolCall dispatches a tool call to the appropriate handler.
func (a *Agent) handleToolCall(ctx context.Context, fc genai.FunctionCall) (string, error) {
	// Convert args to map[string]interface{}
	args := make(map[string]interface{})
	for k, v := range fc.Args {
		args[k] = v
	}

	log.Printf("Executing tool: %s with args: %v", fc.Name, args)

	return a.toolHandler.HandleToolCall(ctx, fc.Name, args)
}

// ConsolidateMemory summarizes the session and saves the experience.
func (a *Agent) ConsolidateMemory(ctx context.Context) error {
	if len(a.chatSession.History) < 2 {
		return nil // Not enough history to consolidate
	}

	// Build a summary prompt
	var historyText strings.Builder
	for _, content := range a.chatSession.History {
		for _, part := range content.Parts {
			if text, ok := part.(genai.Text); ok {
				historyText.WriteString(fmt.Sprintf("[%s]: %s\n", content.Role, string(text)))
			}
		}
	}

	summarizePrompt := fmt.Sprintf(`请分析以下对话，如果其中包含了解决问题的经验，请提取出来。
返回 JSON 格式：{"has_experience": bool, "error_pattern": string, "root_cause": string, "solution": string}
如果没有值得记录的经验，has_experience 设为 false。

对话历史：
%s`, historyText.String())

	// Use a fresh model call for summarization
	model := a.llmClient.ChatModel()
	resp, err := model.GenerateContent(ctx, genai.Text(summarizePrompt))
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	// Parse the summary
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil
	}

	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			var summary struct {
				HasExperience bool   `json:"has_experience"`
				ErrorPattern  string `json:"error_pattern"`
				RootCause     string `json:"root_cause"`
				Solution      string `json:"solution"`
			}

			if err := json.Unmarshal([]byte(text), &summary); err != nil {
				log.Printf("Failed to parse summary: %v", err)
				continue
			}

			if summary.HasExperience && summary.ErrorPattern != "" {
				// Generate embedding and save
				embedding, err := a.llmClient.Embed(ctx, summary.ErrorPattern)
				if err != nil {
					log.Printf("Failed to embed: %v", err)
					continue
				}

				if err := a.memoryStore.SaveExperience(ctx, summary.ErrorPattern, summary.RootCause, summary.Solution, embedding); err != nil {
					log.Printf("Failed to save experience: %v", err)
				} else {
					log.Printf("Experience consolidated successfully")
				}
			}
		}
	}

	return nil
}

// Close releases resources and optionally consolidates memory.
func (a *Agent) Close(ctx context.Context, consolidate bool) error {
	if consolidate {
		if err := a.ConsolidateMemory(ctx); err != nil {
			log.Printf("Warning: failed to consolidate memory: %v", err)
		}
	}
	return nil
}
