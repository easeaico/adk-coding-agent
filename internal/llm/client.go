// Package llm provides wrapper interfaces and implementations for LLM interactions.
package llm

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// Embedder provides text embedding capability.
type Embedder interface {
	// Embed generates an embedding vector for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Client wraps the Google GenAI client and provides LLM interaction methods.
type Client struct {
	client         *genai.Client
	embeddingModel *genai.EmbeddingModel
	chatModel      *genai.GenerativeModel
}

// NewClient creates a new LLM client with the given API key.
func NewClient(ctx context.Context, apiKey string) (*Client, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &Client{
		client:         client,
		embeddingModel: client.EmbeddingModel("text-embedding-004"),
		chatModel:      client.GenerativeModel("gemini-1.5-flash"),
	}, nil
}

// Close releases resources held by the client.
func (c *Client) Close() error {
	return c.client.Close()
}

// Embed generates an embedding vector for the given text.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := c.embeddingModel.EmbedContent(ctx, genai.Text(text))
	if err != nil {
		return nil, fmt.Errorf("failed to embed content: %w", err)
	}

	if resp.Embedding == nil {
		return nil, fmt.Errorf("no embedding returned")
	}

	return resp.Embedding.Values, nil
}

// ChatModel returns the configured chat model.
func (c *Client) ChatModel() *genai.GenerativeModel {
	return c.chatModel
}

// ConfigureModel configures the chat model with system instruction and tools.
func (c *Client) ConfigureModel(systemInstruction string, tools []*genai.Tool) {
	c.chatModel.SystemInstruction = genai.NewUserContent(genai.Text(systemInstruction))
	c.chatModel.Tools = tools
}

// Ensure Client implements Embedder
var _ Embedder = (*Client)(nil)
