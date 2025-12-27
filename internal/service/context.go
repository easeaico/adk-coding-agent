// Package service provides the core agent logic and session management.
package service

import (
	"github.com/easeaico/adk-memory-agent/internal/memory"
	"github.com/google/generative-ai-go/genai"
)

// AgentContext maintains the different layers of memory for a session.
type AgentContext struct {
	// GlobalRules are static rules injected into the system prompt.
	GlobalRules []string

	// RelevantHistory contains dynamically retrieved historical experiences.
	RelevantHistory []memory.Experience

	// SessionHistory maintains the current conversation history.
	SessionHistory []*genai.Content
}

// NewAgentContext creates a new empty agent context.
func NewAgentContext() *AgentContext {
	return &AgentContext{
		GlobalRules:     make([]string, 0),
		RelevantHistory: make([]memory.Experience, 0),
		SessionHistory:  make([]*genai.Content, 0),
	}
}

// AddUserMessage adds a user message to the session history.
func (c *AgentContext) AddUserMessage(text string) {
	c.SessionHistory = append(c.SessionHistory, &genai.Content{
		Role:  "user",
		Parts: []genai.Part{genai.Text(text)},
	})
}

// AddModelResponse adds a model response to the session history.
func (c *AgentContext) AddModelResponse(parts ...genai.Part) {
	c.SessionHistory = append(c.SessionHistory, &genai.Content{
		Role:  "model",
		Parts: parts,
	})
}

// ClearHistory clears the session history while preserving rules.
func (c *AgentContext) ClearHistory() {
	c.SessionHistory = make([]*genai.Content, 0)
	c.RelevantHistory = make([]memory.Experience, 0)
}
