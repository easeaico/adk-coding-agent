package memory

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

// Store defines the contract for memory operations.
// It abstracts the storage layer for both semantic and episodic memories.
type Store interface {
	// GetProjectRules retrieves all active project rules (semantic memory).
	// These rules are injected into the system prompt to guide agent behavior.
	GetProjectRules(ctx context.Context) ([]string, error)

	// SearchSimilarIssues performs a vector similarity search to find past experiences
	// that are relevant to the current problem (episodic memory with RAG).
	SearchSimilarIssues(ctx context.Context, queryVector []float32, limit int) ([]Experience, error)

	// SaveExperience consolidates a new experience into the database.
	// This is called after successfully resolving an issue to build knowledge.
	SaveExperience(ctx context.Context, pattern, cause, solution string, vector []float32) error

	// Close releases any resources held by the store.
	Close()
}

// PostgresStore implements the Store interface using PostgreSQL with pgvector extension.
// It provides persistent storage for both semantic memory (project rules) and
// episodic memory (past experiences with vector embeddings).
type PostgresStore struct {
	pool *pgxpool.Pool // Connection pool for database operations
}

// NewPostgresStore creates a new PostgresStore connected to the given database URL.
// The URL should be in the format: postgres://user:password@host:port/database
// It creates a connection pool and verifies the connection with a ping.
// Returns an error if the connection cannot be established.
func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresStore{pool: pool}, nil
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
// It uses PostgreSQL's pgvector extension to perform vector similarity search.
// The results are ordered by similarity (most similar first) and limited to the specified count.
// Returns an error if the database query fails.
func (s *PostgresStore) SearchSimilarIssues(ctx context.Context, queryVector []float32, limit int) ([]Experience, error) {
	// Convert float32 slice to pgvector type for database query
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
// It saves the error pattern, root cause, solution, and associated embedding vector.
// The task signature is automatically generated from the first 50 characters of the pattern.
// Returns an error if the database insert fails.
func (s *PostgresStore) SaveExperience(ctx context.Context, pattern, cause, solution string, vector []float32) error {
	// Generate a simple task signature from the first 50 characters of the pattern
	// Use []rune to properly handle multi-byte characters (e.g., Chinese, emoji)
	signature := pattern
	runes := []rune(signature)
	if len(runes) > 50 {
		signature = string(runes[:50])
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
func (s *PostgresStore) Close() {
	s.pool.Close()
}
