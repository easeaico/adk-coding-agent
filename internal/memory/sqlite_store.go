package memory

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements the Store interface using SQLite.
// It provides persistent storage for both semantic memory (project rules) and
// episodic memory (past experiences with vector embeddings).
// Vector similarity search is performed in application memory using cosine similarity.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLiteStore connected to the given database path.
// The path should be a file path (e.g., "./data.db") or ":memory:" for in-memory database.
// It opens the database connection and verifies connectivity with a ping.
// Returns an error if the connection cannot be established.
func NewSQLiteStore(ctx context.Context, dbPath string) (*SQLiteStore, error) {
	// Enable WAL mode and foreign keys for better performance and data integrity
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// InitSchema creates the necessary tables if they don't exist.
// This should be called after creating a new SQLiteStore to ensure
// the database schema is properly set up.
func (s *SQLiteStore) InitSchema(ctx context.Context) error {
	schema := `
		-- Semantic Memory: Project Rules
		CREATE TABLE IF NOT EXISTS project_rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			category TEXT NOT NULL,
			rule_content TEXT NOT NULL,
			priority INTEGER DEFAULT 1,
			is_active INTEGER DEFAULT 1,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP
		);

		-- Index for category-based queries
		CREATE INDEX IF NOT EXISTS idx_rules_category ON project_rules(category);

		-- Episodic Memory: Issue History
		CREATE TABLE IF NOT EXISTS issue_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_signature TEXT,
			error_pattern TEXT,
			root_cause TEXT,
			solution_summary TEXT,
			embedding BLOB,
			occurred_at TEXT DEFAULT CURRENT_TIMESTAMP
		);
	`

	_, err := s.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}

// GetProjectRules retrieves all active project rules from the database.
func (s *SQLiteStore) GetProjectRules(ctx context.Context) ([]string, error) {
	query := `
		SELECT rule_content 
		FROM project_rules 
		WHERE is_active = 1 
		ORDER BY priority DESC, category, id
	`

	rows, err := s.db.QueryContext(ctx, query)
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

// experienceWithScore is an internal type for sorting experiences by similarity score.
type experienceWithScore struct {
	Experience
	score float32
}

// SearchSimilarIssues finds past experiences similar to the query vector using cosine similarity.
// Unlike PostgreSQL with pgvector, this implementation loads all embeddings into memory
// and computes similarity scores in the application layer.
// This approach is suitable for smaller datasets (< 10K records).
// Results are ordered by similarity (most similar first) and limited to the specified count.
func (s *SQLiteStore) SearchSimilarIssues(ctx context.Context, queryVector []float32, limit int) ([]Experience, error) {
	query := `
		SELECT id, task_signature, error_pattern, root_cause, solution_summary, embedding, occurred_at
		FROM issue_history
		WHERE embedding IS NOT NULL
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query issues: %w", err)
	}
	defer rows.Close()

	var results []experienceWithScore
	for rows.Next() {
		var exp Experience
		var embeddingBlob []byte
		var occurredAtStr string
		err := rows.Scan(
			&exp.ID,
			&exp.TaskSignature,
			&exp.ErrorPattern,
			&exp.RootCause,
			&exp.Solution,
			&embeddingBlob,
			&occurredAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan experience: %w", err)
		}

		// Parse SQLite timestamp string to time.Time
		exp.OccurredAt, _ = parseTimestamp(occurredAtStr)

		// Decode the embedding and calculate similarity
		storedVector := decodeVector(embeddingBlob)
		if len(storedVector) > 0 && len(storedVector) == len(queryVector) {
			similarity := cosineSimilarity(queryVector, storedVector)
			exp.SimilarityScore = similarity
			results = append(results, experienceWithScore{
				Experience: exp,
				score:      similarity,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating issues: %w", err)
	}

	// Sort by similarity score (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// Return top-k results
	topK := min(limit, len(results))
	experiences := make([]Experience, topK)
	for i := range topK {
		experiences[i] = results[i].Experience
	}

	return experiences, nil
}

// SaveExperience stores a new experience in the issue_history table.
// It saves the error pattern, root cause, solution, and associated embedding vector.
// The task signature is automatically generated from the first 50 runes (characters) of the pattern,
// using []rune to properly handle multi-byte characters (e.g., Chinese, emoji).
// Returns an error if the database insert fails.
func (s *SQLiteStore) SaveExperience(ctx context.Context, pattern, cause, solution string, vector []float32) error {
	// Generate a simple task signature from the first 50 runes of the pattern
	// Use []rune to properly handle multi-byte characters (e.g., Chinese, emoji)
	signature := pattern
	runes := []rune(signature)
	if len(runes) > 50 {
		signature = string(runes[:50])
	}

	// Encode vector to binary
	embeddingBlob := encodeVector(vector)

	query := `
		INSERT INTO issue_history (task_signature, error_pattern, root_cause, solution_summary, embedding)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query, signature, pattern, cause, solution, embeddingBlob)
	if err != nil {
		return fmt.Errorf("failed to save experience: %w", err)
	}

	return nil
}

// Close releases the database connection.
func (s *SQLiteStore) Close() {
	s.db.Close()
}

// encodeVector converts a float32 slice to a byte slice for storage.
// Each float32 is encoded as 4 bytes in little-endian format.
func encodeVector(v []float32) []byte {
	if v == nil {
		return nil
	}
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// decodeVector converts a byte slice back to a float32 slice.
// Each 4 bytes are decoded as one float32 in little-endian format.
func decodeVector(b []byte) []float32 {
	if b == nil || len(b) == 0 || len(b)%4 != 0 {
		return nil
	}
	v := make([]float32, len(b)/4)
	for i := range v {
		bits := binary.LittleEndian.Uint32(b[i*4:])
		v[i] = math.Float32frombits(bits)
	}
	return v
}

// cosineSimilarity calculates the cosine similarity between two vectors.
// The result is in range [-1, 1], where 1 means identical direction,
// 0 means orthogonal, and -1 means opposite direction.
// For normalized embedding vectors, this is equivalent to dot product.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// parseTimestamp parses a SQLite timestamp string to time.Time.
// SQLite stores timestamps as TEXT in ISO8601/RFC3339 format.
func parseTimestamp(s string) (time.Time, error) {
	// Try various formats that SQLite might use
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.000",
		"2006-01-02T15:04:05.000",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", s)
}
