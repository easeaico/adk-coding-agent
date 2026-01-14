package memory

import (
	"context"
	"math"
	"os"
	"testing"
)

// TestNewSQLiteStore tests SQLite store creation and initialization.
func TestNewSQLiteStore(t *testing.T) {
	ctx := context.Background()

	// Test with in-memory database
	store, err := NewSQLiteStore(ctx, ":memory:")
	if err != nil {
		t.Fatalf("failed to create SQLite store: %v", err)
	}
	defer store.Close()

	// Initialize schema
	if err := store.InitSchema(ctx); err != nil {
		t.Fatalf("failed to initialize schema: %v", err)
	}
}

// TestSQLiteStore_GetProjectRules tests retrieving project rules from SQLite.
func TestSQLiteStore_GetProjectRules(t *testing.T) {
	ctx := context.Background()

	store, err := NewSQLiteStore(ctx, ":memory:")
	if err != nil {
		t.Fatalf("failed to create SQLite store: %v", err)
	}
	defer store.Close()

	if err := store.InitSchema(ctx); err != nil {
		t.Fatalf("failed to initialize schema: %v", err)
	}

	// Insert test rules
	_, err = store.db.ExecContext(ctx, `
		INSERT INTO project_rules (category, rule_content, priority, is_active) VALUES
			('STYLE', 'Test rule 1', 2, 1),
			('SECURITY', 'Test rule 2', 1, 1),
			('STYLE', 'Inactive rule', 1, 0)
	`)
	if err != nil {
		t.Fatalf("failed to insert test rules: %v", err)
	}

	// Retrieve rules
	rules, err := store.GetProjectRules(ctx)
	if err != nil {
		t.Fatalf("failed to get project rules: %v", err)
	}

	// Should get 2 active rules (inactive one excluded)
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}

	// First rule should be the higher priority one
	if rules[0] != "Test rule 1" {
		t.Errorf("expected first rule to be 'Test rule 1', got '%s'", rules[0])
	}
}

// TestSQLiteStore_SaveAndSearchExperiences tests saving and searching experiences.
func TestSQLiteStore_SaveAndSearchExperiences(t *testing.T) {
	ctx := context.Background()

	store, err := NewSQLiteStore(ctx, ":memory:")
	if err != nil {
		t.Fatalf("failed to create SQLite store: %v", err)
	}
	defer store.Close()

	if err := store.InitSchema(ctx); err != nil {
		t.Fatalf("failed to initialize schema: %v", err)
	}

	// Create test vectors (768 dimensions)
	vector1 := make([]float32, 768)
	vector2 := make([]float32, 768)
	queryVector := make([]float32, 768)

	// vector1 and queryVector are similar (high similarity)
	for i := 0; i < 768; i++ {
		vector1[i] = float32(i) / 768.0
		queryVector[i] = float32(i) / 768.0
	}

	// vector2 is different (low similarity)
	for i := 0; i < 768; i++ {
		vector2[i] = float32(768-i) / 768.0
	}

	// Save experiences
	err = store.SaveExperience(ctx, "Error pattern 1", "Root cause 1", "Solution 1", vector1)
	if err != nil {
		t.Fatalf("failed to save experience 1: %v", err)
	}

	err = store.SaveExperience(ctx, "Error pattern 2", "Root cause 2", "Solution 2", vector2)
	if err != nil {
		t.Fatalf("failed to save experience 2: %v", err)
	}

	// Search similar experiences
	experiences, err := store.SearchSimilarIssues(ctx, queryVector, 10)
	if err != nil {
		t.Fatalf("failed to search similar issues: %v", err)
	}

	if len(experiences) != 2 {
		t.Fatalf("expected 2 experiences, got %d", len(experiences))
	}

	// First result should be more similar (experience 1)
	if experiences[0].ErrorPattern != "Error pattern 1" {
		t.Errorf("expected first experience to be 'Error pattern 1', got '%s'", experiences[0].ErrorPattern)
	}

	// Verify similarity scores are in valid range (with small epsilon for floating point)
	const epsilon = 0.0001
	for i, exp := range experiences {
		if exp.SimilarityScore < -1-epsilon || exp.SimilarityScore > 1+epsilon {
			t.Errorf("experience %d has invalid similarity score: %f", i, exp.SimilarityScore)
		}
	}

	// First experience should have higher similarity than second
	if experiences[0].SimilarityScore <= experiences[1].SimilarityScore {
		t.Errorf("expected first experience to have higher similarity, got %f <= %f",
			experiences[0].SimilarityScore, experiences[1].SimilarityScore)
	}
}

// TestSQLiteStore_SaveExperience_SignatureTruncation tests that signature truncation
// properly handles multi-byte characters.
func TestSQLiteStore_SaveExperience_SignatureTruncation(t *testing.T) {
	ctx := context.Background()

	store, err := NewSQLiteStore(ctx, ":memory:")
	if err != nil {
		t.Fatalf("failed to create SQLite store: %v", err)
	}
	defer store.Close()

	if err := store.InitSchema(ctx); err != nil {
		t.Fatalf("failed to initialize schema: %v", err)
	}

	// Test with long Chinese text (should truncate to 50 runes)
	longPattern := "这是一个非常长的错误消息，超过了五十个字符的限制，应该被正确截断，不会在字符中间断开"
	vector := make([]float32, 768)

	err = store.SaveExperience(ctx, longPattern, "cause", "solution", vector)
	if err != nil {
		t.Fatalf("failed to save experience: %v", err)
	}

	// Verify the signature was truncated
	var signature string
	err = store.db.QueryRowContext(ctx, "SELECT task_signature FROM issue_history WHERE id = 1").Scan(&signature)
	if err != nil {
		t.Fatalf("failed to query signature: %v", err)
	}

	runeCount := len([]rune(signature))
	if runeCount > 50 {
		t.Errorf("signature should be at most 50 runes, got %d", runeCount)
	}
}

// TestVectorEncodeDecode tests the vector encoding and decoding functions.
func TestVectorEncodeDecode(t *testing.T) {
	tests := []struct {
		name   string
		vector []float32
	}{
		{
			name:   "nil vector",
			vector: nil,
		},
		{
			name:   "empty vector",
			vector: []float32{},
		},
		{
			name:   "single element",
			vector: []float32{3.14159},
		},
		{
			name:   "multiple elements",
			vector: []float32{1.0, 2.0, 3.0, -4.5, 0.0},
		},
		{
			name:   "768 dimension vector",
			vector: make768Vector(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := encodeVector(tt.vector)
			decoded := decodeVector(encoded)

			if tt.vector == nil {
				if decoded != nil {
					t.Errorf("expected nil, got %v", decoded)
				}
				return
			}

			if len(tt.vector) == 0 {
				if len(decoded) != 0 {
					t.Errorf("expected empty vector, got length %d", len(decoded))
				}
				return
			}

			if len(decoded) != len(tt.vector) {
				t.Fatalf("length mismatch: expected %d, got %d", len(tt.vector), len(decoded))
			}

			for i := range tt.vector {
				if decoded[i] != tt.vector[i] {
					t.Errorf("element %d mismatch: expected %f, got %f", i, tt.vector[i], decoded[i])
				}
			}
		})
	}
}

// TestCosineSimilarity tests the cosine similarity function.
func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float32
		epsilon  float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1.0, 2.0, 3.0},
			b:        []float32{1.0, 2.0, 3.0},
			expected: 1.0,
			epsilon:  0.0001,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1.0, 2.0, 3.0},
			b:        []float32{-1.0, -2.0, -3.0},
			expected: -1.0,
			epsilon:  0.0001,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1.0, 0.0},
			b:        []float32{0.0, 1.0},
			expected: 0.0,
			epsilon:  0.0001,
		},
		{
			name:     "different length vectors",
			a:        []float32{1.0, 2.0},
			b:        []float32{1.0, 2.0, 3.0},
			expected: 0.0,
			epsilon:  0.0001,
		},
		{
			name:     "empty vectors",
			a:        []float32{},
			b:        []float32{},
			expected: 0.0,
			epsilon:  0.0001,
		},
		{
			name:     "zero vector",
			a:        []float32{0.0, 0.0, 0.0},
			b:        []float32{1.0, 2.0, 3.0},
			expected: 0.0,
			epsilon:  0.0001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			if math.Abs(float64(result-tt.expected)) > float64(tt.epsilon) {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

// TestSQLiteStore_FileDatabase tests using a file-based SQLite database.
func TestSQLiteStore_FileDatabase(t *testing.T) {
	ctx := context.Background()

	// Create a temporary file for the database
	tmpFile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Create store with file database
	store, err := NewSQLiteStore(ctx, tmpPath)
	if err != nil {
		t.Fatalf("failed to create SQLite store: %v", err)
	}

	if err := store.InitSchema(ctx); err != nil {
		t.Fatalf("failed to initialize schema: %v", err)
	}

	// Insert a rule
	_, err = store.db.ExecContext(ctx, `INSERT INTO project_rules (category, rule_content) VALUES ('TEST', 'Test rule')`)
	if err != nil {
		t.Fatalf("failed to insert rule: %v", err)
	}

	// Close and reopen
	store.Close()

	store2, err := NewSQLiteStore(ctx, tmpPath)
	if err != nil {
		t.Fatalf("failed to reopen SQLite store: %v", err)
	}
	defer store2.Close()

	// Verify data persisted
	rules, err := store2.GetProjectRules(ctx)
	if err != nil {
		t.Fatalf("failed to get project rules: %v", err)
	}

	if len(rules) != 1 || rules[0] != "Test rule" {
		t.Errorf("expected 1 rule 'Test rule', got %v", rules)
	}
}

// make768Vector creates a test 768-dimensional vector.
func make768Vector() []float32 {
	v := make([]float32, 768)
	for i := range v {
		v[i] = float32(i) / 768.0
	}
	return v
}
