// Package memory provides memory storage interfaces and implementations
// for the tiered memory architecture of the Legacy Code Hunter agent.
package memory

import "time"

// Experience represents an episodic memory entry - a past issue and its resolution.
// It stores information about a problem that was encountered and how it was solved,
// along with a vector embedding for similarity search.
type Experience struct {
	ID              int       // Unique identifier in the database
	TaskSignature   string    // Short signature (first 50 chars) for quick identification
	ErrorPattern    string    // Description of the error or problem pattern
	RootCause       string    // Root cause analysis of the issue
	Solution        string    // Solution or fix that resolved the issue
	SimilarityScore float32   // Similarity score when returned from search (0-1, higher is more similar)
	OccurredAt      time.Time // Timestamp when the issue was encountered and resolved
}

// ProjectRule represents a semantic memory entry - a project rule or constraint.
// These rules are injected into the system prompt to guide agent behavior
// and enforce project-specific coding standards and practices.
type ProjectRule struct {
	ID          int       // Unique identifier in the database
	Category    string    // Category or type of rule (e.g., "naming", "error_handling")
	RuleContent string    // The actual rule text to be included in the system prompt
	Priority    int       // Priority level (higher values take precedence)
	IsActive    bool      // Whether this rule is currently active
	CreatedAt   time.Time // Timestamp when the rule was created
}
