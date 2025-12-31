package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	adkmemory "google.golang.org/adk/memory"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// Embedder is an interface for generating text embeddings.
// This is needed for the memory.Service Search method.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// PostgresStore implements both the Store interface and adk's memory.Service interface using PostgreSQL with pgvector.
type PostgresStore struct {
	pool     *pgxpool.Pool
	embedder Embedder // Optional embedder for memory.Service.Search
}

// NewPostgresStore creates a new PostgresStore connected to the given database URL.
// The URL should be in the format: postgres://user:password@host:port/database
// embedder is optional but required for memory.Service.Search to work properly.
func NewPostgresStore(ctx context.Context, databaseURL string, embedder Embedder) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresStore{pool: pool, embedder: embedder}, nil
}

// GetProjectRules retrieves all active project rules from the database.
func (s *PostgresStore) GetProjectRules(ctx context.Context) ([]string, error) {
	query := `
		SELECT rule_content 
		FROM project_rules 
		WHERE is_active = TRUE 
		ORDER BY priority DESC, category, id
	`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query project rules: %w", err)
	}
	defer rows.Close()

	var rules []string
	for rows.Next() {
		var rule string
		if err := rows.Scan(&rule); err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rules: %w", err)
	}

	return rules, nil
}

// SearchSimilarIssues finds past experiences similar to the query vector using cosine similarity.
func (s *PostgresStore) SearchSimilarIssues(ctx context.Context, queryVector []float32, limit int) ([]Experience, error) {
	// Convert to pgvector type
	vec := pgvector.NewVector(queryVector)

	query := `
		SELECT id, task_signature, error_pattern, root_cause, solution_summary, 
		       1 - (embedding <=> $1) as similarity, occurred_at
		FROM issue_history
		WHERE embedding IS NOT NULL
		ORDER BY embedding <=> $1
		LIMIT $2
	`

	rows, err := s.pool.Query(ctx, query, vec, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search similar issues: %w", err)
	}
	defer rows.Close()

	var experiences []Experience
	for rows.Next() {
		var exp Experience
		err := rows.Scan(
			&exp.ID,
			&exp.TaskSignature,
			&exp.ErrorPattern,
			&exp.RootCause,
			&exp.Solution,
			&exp.SimilarityScore,
			&exp.OccurredAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan experience: %w", err)
		}
		experiences = append(experiences, exp)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating experiences: %w", err)
	}

	return experiences, nil
}

// SaveExperience stores a new experience in the issue_history table.
func (s *PostgresStore) SaveExperience(ctx context.Context, pattern, cause, solution string, vector []float32) error {
	// Generate a simple task signature from the first 50 chars of the pattern
	signature := pattern
	if len(signature) > 50 {
		signature = signature[:50]
	}

	vec := pgvector.NewVector(vector)

	query := `
		INSERT INTO issue_history (task_signature, error_pattern, root_cause, solution_summary, embedding)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := s.pool.Exec(ctx, query, signature, pattern, cause, solution, vec)
	if err != nil {
		return fmt.Errorf("failed to save experience: %w", err)
	}

	return nil
}

// Close releases the connection pool.
func (s *PostgresStore) Close() error {
	s.pool.Close()
	return nil
}

// AddSession implements memory.Service interface.
// It extracts relevant information from the session and stores it as experiences.
// According to ADK docs, this should ingest session contents into long-term knowledge.
func (s *PostgresStore) AddSession(ctx context.Context, sess session.Session) error {
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
		err = s.SaveExperience(ctx, userQuery, "", agentResponse, queryVector)
		if err != nil {
			return fmt.Errorf("failed to save session to memory: %w", err)
		}
	}
	
	return nil
}

// Search implements memory.Service interface.
// It performs a vector similarity search based on the query and returns memory entries.
func (s *PostgresStore) Search(ctx context.Context, req *adkmemory.SearchRequest) (*adkmemory.SearchResponse, error) {
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
	experiences, err := s.SearchSimilarIssues(ctx, queryVector, 10)
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

// Ensure PostgresStore implements both interfaces
var _ Store = (*PostgresStore)(nil)
var _ adkmemory.Service = (*PostgresStore)(nil)
