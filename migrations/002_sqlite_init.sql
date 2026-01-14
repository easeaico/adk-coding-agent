-- SQLite Migration: Initialize database schema
-- This migration creates the same logical structure as the PostgreSQL version (001_init.sql)
-- but adapted for SQLite syntax and without pgvector extension.
-- Vector similarity search is performed in the application layer instead of the database.

-- Semantic Memory: Project Rules
-- Stores static, globally-effective knowledge like code style and architecture constraints
CREATE TABLE IF NOT EXISTS project_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    category TEXT NOT NULL,           -- e.g., "STYLE", "SECURITY", "ARCHITECTURE"
    rule_content TEXT NOT NULL,       -- e.g., "禁止在循环中使用 defer"
    priority INTEGER DEFAULT 1,       -- Rule weight
    is_active INTEGER DEFAULT 1,      -- SQLite uses INTEGER for boolean (0=false, 1=true)
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Index for category-based queries
CREATE INDEX IF NOT EXISTS idx_rules_category ON project_rules(category);

-- Episodic Memory: Issue History
-- Stores dynamically accumulated experience
-- Vector embeddings are stored as BLOB and searched in-memory
CREATE TABLE IF NOT EXISTS issue_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_signature TEXT,              -- Short fingerprint of task/error
    error_pattern TEXT,               -- Original error message or phenomenon description
    root_cause TEXT,                  -- Root cause analysis
    solution_summary TEXT,            -- Solution/code change summary
    embedding BLOB,                   -- Core: Embedding vector stored as binary (768 float32 values = 3072 bytes)
    occurred_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Insert sample rules for testing (same as PostgreSQL version)
INSERT INTO project_rules (category, rule_content, priority) VALUES
    ('STYLE', '禁止在循环中使用 defer', 1),
    ('STYLE', '所有导出的函数必须有文档注释', 1),
    ('SECURITY', '禁止在代码中硬编码密钥或密码', 2),
    ('ARCHITECTURE', '数据库操作必须通过 Repository 层', 1),
    ('ARCHITECTURE', 'HTTP Handler 不得直接调用数据库', 1);
