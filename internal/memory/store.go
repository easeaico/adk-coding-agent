package memory

import "context"

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
	Close() error
}
