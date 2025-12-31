package memory

import (
	"context"
	"fmt"
	"strings"

	adkmemory "google.golang.org/adk/memory"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

type Service struct {
	store    Store
	embedder Embedder // Optional embedder for memory.Service.Search
}

// NewService creates a new memory service with the given store and embedder.
func NewService(store Store, embedder Embedder) *Service {
	return &Service{store: store, embedder: embedder}
}

// AddSession implements memory.Service interface.
// It extracts relevant information from the session and stores it as experiences.
// According to ADK docs, this should ingest session contents into long-term knowledge.
func (s *Service) AddSession(ctx context.Context, sess session.Session) error {
	if s.embedder == nil {
		// Without embedder, we can't create embeddings, so skip ingestion
		return nil
	}

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
		if event.Author != "user" && event.LLMResponse.Content != nil {
			textParts := extractTextFromContent([]*genai.Content{event.LLMResponse.Content})
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
	if s.embedder == nil {
		// Without embedder, return empty results
		return &adkmemory.SearchResponse{Memories: []adkmemory.Entry{}}, nil
	}

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

// extractTextFromContent extracts text from genai.Content parts
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
