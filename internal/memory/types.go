// Package memory provides memory storage interfaces and implementations
// for the tiered memory architecture of the Legacy Code Hunter agent.
package memory

import "time"

// Experience represents an episodic memory entry - a past issue and its resolution.
type Experience struct {
	ID              int
	TaskSignature   string
	ErrorPattern    string
	RootCause       string
	Solution        string
	SimilarityScore float32
	OccurredAt      time.Time
}

// ProjectRule represents a semantic memory entry - a project rule or constraint.
type ProjectRule struct {
	ID          int
	Category    string
	RuleContent string
	Priority    int
	IsActive    bool
	CreatedAt   time.Time
}
