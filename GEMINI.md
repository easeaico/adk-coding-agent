# Project Context: Legacy Code Hunter (adk-memory-agent)

## Project Overview
**Legacy Code Hunter** is an intelligent coding assistant agent built using the **Go ADK (Agent Development Kit)** and **Google Gemini**. Its primary purpose is to help developers understand, debug, and fix code in legacy codebases by leveraging a tiered memory system.

**Key Technologies:**
*   **Language:** Go 1.25+
*   **AI Model:** Google Gemini (`gemini-3-pro` for reasoning, `text-embedding-004` for embeddings)
*   **Framework:** `google.golang.org/adk` v0.3.0
*   **Database:** PostgreSQL 15+ with `pgvector` extension (for vector similarity search)
*   **Drivers:** `pgx/v5`, `pgvector-go`

## Architecture & Memory System
The agent employs a three-tiered memory architecture:

1.  **Semantic Memory (Project Rules):**
    *   **Purpose:** Enforces coding standards, security policies, and architectural constraints.
    *   **Storage:** `project_rules` table in PostgreSQL.
    *   **Usage:** Loaded at startup and injected into the System Prompt.

2.  **Episodic Memory (Issue History):**
    *   **Purpose:** Stores past debugging experiences (RAG - Retrieval Augmented Generation).
    *   **Storage:** `issue_history` table in PostgreSQL.
    *   **Mechanism:** Uses 768-dimensional vector embeddings to find similar past issues via `search_past_issues` tool.

3.  **Procedural Memory (Tools):**
    *   **Purpose:** Interaction with the filesystem and knowledge base.
    *   **Implementation:** Defined in `internal/tools/`.

## Key Files
*   `cmd/agent/main.go`: Application entry point. Initializes DB, Memory Service, and Agent Launcher.
*   `internal/config/config.go`: Loads environment variables (`DATABASE_URL`, `GOOGLE_API_KEY`, `WORK_DIR`).
*   `internal/agent/agent.go`: Configures the `gemini-3-pro` agent, system prompt template, and tool registration.
*   `internal/memory/store.go`: PostgreSQL implementation using `pgxpool` and `pgvector`.
*   `internal/tools/tools.go`: Implementation of agent tools (`read_file_content`, `search_past_issues`, etc.).
*   `migrations/001_init.sql`: Database schema definition.

## Building and Running

### Prerequisites
*   Go 1.25 or higher
*   PostgreSQL with `vector` extension enabled (`CREATE EXTENSION vector;`)
*   Google GenAI API Key

### Setup
1.  **Environment Variables:**
    ```bash
    export GOOGLE_API_KEY="your-api-key"
    export DATABASE_URL="postgres://user:pass@localhost:5432/dbname"
    # Optional: defaults to current directory
    export WORK_DIR="/path/to/project"
    ```

2.  **Database Migration:**
    ```bash
    psql -d your_db_name -f migrations/001_init.sql
    ```

### Execution
*   **Run Agent:**
    ```bash
    go run ./cmd/agent
    ```
*   **Build Binary:**
    ```bash
    go build -o bin/agent ./cmd/agent
    ```
*   **Run Tests:**
    ```bash
    go test ./...
    ```

## Development Conventions

### Agent Behavior
*   **Memory First:** The agent is instructed to check `search_past_issues` *before* attempting to solve complex bugs.
*   **Knowledge Capture:** After solving a problem, the agent should use `save_experience` to store the solution for future reference.

### Codebase Standards
*   **Security:**
    *   File access tools (`read_file_content`, `list_directory`) **must** perform path traversal checks (using `filepath.Rel` and checking for `..`).
    *   SQL queries **must** use parameter placeholders (`$1`, `$2`) to prevent injection.
*   **Safety:**
    *   `read_file_content` truncates files larger than 10KB to prevent context window exhaustion.
    *   Truncation logic respects UTF-8 boundaries (`utf8.RuneStart`) to avoid corrupting multi-byte characters.
*   **Error Handling:** Wrap errors with context using `fmt.Errorf("...: %w", err)`.
