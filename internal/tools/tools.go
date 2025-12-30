// Package tools defines ADK tool declarations for the Legacy Code Hunter agent.
// These tools represent the agent's procedural memory - its ability to interact
// with the external world.
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/easeaico/adk-memory-agent/internal/memory"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// Embedder is an interface for generating text embeddings.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// ToolsConfig holds dependencies for creating tools.
type ToolsConfig struct {
	Store    memory.Store
	Embedder Embedder
	WorkDir  string
}

// --- Tool Input/Output Structs ---

// SearchPastIssuesArgs is the input for search_past_issues tool.
type SearchPastIssuesArgs struct {
	ErrorDescription string `json:"error_description" jsonschema:"description=对错误现象或报错日志的简要描述"`
}

// SearchPastIssuesResult is the output for search_past_issues tool.
type SearchPastIssuesResult struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ReadFileArgs is the input for read_file_content tool.
type ReadFileArgs struct {
	Filepath string `json:"filepath" jsonschema:"description=要读取的文件的完整路径"`
}

// ReadFileResult is the output for read_file_content tool.
type ReadFileResult struct {
	Success bool   `json:"success"`
	Data    string `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ListDirectoryArgs is the input for list_directory tool.
type ListDirectoryArgs struct {
	Path string `json:"path" jsonschema:"description=要列出内容的目录路径"`
}

// ListDirectoryResult is the output for list_directory tool.
type ListDirectoryResult struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SaveExperienceArgs is the input for save_experience tool.
type SaveExperienceArgs struct {
	ErrorPattern string `json:"error_pattern" jsonschema:"description=问题的错误模式或现象描述"`
	RootCause    string `json:"root_cause" jsonschema:"description=问题的根本原因分析"`
	Solution     string `json:"solution" jsonschema:"description=解决方案的摘要"`
}

// SaveExperienceResult is the output for save_experience tool.
type SaveExperienceResult struct {
	Success bool   `json:"success"`
	Data    string `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// --- Tool Handlers ---

func createSearchPastIssuesTool(cfg ToolsConfig) (tool.Tool, error) {
	handler := func(ctx tool.Context, args SearchPastIssuesArgs) (SearchPastIssuesResult, error) {
		if args.ErrorDescription == "" {
			return SearchPastIssuesResult{Success: false, Error: "error_description is required"}, nil
		}

		// Generate embedding for the query
		embedding, err := cfg.Embedder.Embed(ctx, args.ErrorDescription)
		if err != nil {
			return SearchPastIssuesResult{Success: false, Error: fmt.Sprintf("failed to generate embedding: %v", err)}, nil
		}

		// Search for similar issues
		experiences, err := cfg.Store.SearchSimilarIssues(ctx, embedding, 3)
		if err != nil {
			return SearchPastIssuesResult{Success: false, Error: fmt.Sprintf("failed to search issues: %v", err)}, nil
		}

		if len(experiences) == 0 {
			return SearchPastIssuesResult{Success: true, Data: "没有找到相关的历史问题。"}, nil
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

		return SearchPastIssuesResult{Success: true, Data: results}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "search_past_issues",
		Description: "当遇到不确定的错误或复杂 Bug 时，搜索过去是否处理过类似问题。返回相关的历史问题和解决方案。",
	}, handler)
}

func createReadFileTool(cfg ToolsConfig) (tool.Tool, error) {
	handler := func(ctx tool.Context, args ReadFileArgs) (ReadFileResult, error) {
		if args.Filepath == "" {
			return ReadFileResult{Success: false, Error: "filepath is required"}, nil
		}

		filePath := args.Filepath

		// Resolve relative paths against working directory
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(cfg.WorkDir, filePath)
		}

		// Security check: ensure path is within working directory
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return ReadFileResult{Success: false, Error: fmt.Sprintf("invalid path: %v", err)}, nil
		}

		absWorkDir, _ := filepath.Abs(cfg.WorkDir)
		if !strings.HasPrefix(absPath, absWorkDir) {
			return ReadFileResult{Success: false, Error: "access denied: path is outside working directory"}, nil
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			return ReadFileResult{Success: false, Error: fmt.Sprintf("failed to read file: %v", err)}, nil
		}

		// Limit content size
		maxSize := 10000
		contentStr := string(content)
		if len(contentStr) > maxSize {
			contentStr = contentStr[:maxSize] + "\n... (truncated)"
		}

		return ReadFileResult{Success: true, Data: contentStr}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "read_file_content",
		Description: "读取指定路径的代码文件内容。用于理解和分析代码。",
	}, handler)
}

func createListDirectoryTool(cfg ToolsConfig) (tool.Tool, error) {
	handler := func(ctx tool.Context, args ListDirectoryArgs) (ListDirectoryResult, error) {
		dirPath := args.Path
		if dirPath == "" {
			dirPath = cfg.WorkDir
		}

		// Resolve relative paths
		if !filepath.IsAbs(dirPath) {
			dirPath = filepath.Join(cfg.WorkDir, dirPath)
		}

		// Security check
		absPath, err := filepath.Abs(dirPath)
		if err != nil {
			return ListDirectoryResult{Success: false, Error: fmt.Sprintf("invalid path: %v", err)}, nil
		}

		absWorkDir, _ := filepath.Abs(cfg.WorkDir)
		if !strings.HasPrefix(absPath, absWorkDir) {
			return ListDirectoryResult{Success: false, Error: "access denied: path is outside working directory"}, nil
		}

		entries, err := os.ReadDir(absPath)
		if err != nil {
			return ListDirectoryResult{Success: false, Error: fmt.Sprintf("failed to read directory: %v", err)}, nil
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

		return ListDirectoryResult{Success: true, Data: items}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "list_directory",
		Description: "列出指定目录下的文件和子目录。用于探索项目结构。",
	}, handler)
}

func createSaveExperienceTool(cfg ToolsConfig) (tool.Tool, error) {
	handler := func(ctx tool.Context, args SaveExperienceArgs) (SaveExperienceResult, error) {
		if args.ErrorPattern == "" || args.RootCause == "" || args.Solution == "" {
			return SaveExperienceResult{Success: false, Error: "error_pattern, root_cause, and solution are all required"}, nil
		}

		// Generate embedding for the error pattern
		embedding, err := cfg.Embedder.Embed(ctx, args.ErrorPattern)
		if err != nil {
			return SaveExperienceResult{Success: false, Error: fmt.Sprintf("failed to generate embedding: %v", err)}, nil
		}

		// Save to database
		if err := cfg.Store.SaveExperience(ctx, args.ErrorPattern, args.RootCause, args.Solution, embedding); err != nil {
			return SaveExperienceResult{Success: false, Error: fmt.Sprintf("failed to save experience: %v", err)}, nil
		}

		return SaveExperienceResult{Success: true, Data: "经验已成功保存到知识库。"}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "save_experience",
		Description: "将成功解决的问题经验保存到知识库中，供将来参考。",
	}, handler)
}

// BuildTools creates all agent tools with the given configuration.
func BuildTools(cfg ToolsConfig) ([]tool.Tool, error) {
	var tools []tool.Tool

	searchTool, err := createSearchPastIssuesTool(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create search_past_issues tool: %w", err)
	}
	tools = append(tools, searchTool)

	readFileTool, err := createReadFileTool(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create read_file_content tool: %w", err)
	}
	tools = append(tools, readFileTool)

	listDirTool, err := createListDirectoryTool(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create list_directory tool: %w", err)
	}
	tools = append(tools, listDirTool)

	saveExpTool, err := createSaveExperienceTool(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create save_experience tool: %w", err)
	}
	tools = append(tools, saveExpTool)

	return tools, nil
}
