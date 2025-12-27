// Package tools defines ADK tool declarations for the Legacy Code Hunter agent.
// These tools represent the agent's procedural memory - its ability to interact
// with the external world.
package tools

import "github.com/google/generative-ai-go/genai"

// SearchBugHistory is a tool that allows the agent to search for past issues
// and their resolutions. This enables the "recall" capability.
var SearchBugHistory = &genai.Tool{
	FunctionDeclarations: []*genai.FunctionDeclaration{{
		Name:        "search_past_issues",
		Description: "当遇到不确定的错误或复杂 Bug 时，搜索过去是否处理过类似问题。返回相关的历史问题和解决方案。",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"error_description": {
					Type:        genai.TypeString,
					Description: "对错误现象或报错日志的简要描述",
				},
			},
			Required: []string{"error_description"},
		},
	}},
}

// ReadFile is a tool that allows the agent to read file contents.
// This is a perception tool for understanding the codebase.
var ReadFile = &genai.Tool{
	FunctionDeclarations: []*genai.FunctionDeclaration{{
		Name:        "read_file_content",
		Description: "读取指定路径的代码文件内容。用于理解和分析代码。",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"filepath": {
					Type:        genai.TypeString,
					Description: "要读取的文件的完整路径",
				},
			},
			Required: []string{"filepath"},
		},
	}},
}

// ListDirectory is a tool that allows the agent to explore the project structure.
var ListDirectory = &genai.Tool{
	FunctionDeclarations: []*genai.FunctionDeclaration{{
		Name:        "list_directory",
		Description: "列出指定目录下的文件和子目录。用于探索项目结构。",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"path": {
					Type:        genai.TypeString,
					Description: "要列出内容的目录路径",
				},
			},
			Required: []string{"path"},
		},
	}},
}

// SaveExperience is a tool that allows the agent to save new learnings.
var SaveExperience = &genai.Tool{
	FunctionDeclarations: []*genai.FunctionDeclaration{{
		Name:        "save_experience",
		Description: "将成功解决的问题经验保存到知识库中，供将来参考。",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"error_pattern": {
					Type:        genai.TypeString,
					Description: "问题的错误模式或现象描述",
				},
				"root_cause": {
					Type:        genai.TypeString,
					Description: "问题的根本原因分析",
				},
				"solution": {
					Type:        genai.TypeString,
					Description: "解决方案的摘要",
				},
			},
			Required: []string{"error_pattern", "root_cause", "solution"},
		},
	}},
}

// AllTools returns all available tools for the agent.
func AllTools() []*genai.Tool {
	return []*genai.Tool{
		SearchBugHistory,
		ReadFile,
		ListDirectory,
		SaveExperience,
	}
}
