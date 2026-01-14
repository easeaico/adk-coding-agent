# AGENTS.md

This file documents the `adk-coding-agent` repository for AI agents. It provides context, commands, and patterns to work effectively in this codebase.

## 1. Project Overview

**Legacy Code Hunter** is a Go-based intelligent coding assistant built with the Google Go ADK (`google.golang.org/adk`). It uses a tiered memory system (Semantic, Episodic, Procedural) backed by PostgreSQL/pgvector or SQLite to help developers understand and debug legacy code.

- **Main Agent Logic**: `internal/agent/hunter.go` (renamed from `agent.go`)
- **Memory Logic**: `internal/memory/`
- **Tools**: `internal/tools/`
- **Entry Point**: `cmd/agent/main.go`

## 2. Essential Commands

### Build & Run
- **Run Agent**: `go run ./cmd/agent`
- **Build Binary**: `go build -o bin/agent ./cmd/agent`
- **Run Binary**: `./bin/agent`

### Testing
- **Run All Tests**: `go test ./...`
- **Run Memory Tests**: `go test -v ./internal/memory/...`

### Database Setup

**PostgreSQL (default):**
- **Migrations**: `psql -d <dbname> -f migrations/001_init.sql`
- **Extensions**: Requires `pgvector` extension in PostgreSQL.

**SQLite (alternative):**
- **Migrations**: `sqlite3 data.db < migrations/002_sqlite_init.sql`
- **No extensions required**: Vector search is performed in application memory.

## 3. Directory Structure & Key Files

```text
.
├── cmd/
│   └── agent/
│       └── main.go           # Application entry point, dependency wiring
├── internal/
│   ├── agent/
│   │   └── hunter.go         # Agent definition, system prompt (Chinese), tool registration
│   ├── config/
│   │   └── config.go         # Env var loading (GOOGLE_API_KEY, DATABASE_URL, WORK_DIR)
│   ├── memory/
│   │   ├── service.go        # Memory service implementation
│   │   ├── store.go          # PostgreSQL + pgvector storage
│   │   ├── sqlite_store.go   # SQLite storage (in-memory vector search)
│   │   └── types.go          # Domain models (Experience, ProjectRule)
│   └── tools/
│       └── tools.go          # Tool definitions (Search, Read, List, Save)
├── migrations/
│   ├── 001_init.sql          # PostgreSQL schema (project_rules, issue_history)
│   └── 002_sqlite_init.sql   # SQLite schema (same tables, no pgvector)
├── AGENTS.md                 # This file
└── go.mod                    # Dependencies (Go 1.25+)
```

## 4. Code Patterns & Conventions

- **Language**: Go 1.25+
- **Error Handling**: Wrap errors with `fmt.Errorf("...: %w", err)`.
- **Database**:
    - PostgreSQL: Use `pgx/v5` driver with `pgvector` for vector similarity search.
    - SQLite: Use `modernc.org/sqlite` (pure Go, no CGO) with in-memory cosine similarity.
    - Use parameterized queries (`$1`, `$2` for PostgreSQL; `?` for SQLite).
- **Tools**:
    - Defined in `internal/tools/tools.go`.
    - Must implement `google.golang.org/adk/tool` interface.
    - Tools: `search_past_issues`, `read_file_content`, `list_directory` (and `list_files` alias), `save_experience`.
    - **Security**: File access tools strictly validate paths against `WORK_DIR`.
- **System Prompt**:
    - Located in `internal/agent/hunter.go`.
    - Dynamically builds prompt using `text/template`.
    - Injects `project_rules` from the database.
    - Base instructions are in Chinese.

## 5. Configuration (Env Vars)

- `GOOGLE_API_KEY`: Required for Gemini API.
- `DB_TYPE`: Database type, either `postgres` (default) or `sqlite`.
- `DATABASE_URL`: 
    - PostgreSQL: Connection string (e.g., `postgres://user:pass@localhost:5432/dbname`).
    - SQLite: File path (e.g., `./data.db` or `/path/to/database.db`).
- `WORK_DIR`: Root directory for file operations (defaults to CWD).

## 6. Development Tips

- **Agent Rename**: Note that the core agent file was renamed to `hunter.go`. References to `agent.go` in old docs might be outdated.
- **Memory System**:
    - **Semantic**: Static rules (`project_rules`).
    - **Episodic**: Past experiences (`issue_history`).
    - **Procedural**: Tools (filesystem, DB).
- **String Truncation**: Use `truncateString` helper in `tools.go` to handle multi-byte characters safely (`utf8.RuneStart`).
