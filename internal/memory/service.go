// Package memory provides memory storage and retrieval services for the agent.
// It implements a tiered memory architecture with:
// - Semantic memory: Project rules and guidelines stored in the database
// - Episodic memory: Past experiences and solutions with vector similarity search
// The package implements ADK's memory.Service interface for integration with
// the ADK framework.
package memory

import (
	"context"
	"fmt"
	"strings"

	adkmemory "google.golang.org/adk/memory"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/easeaico/adk-memory-agent/internal/config"
)

// Embedder wraps the genai client for generating text embeddings.
// It uses Google's text-embedding-004 model to convert text into vector
// representations for similarity search.
type Embedder struct {
	client *genai.Client
}

// NewEmbedder creates a new Embedder with the given GenAI client.
func NewEmbedder(client *genai.Client) *Embedder {
	return &Embedder{client: client}
}

// Embed generates a vector embedding for the given text using the text-embedding-004 model.
// The returned vector can be used for similarity search in the vector database.
// Returns an error if the embedding generation fails.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := e.client.Models.EmbedContent(ctx, "text-embedding-004", genai.Text(text), nil)
	if err != nil {
		return nil, err
	}
	return resp.Embeddings[0].Values, nil
}

// EmbedderInterface defines the interface for embedding generation.
// This allows for easier testing by using mock implementations.
type EmbedderInterface interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Service implements ADK's memory.Service interface, providing vector similarity search
// and session ingestion capabilities for the agent's long-term memory.
// It combines a Store for database operations and an Embedder for text-to-vector conversion.
type Service struct {
	store    Store             // Database store for memory operations
	embedder EmbedderInterface // Embedder for generating query vectors in Search and AddSession
}

// NewService creates a new memory service with the given store and embedder.
func NewService(ctx context.Context, store Store, cfg config.Config) (*Service, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("failed to create GenAI client: API key is required")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	embedder := NewEmbedder(client)
	return &Service{store: store, embedder: embedder}, nil
}

// AddSession implements memory.Service interface.
// It extracts relevant information from the session and stores it as experiences.
// According to ADK docs, this should ingest session contents into long-term knowledge.
func (s *Service) AddSession(ctx context.Context, sess session.Session) error {
	events := sess.Events()

	// Extract user questions and agent responses from the session
	var userQuery string
	var agentResponse string
	hasExplicitSave := false

	for event := range events.All() {
		// Extract user input from events
		if event.Author == "user" && event.Content != nil {
			textParts := extractTextFromContent([]*genai.Content{event.Content})
			if len(textParts) > 0 {
				userQuery = strings.Join(textParts, " ")
			}
		}

		// Extract agent response
		if event.Author != "user" && event.Content != nil {
			textParts := extractTextFromContent([]*genai.Content{event.Content})
			if len(textParts) > 0 {
				agentResponse = strings.Join(textParts, " ")
			}
		}

		// Check if this event contains function calls that might be save_experience
		// We check the content parts for function call indicators
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.FunctionCall != nil && part.FunctionCall.Name == "save_experience" {
					hasExplicitSave = true
					break
				}
			}
		}
	}

	// If experience was explicitly saved via tool, skip to avoid duplicates
	if hasExplicitSave {
		return nil
	}

	// Only save if we have both a query and a meaningful response
	if userQuery != "" && agentResponse != "" && len(agentResponse) > 20 {
		// Generate embedding for the user query
		queryVector, err := s.embedder.Embed(ctx, userQuery)
		if err != nil {
			return fmt.Errorf("failed to generate embedding for session: %w", err)
		}

		// Save as experience
		// Use user query as pattern, agent response as solution
		err = s.store.SaveExperience(ctx, userQuery, "", agentResponse, queryVector)
		if err != nil {
			return fmt.Errorf("failed to save session to memory: %w", err)
		}
	}

	return nil
}

// Search implements memory.Service interface.
// It performs a vector similarity search based on the query and returns memory entries.
func (s *Service) Search(ctx context.Context, req *adkmemory.SearchRequest) (*adkmemory.SearchResponse, error) {
	// Generate embedding for the query
	queryVector, err := s.embedder.Embed(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search for similar issues (limit to 10 most relevant)
	experiences, err := s.store.SearchSimilarIssues(ctx, queryVector, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to search similar issues: %w", err)
	}

	// Convert experiences to memory entries
	memories := make([]adkmemory.Entry, 0, len(experiences))
	for _, exp := range experiences {
		// Build content from experience
		var parts []string
		if exp.ErrorPattern != "" {
			parts = append(parts, "问题: "+exp.ErrorPattern)
		}
		if exp.RootCause != "" {
			parts = append(parts, "原因: "+exp.RootCause)
		}
		if exp.Solution != "" {
			parts = append(parts, "解决方案: "+exp.Solution)
		}

		content := strings.Join(parts, "\n")
		if content == "" {
			continue
		}

		// genai.Text returns []*Content, we need the first one
		contentParts := genai.Text(content)
		if len(contentParts) == 0 {
			continue
		}

		memories = append(memories, adkmemory.Entry{
			Content:   contentParts[0],
			Author:    "system",
			Timestamp: exp.OccurredAt,
		})
	}

	return &adkmemory.SearchResponse{Memories: memories}, nil
}

// extractTextFromContent extracts all text parts from a slice of genai.Content.
// It iterates through all content parts and collects non-empty text segments.
// This is used to extract user queries and agent responses from session events.
func extractTextFromContent(content []*genai.Content) []string {
	var texts []string
	for _, c := range content {
		for _, part := range c.Parts {
			if text := part.Text; text != "" {
				texts = append(texts, text)
			}
		}
	}
	return texts
}
