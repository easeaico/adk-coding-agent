# AGENTS.md

This file contains guidelines and commands for agentic coding agents working in this repository.

## Project Overview

**Legacy Code Hunter (遗留代码猎手)** - A tiered memory intelligent agent based on ADK-go and PostgreSQL.

The agent implements three types of memory:
- **Semantic Memory**: Project rules and guidelines stored in the database
- **Episodic Memory**: Past experiences and solutions with vector similarity search (RAG)
- **Procedural Memory**: Tools for file operations, searching, and experience management

## Build, Lint, and Test Commands

### Build Commands
```bash
# Build the main agent
go build -o bin/agent ./cmd/agent

# Build for different platforms
GOOS=linux GOARCH=amd64 go build -o bin/agent-linux ./cmd/agent
GOOS=darwin GOARCH=amd64 go build -o bin/agent-darwin ./cmd/agent
GOOS=windows GOARCH=amd64 go build -o bin/agent.exe ./cmd/agent
```

### Run Commands
```bash
# Run the agent directly
go run ./cmd/agent

# Run with specific working directory
WORK_DIR="/path/to/project" go run ./cmd/agent

# The agent uses ADK launcher, which supports various command-line arguments
# See: google.golang.org/adk/cmd/launcher
```

### Test Commands
```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test ./internal/memory/...
go test ./internal/tools/...

# Run a single test function
go test -run TestFunctionName ./path/to/package

# Run tests with coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...

# Run benchmarks
go test -bench=. ./...
```

### Lint and Format Commands
```bash
# Format code
go fmt ./...

# Run go vet
go vet ./...

# Run staticcheck (if installed)
staticcheck ./...

# Run golangci-lint (if installed)
golangci-lint run
```

### Database Commands
```bash
# Enable pgvector extension (required before running migrations)
psql -d your_database -c "CREATE EXTENSION IF NOT EXISTS vector;"

# Run database migrations
psql -d your_database -f migrations/001_init.sql

# The migration creates:
# - project_rules table (semantic memory)
# - issue_history table (episodic memory with vector embeddings)
```

## Code Style Guidelines

### Import Organization
- Use standard Go import grouping: stdlib, third-party, internal packages
- Sort imports within each group alphabetically
- Use blank line between groups
- Example:
```go
import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/adk/agent"
	"google.golang.org/genai"

	"github.com/easeaico/adk-memory-agent/internal/memory"
)
```

### Naming Conventions
- **Package names**: lowercase, short, descriptive (e.g., `memory`, `tools`, `agent`, `config`)
- **Constants**: UPPER_SNAKE_CASE for exported constants
- **Variables**: camelCase, with descriptive names
- **Functions**: PascalCase for exported, camelCase for unexported
- **Interfaces**: Should end with `-er` suffix when possible (e.g., `Store`, `Embedder`)
- **Structs**: PascalCase, with clear field names

### Error Handling
- Always handle errors explicitly
- Use `fmt.Errorf` with `%w` verb for error wrapping
- Return errors as the last return value
- Example:
```go
func doSomething() (string, error) {
	result, err := someOperation()
	if err != nil {
		return "", fmt.Errorf("failed to do something: %w", err)
	}
	return result, nil
}
```

### Function and Method Documentation
- Exported functions must have godoc comments
- Comments should start with function name
- Include parameter descriptions and return values
- Example:
```go
// NewService creates a new memory service with the given store and configuration.
// It initializes a GenAI client and embedder for vector operations.
func NewService(ctx context.Context, store Store, cfg config.Config) (*Service, error) {
	// ...
}
```

### Struct Field Tags
- Use JSON tags for tool input/output structs
- Use jsonschema tags for tool parameters (handled by ADK functiontool)
- Example:
```go
type SearchPastIssuesArgs struct {
	ErrorDescription string `json:"error_description"`
}
```

### Context Usage
- Accept `context.Context` as the first parameter in functions that need it
- Pass context through call chains properly
- Use context for cancellation and timeouts
- Example:
```go
func (s *Service) Search(ctx context.Context, req *adkmemory.SearchRequest) (*adkmemory.SearchResponse, error) {
	// Use ctx for database operations, HTTP calls, etc.
}
```

### Interface Design
- Keep interfaces small and focused
- Accept interfaces as parameters, return concrete types
- Use composition for interface building
- Example:
```go
type Store interface {
	GetProjectRules(ctx context.Context) ([]string, error)
	SearchSimilarIssues(ctx context.Context, queryVector []float32, limit int) ([]Experience, error)
	SaveExperience(ctx context.Context, pattern, cause, solution string, vector []float32) error
	Close()
}
```

### Database Operations
- Use connection pooling (pgxpool)
- Always close resources (defer rows.Close())
- Use parameterized queries ($1, $2, etc.) for security
- Handle nil values properly
- Use []rune for multi-byte character truncation (not byte slicing)
- Example:
```go
func (s *PostgresStore) SaveExperience(ctx context.Context, pattern, cause, solution string, vector []float32) error {
	// Use []rune for proper multi-byte character handling
	signature := pattern
	runes := []rune(signature)
	if len(runes) > 50 {
		signature = string(runes[:50])
	}
	
	vec := pgvector.NewVector(vector)
	query := `INSERT INTO issue_history (task_signature, error_pattern, root_cause, solution_summary, embedding)
	          VALUES ($1, $2, $3, $4, $5)`
	_, err := s.pool.Exec(ctx, query, signature, pattern, cause, solution, vec)
	return err
}
```

### Testing Guidelines
- Use table-driven tests for multiple test cases
- Test both success and error paths
- Mock external dependencies using interfaces
- Example:
```go
func TestService_Search(t *testing.T) {
	tests := []struct {
		name         string
		request      *adkmemory.SearchRequest
		embedder     *mockEmbedder
		store        *mockStore
		wantError    bool
		checkResult  func(*testing.T, *adkmemory.SearchResponse)
	}{
		// test cases
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// test implementation
		})
	}
}
```

### Configuration Management
- Use environment variables for configuration
- Provide sensible defaults
- Validate required configuration at startup
- Example:
```go
type Config struct {
	DatabaseURL string // PostgreSQL connection string (required)
	APIKey      string // Google GenAI API key (required)
	WorkDir     string // Working directory for file operations (optional)
}

func Load() Config {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		APIKey:      os.Getenv("GOOGLE_API_KEY"),
		WorkDir:     os.Getenv("WORK_DIR"),
	}
	
	if cfg.WorkDir == "" {
		cfg.WorkDir, _ = os.Getwd()
	}
	
	// Validate required fields
	if cfg.APIKey == "" {
		log.Fatal("GOOGLE_API_KEY environment variable is required")
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}
	
	return cfg
}
```

### Tool Development (ADK Specific)
- Follow ADK tool patterns with input/output structs
- Use functiontool.New for creating tools
- Implement proper error handling in tool handlers
- Return structured results with success/error fields
- Include security checks (e.g., path traversal prevention)
- Example:
```go
type SearchPastIssuesArgs struct {
	ErrorDescription string `json:"error_description"`
}

type SearchPastIssuesResult struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func createSearchPastIssuesTool(cfg ToolsConfig) (tool.Tool, error) {
	handler := func(ctx tool.Context, args SearchPastIssuesArgs) (SearchPastIssuesResult, error) {
		// Tool implementation
	}
	
	return functiontool.New(functiontool.Config{
		Name:        "search_past_issues",
		Description: "搜索历史问题库",
	}, handler)
}
```

### String Truncation for Multi-byte Characters
- Always use `[]rune` for character-based truncation, not byte slicing
- This prevents breaking multi-byte characters (Chinese, emoji, etc.)
- Example:
```go
// CORRECT: Use []rune for multi-byte character handling
signature := pattern
runes := []rune(signature)
if len(runes) > 50 {
	signature = string(runes[:50])
}

// WRONG: Byte-based truncation breaks multi-byte characters
if len(signature) > 50 {
	signature = signature[:50] // This can break Chinese characters!
}
```

## Project Structure

```
.
├── cmd/
│   └── agent/              # Main application entry point
│       └── main.go        # Initializes database, memory service, and agent
├── internal/
│   ├── agent/             # Agent initialization and configuration
│   │   └── agent.go       # NewCodingAgent creates LLM agent with tools and system prompt
│   ├── config/            # Configuration management
│   │   └── config.go      # Loads config from environment variables
│   ├── memory/            # Memory service and storage
│   │   ├── service.go     # Implements ADK memory.Service interface
│   │   ├── store.go       # PostgresStore implements Store interface
│   │   ├── types.go       # Experience and ProjectRule types
│   │   ├── service_test.go
│   │   └── store_test.go
│   └── tools/             # ADK tool definitions
│       ├── tools.go       # BuildTools creates all agent tools
│       └── tools_test.go
├── migrations/            # Database migration files
│   └── 001_init.sql      # Creates project_rules and issue_history tables
├── bin/                  # Build output directory (gitignored)
├── go.mod                # Go module dependencies
├── go.sum                # Dependency checksums
├── README.md             # Project documentation
└── AGENTS.md             # This file
```

## Key Components

### Memory Service (`internal/memory/service.go`)
- Implements `google.golang.org/adk/memory.Service` interface
- Provides `Search()` for vector similarity search
- Provides `AddSession()` for automatic experience ingestion
- Uses `text-embedding-004` model for embeddings

### Store Interface (`internal/memory/store.go`)
- `GetProjectRules()`: Retrieves semantic memory (project rules)
- `SearchSimilarIssues()`: Vector similarity search using pgvector
- `SaveExperience()`: Saves episodic memory with embeddings
- `PostgresStore`: PostgreSQL implementation with pgvector support

### Agent Tools (`internal/tools/tools.go`)
1. **search_past_issues**: Search for similar past issues using vector similarity
2. **read_file_content**: Read file contents (with path traversal protection)
3. **list_directory**: List directory contents
4. **save_experience**: Explicitly save problem-solving experiences

### Agent Configuration (`internal/agent/agent.go`)
- Loads project rules from database
- Builds system prompt with rules
- Creates GenAI client and embedder
- Initializes LLM agent with `gemini-3-pro` model
- Configures tools and system instructions

## Environment Variables

**Required:**
- `GOOGLE_API_KEY`: Google GenAI API key for Gemini API access
- `DATABASE_URL`: PostgreSQL connection string (e.g., `postgres://user:password@localhost:5432/dbname`)

**Optional:**
- `WORK_DIR`: Working directory for file operations (default: current directory)

## Dependencies

This project uses:
- **Go 1.25+**
- **google.golang.org/adk v0.3.0**: ADK framework for agent development
- **google.golang.org/genai v1.40.0**: Google GenAI SDK (Gemini API)
- **github.com/jackc/pgx/v5 v5.8.0**: PostgreSQL driver
- **github.com/pgvector/pgvector-go v0.3.0**: pgvector extension for vector similarity search

Always check `go.mod` for current dependency versions before adding new packages.

## Database Schema

### project_rules (Semantic Memory)
- Stores project rules and guidelines
- Injected into system prompt
- Fields: `id`, `category`, `rule_content`, `priority`, `is_active`, `created_at`

### issue_history (Episodic Memory)
- Stores past experiences with vector embeddings
- Supports similarity search using pgvector
- Fields: `id`, `task_signature`, `error_pattern`, `root_cause`, `solution_summary`, `embedding` (vector(768)), `occurred_at`
- Index: IVFFlat index on `embedding` column for fast similarity search

## Agent Workflow

1. **Initialization** (`cmd/agent/main.go`):
   - Load configuration from environment
   - Connect to PostgreSQL database
   - Create memory service (implements ADK memory.Service)
   - Initialize coding agent with tools and system prompt
   - Start ADK launcher

2. **Memory Operations**:
   - **Semantic Memory**: Project rules loaded at startup, injected into system prompt
   - **Episodic Memory**: Automatically saved after sessions, searchable via vector similarity
   - **Procedural Memory**: Tools available for file operations and experience management

3. **Tool Usage**:
   - Agent can call tools to read files, search history, list directories, save experiences
   - Tools include security checks (path traversal prevention, size limits)

## Common Tasks

### Adding a New Tool
1. Define input/output structs in `internal/tools/tools.go`
2. Create tool handler function
3. Register tool in `BuildTools()` function
4. Update system prompt if needed (in `internal/agent/agent.go`)

### Adding a Project Rule
```sql
INSERT INTO project_rules (category, rule_content, priority) 
VALUES ('STYLE', 'Your rule here', 1);
```

### Testing Vector Search
```go
// Generate embedding
embedding, err := embedder.Embed(ctx, "error description")

// Search for similar issues
experiences, err := store.SearchSimilarIssues(ctx, embedding, 10)
```

## Notes

- The agent uses `gemini-3-pro` model by default
- Embeddings use `text-embedding-004` model (768 dimensions)
- Vector similarity search uses cosine similarity (via pgvector)
- File operations are restricted to `WORK_DIR` for security
- Multi-byte character handling is critical for Chinese text support
