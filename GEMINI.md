# Legacy Code Hunter (Legacy Code Hunter)

## Project Overview

**Legacy Code Hunter** is an intelligent coding assistant designed to help developers navigate, understand, and improve legacy codebases. Built with the Go Agent Development Kit (ADK) and Google's Gemini models, it leverages a sophisticated memory system to provide context-aware assistance.

The core philosophy is to augment the developer's capabilities with three types of memory:
*   **Semantic Memory:** Enforces project-specific rules and coding standards (e.g., "always use `sync.Map` for concurrent access").
*   **Episodic Memory:** Recalls past debugging experiences and solutions using Retrieval-Augmented Generation (RAG) powered by PostgreSQL and `pgvector`.
*   **Procedural Memory:** Utilizes a set of tools to actively investigate the codebase and interact with the knowledge base.

## Technical Architecture

*   **Language:** Go 1.25+
*   **AI Model:** Gemini 1.5 Pro (via `google.golang.org/genai`)
*   **Framework:** Google Go ADK (`google.golang.org/adk`)
*   **Database:** PostgreSQL with `pgvector` extension for vector similarity search.
*   **Key Dependencies:**
    *   `github.com/jackc/pgx/v5`: PostgreSQL driver.
    *   `github.com/pgvector/pgvector-go`: Vector operations for Go.

## Directory Structure

*   `cmd/agent/`: Main application entry point (`main.go`). Initializes the database, memory service, and launches the agent.
*   `internal/agent/`: Agent logic (`agent.go`). Configures the system prompt and initializes the Gemini model.
*   `internal/config/`: Configuration loading from environment variables.
*   `internal/memory/`:
    *   `store.go`: PostgreSQL storage implementation for project rules and issue history.
    *   `service.go`: Memory service logic for RAG and session management.
    *   `types.go`: Domain models (`ProjectRule`, `Experience`).
*   `internal/tools/`: Definition of tools available to the agent (`tools.go`).
*   `migrations/`: SQL scripts for database schema setup (`001_init.sql`).

## Building and Running

### Prerequisites

1.  **Go 1.25+** installed.
2.  **PostgreSQL** installed with the `vector` extension enabled.
    ```sql
    CREATE EXTENSION IF NOT EXISTS vector;
    ```

### Setup

1.  **Database Migration:**
    Apply the schema to your PostgreSQL database:
    ```bash
    psql -d <your_dbname> -f migrations/001_init.sql
    ```

2.  **Environment Variables:**
    Set the following environment variables:
    ```bash
    export GOOGLE_API_KEY="your-gemini-api-key"
    export DATABASE_URL="postgres://user:password@localhost:5432/dbname"
    export WORK_DIR="/absolute/path/to/project/root" # Optional, defaults to current directory
    ```

### Execution

To run the agent:

```bash
go run ./cmd/agent
```

To build a binary:

```bash
go build -o bin/agent ./cmd/agent
./bin/agent
```

## Agent Tools

The agent is equipped with the following tools (defined in `internal/tools/tools.go`):

1.  **`search_past_issues`**:
    *   *Input:* `error_description` (string)
    *   *Purpose:* Searches the `issue_history` table for similar past errors using vector cosine similarity. Returns relevant solutions.

2.  **`read_file_content`**:
    *   *Input:* `filepath` (string)
    *   *Purpose:* Safely reads the content of a file within the working directory.

3.  **`list_directory`** / **`list_files`**:
    *   *Input:* `path` (string)
    *   *Purpose:* Lists files and subdirectories to explore the project structure.

4.  **`save_experience`**:
    *   *Input:* `error_pattern`, `root_cause`, `solution`
    *   *Purpose:* Explicitly saves a new problem-solving experience to the knowledge base for future retrieval.

## Development Conventions

*   **Database Schema:** The `project_rules` table stores static guidelines (Style, Security, Architecture). The `issue_history` table stores dynamic problem-solving records with 768-dimensional embeddings.
*   **Security:** File access tools include strict checks to prevent path traversal outside the configured `WORK_DIR`.
*   **System Prompt:** The agent's system prompt is dynamically built to include active project rules from the database.
