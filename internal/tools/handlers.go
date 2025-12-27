package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/easeaico/adk-memory-agent/internal/llm"
	"github.com/easeaico/adk-memory-agent/internal/memory"
)

// Handler provides implementations for all agent tools.
type Handler struct {
	store    memory.Store
	embedder llm.Embedder
	workDir  string
}

// NewHandler creates a new tool handler with the given dependencies.
func NewHandler(store memory.Store, embedder llm.Embedder, workDir string) *Handler {
	return &Handler{
		store:    store,
		embedder: embedder,
		workDir:  workDir,
	}
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// HandleToolCall dispatches and executes a tool call based on its name.
func (h *Handler) HandleToolCall(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	var result ToolResult

	switch name {
	case "search_past_issues":
		result = h.handleSearchPastIssues(ctx, args)
	case "read_file_content":
		result = h.handleReadFile(args)
	case "list_directory":
		result = h.handleListDirectory(args)
	case "save_experience":
		result = h.handleSaveExperience(ctx, args)
	default:
		result = ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", name),
		}
	}

	jsonResult, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonResult), nil
}

// handleSearchPastIssues searches for similar past issues using vector similarity.
func (h *Handler) handleSearchPastIssues(ctx context.Context, args map[string]interface{}) ToolResult {
	description, ok := args["error_description"].(string)
	if !ok || description == "" {
		return ToolResult{Success: false, Error: "error_description is required"}
	}

	// Generate embedding for the query
	embedding, err := h.embedder.Embed(ctx, description)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("failed to generate embedding: %v", err)}
	}

	// Search for similar issues
	experiences, err := h.store.SearchSimilarIssues(ctx, embedding, 3)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("failed to search issues: %v", err)}
	}

	if len(experiences) == 0 {
		return ToolResult{
			Success: true,
			Data:    "没有找到相关的历史问题。",
		}
	}

	// Format results
	var results []map[string]interface{}
	for _, exp := range experiences {
		results = append(results, map[string]interface{}{
			"id":         exp.ID,
			"pattern":    exp.ErrorPattern,
			"cause":      exp.RootCause,
			"solution":   exp.Solution,
			"similarity": fmt.Sprintf("%.2f%%", exp.SimilarityScore*100),
		})
	}

	return ToolResult{Success: true, Data: results}
}

// handleReadFile reads the content of a file.
func (h *Handler) handleReadFile(args map[string]interface{}) ToolResult {
	filePath, ok := args["filepath"].(string)
	if !ok || filePath == "" {
		return ToolResult{Success: false, Error: "filepath is required"}
	}

	// Resolve relative paths against working directory
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(h.workDir, filePath)
	}

	// Security check: ensure path is within working directory
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("invalid path: %v", err)}
	}

	absWorkDir, _ := filepath.Abs(h.workDir)
	if !strings.HasPrefix(absPath, absWorkDir) {
		return ToolResult{Success: false, Error: "access denied: path is outside working directory"}
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("failed to read file: %v", err)}
	}

	// Limit content size
	maxSize := 10000
	contentStr := string(content)
	if len(contentStr) > maxSize {
		contentStr = contentStr[:maxSize] + "\n... (truncated)"
	}

	return ToolResult{Success: true, Data: contentStr}
}

// handleListDirectory lists the contents of a directory.
func (h *Handler) handleListDirectory(args map[string]interface{}) ToolResult {
	dirPath, ok := args["path"].(string)
	if !ok || dirPath == "" {
		dirPath = h.workDir
	}

	// Resolve relative paths
	if !filepath.IsAbs(dirPath) {
		dirPath = filepath.Join(h.workDir, dirPath)
	}

	// Security check
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("invalid path: %v", err)}
	}

	absWorkDir, _ := filepath.Abs(h.workDir)
	if !strings.HasPrefix(absPath, absWorkDir) {
		return ToolResult{Success: false, Error: "access denied: path is outside working directory"}
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("failed to read directory: %v", err)}
	}

	var items []map[string]interface{}
	for _, entry := range entries {
		info, _ := entry.Info()
		item := map[string]interface{}{
			"name":  entry.Name(),
			"isDir": entry.IsDir(),
		}
		if info != nil {
			item["size"] = info.Size()
		}
		items = append(items, item)
	}

	return ToolResult{Success: true, Data: items}
}

// handleSaveExperience saves a new experience to the knowledge base.
func (h *Handler) handleSaveExperience(ctx context.Context, args map[string]interface{}) ToolResult {
	pattern, _ := args["error_pattern"].(string)
	cause, _ := args["root_cause"].(string)
	solution, _ := args["solution"].(string)

	if pattern == "" || cause == "" || solution == "" {
		return ToolResult{Success: false, Error: "error_pattern, root_cause, and solution are all required"}
	}

	// Generate embedding for the error pattern
	embedding, err := h.embedder.Embed(ctx, pattern)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("failed to generate embedding: %v", err)}
	}

	// Save to database
	if err := h.store.SaveExperience(ctx, pattern, cause, solution, embedding); err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("failed to save experience: %v", err)}
	}

	return ToolResult{Success: true, Data: "经验已成功保存到知识库。"}
}
