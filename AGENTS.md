# AGENTS.md

This file serves as the definitive guide for AI agents (and human developers) working on the **Legacy Code Hunter** project. It documents the architecture, memory systems, tools, and operational procedures.

## Project Overview

**Legacy Code Hunter** is an intelligent coding assistant built with the [Go ADK](https://github.com/google/go-adk) (Agent Development Kit). It is designed to help developers understand legacy code, debug complex issues, and build a knowledge base of solutions over time.

### Core Capabilities
*   **Tiered Memory System**:
    *   **Semantic Memory**: Project rules and guidelines (stored in PostgreSQL).
    *   **Episodic Memory**: History of past issues and solutions (stored in PostgreSQL with pgvector).
    *   **Procedural Memory**: Tools for interacting with the filesystem and database.
*   **RAG (Retrieval-Augmented Generation)**: Automatically searches past experiences to solve new problems.
*   **Automatic Learning**: Ingests session data into long-term memory.

## Architecture

### Components
*   **Agent**: Uses `gemini-3-pro` for reasoning and code generation.
*   **Embeddings**: Uses `text-embedding-004` (768 dimensions) for vector search.
*   **Database**: PostgreSQL with `pgvector` extension.
*   **Framework**: `google.golang.org/adk` v0.3.0.

### Directory Structure
```
.
├── cmd/
│   └── agent/              # Main entry point (initialization & wiring)
├── internal/
│   ├── agent/             # Agent definition, system prompt, tool registration
│   ├── config/            # Environment variable loading
│   ├── memory/            # Memory service implementation
│   │   ├── service.go     # ADK memory.Service implementation (AddSession/Search)
│   │   ├── store.go       # PostgreSQL storage layer (pgxpool + pgvector)
│   │   └── types.go       # Domain models (Experience, ProjectRule)
│   └── tools/             # Tool definitions (Search, Read, List, Save)
├── migrations/            # SQL schemas for database setup
└── go.mod                 # Dependencies
```

## Memory System Details

### 1. Semantic Memory (`project_rules`)
*   **Purpose**: Stores invariant rules (coding style, security policies).
*   **Mechanism**: Loaded at startup and injected into the System Prompt.
*   **Schema**: `id`, `category`, `rule_content`, `priority`, `is_active`.

### 2. Episodic Memory (`issue_history`)
*   **Purpose**: Stores past debugging experiences.
*   **Mechanism**: 
    *   **retrieval**: `SearchSimilarIssues` finds relevant past solutions using vector cosine similarity.
    *   **storage**: `AddSession` automatically captures user queries and agent responses > 20 chars.
*   **Schema**: `id`, `task_signature`, `error_pattern`, `root_cause`, `solution_summary`, `embedding` (vector(768)).

### 3. Procedural Memory (Tools)
The agent is equipped with the following tools defined in `internal/tools/`:

| Tool Name | Description | Inputs |
|-----------|-------------|--------|
| `search_past_issues` | Search the knowledge base for similar past errors. | `error_description` (string) |
| `read_file_content` | Read file content (safe path access). | `filepath` (string) |
| `list_directory` | List files and subdirectories. | `path` (string) |
| `save_experience` | Explicitly save a solution to the knowledge base. | `error_pattern`, `root_cause`, `solution` |

## Development Guide

### Prerequisites
*   Go 1.25+
*   PostgreSQL 15+ (with `vector` extension installed)
*   Google GenAI API Key

### Environment Variables
Create a `.env` file or export these variables:

```bash
export GOOGLE_API_KEY="your-gemini-api-key"
export DATABASE_URL="postgres://user:pass@localhost:5432/dbname"
export WORK_DIR="/absolute/path/to/project/root" # Optional, defaults to CWD
```

### Database Setup
1.  Enable pgvector: `CREATE EXTENSION IF NOT EXISTS vector;`
2.  Run migration: `psql -d dbname -f migrations/001_init.sql`

### Build & Run
```bash
# Run directly
go run ./cmd/agent

# Build binary
go build -o bin/agent ./cmd/agent
./bin/agent
```

### Testing
```bash
# Run all tests
go test ./...

# Run memory service tests
go test -v ./internal/memory/...
```

## Code Standards
*   **Imports**: Group standard lib, 3rd party, and internal packages.
*   **Error Handling**: Wrap errors with `fmt.Errorf("...: %w", err)`.
*   **Multi-byte Safety**: Use `[]rune` for string truncation to avoid breaking Chinese characters.
*   **SQL**: Use parameterized queries (`$1`, `$2`) to prevent injection.

## Workflow for Agents
When acting as an agent on this codebase:
1.  **Check Rules**: Read `migrations/001_init.sql` or DB to understand valid data structures.
2.  **Use Tools**: Prefer `read_file_content` over `cat` (it has safety checks).
3.  **Memory First**: Before fixing a bug, check `search_past_issues` to see if it was solved before.
4.  **Save Knowledge**: After solving a complex bug, call `save_experience` if the automatic ingestion might miss the nuance.
