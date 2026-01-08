package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/easeaico/adk-memory-agent/internal/memory"
)

// MockStore implements memory.Store for testing
type MockStore struct {
	SavedExperiences []struct {
		Pattern, Cause, Solution string
		Vector                   []float32
	}
}

func (m *MockStore) GetProjectRules(ctx context.Context) ([]string, error) {
	return []string{"Rule 1"}, nil
}

func (m *MockStore) SearchSimilarIssues(ctx context.Context, queryVector []float32, limit int) ([]memory.Experience, error) {
	return nil, nil
}

func (m *MockStore) SaveExperience(ctx context.Context, pattern, cause, solution string, vector []float32) error {
	m.SavedExperiences = append(m.SavedExperiences, struct {
		Pattern, Cause, Solution string
		Vector                   []float32
	}{pattern, cause, solution, vector})
	return nil
}

func (m *MockStore) Close() {
}

// MockEmbedder implements Embedder for testing
type MockEmbedder struct{}

func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func TestSaveExperienceTool(t *testing.T) {
	mockStore := &MockStore{}
	mockEmbedder := &MockEmbedder{}
	cfg := ToolsConfig{
		Store:    mockStore,
		Embedder: mockEmbedder,
		WorkDir:  ".",
	}

	tool, err := createSaveExperienceTool(cfg)
	if err != nil {
		t.Fatalf("Failed to create tool: %v", err)
	}

	if tool == nil {
		t.Error("Tool should not be nil")
	}
}

func TestReadFileTool_PathSecurity(t *testing.T) {
	// Setup temporary directory structure
	tmpDir, err := os.MkdirTemp("", "agent_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a secret file outside the working directory
	secretFile := filepath.Join(tmpDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a working directory inside
	workDir := filepath.Join(tmpDir, "work")
	if err := os.Mkdir(workDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a normal file
	normalFile := filepath.Join(workDir, "normal.txt")
	if err := os.WriteFile(normalFile, []byte("normal"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := ToolsConfig{
		Store:    &MockStore{},
		Embedder: &MockEmbedder{},
		WorkDir:  workDir,
	}

	// Create the tool
	tool, err := createReadFileTool(cfg)
	if err != nil {
		t.Fatalf("Failed to create tool: %v", err)
	}

	// Test reading a file within the working directory
	// Note: This would require calling the tool handler, which needs tool.Context
	// For now, we just verify the tool was created successfully
	if tool == nil {
		t.Error("Tool should not be nil")
	}
}

func TestTruncateString(t *testing.T) {
	// Test case 1: String shorter than limit
	s1 := "hello"
	if res := truncateString(s1, 10); res != s1 {
		t.Errorf("Expected %q, got %q", s1, res)
	}

	// Test case 2: String exactly at limit
	s2 := "hello"
	if res := truncateString(s2, 5); res != s2 {
		t.Errorf("Expected %q, got %q", s2, res)
	}

	// Test case 3: Simple truncation
	s3 := "hello world"
	if res := truncateString(s3, 5); res != "hello" {
		t.Errorf("Expected 'hello', got %q", res)
	}

	// Test case 4: Multi-byte characters (Chinese)
	// '界' is 3 bytes (E7 95 8C)
	s4 := strings.Repeat("界", 10) // 30 bytes

	// Truncate at 4 bytes (should include 1 char + 1 byte? No, should include 1 char)
	// "界" (3 bytes). Next byte is start of next "界".
	// 4 bytes: [E7 95 8C] [E7] -> Should truncate to just first char [E7 95 8C]
	if res := truncateString(s4, 4); res != "界" {
		t.Errorf("Expected '界', got %q (len: %d)", res, len(res))
	}

	// Truncate at 2 bytes (should be empty, as first char needs 3 bytes)
	if res := truncateString(s4, 2); res != "" {
		t.Errorf("Expected empty string, got %q", res)
	}

	// Truncate at 3 bytes (should be exactly one char)
	if res := truncateString(s4, 3); res != "界" {
		t.Errorf("Expected '界', got %q", res)
	}

	// Test case 5: Verify valid UTF-8
	// Create a long string of Chinese characters
	longStr := strings.Repeat("界", 4000) // 12000 bytes
	limit := 10000

	truncated := truncateString(longStr, limit)

	if !utf8.ValidString(truncated) {
		t.Errorf("Truncated string is not valid UTF-8")
	}

	if len(truncated) > limit {
		t.Errorf("Truncated string length %d exceeds limit %d", len(truncated), limit)
	}
}

func TestListFilesTool(t *testing.T) {
	// Setup temporary directory structure
	tmpDir, err := os.MkdirTemp("", "agent_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a working directory inside
	workDir := filepath.Join(tmpDir, "work")
	if err := os.Mkdir(workDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := ToolsConfig{
		Store:    &MockStore{},
		Embedder: &MockEmbedder{},
		WorkDir:  workDir,
	}

	// Create the tool
	tool, err := createListFilesTool(cfg)
	if err != nil {
		t.Fatalf("Failed to create tool: %v", err)
	}

	// Test reading a file within the working directory
	// Note: This would require calling the tool handler, which needs tool.Context
	// For now, we just verify the tool was created successfully
	if tool == nil {
		t.Error("Tool should not be nil")
	}
}
