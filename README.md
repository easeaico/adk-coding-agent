# Legacy Code Hunter (遗留代码猎手)

基于 ADK-go 与 PostgreSQL 的分层记忆智能体。

## 功能特点

- **语义记忆**: 存储项目规范和编码规则，确保生成的代码符合团队标准
- **情景记忆**: 使用向量数据库存储历史问题和解决方案，支持 RAG 检索
- **程序性记忆**: 通过工具调用读取代码、搜索历史、保存经验

## 技术栈

- Go 1.25+
- Google Agent SDK
- PostgreSQL + pgvector 或 SQLite（纯 Go，无 CGO 依赖）

## 快速开始

### 1. 环境准备

**方式 A: 使用 PostgreSQL（推荐用于生产环境）**

确保已安装 PostgreSQL 并启用 pgvector 扩展：

```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

执行数据库迁移：

```bash
psql -d your_database -f migrations/001_init.sql
```

**方式 B: 使用 SQLite（推荐用于开发/单机使用）**

无需额外安装，执行数据库迁移：

```bash
sqlite3 data.db < migrations/002_sqlite_init.sql
```

### 2. 配置环境变量

**PostgreSQL 配置：**
```bash
export GOOGLE_API_KEY="your-api-key"
export DB_TYPE="postgres"  # 可选，默认值
export DATABASE_URL="postgres://user:password@localhost:5432/dbname"
export WORK_DIR="/path/to/your/project"  # 可选，默认为当前目录
```

**SQLite 配置：**
```bash
export GOOGLE_API_KEY="your-api-key"
export DB_TYPE="sqlite"
export DATABASE_URL="./data.db"
export WORK_DIR="/path/to/your/project"  # 可选，默认为当前目录
```

### 3. 运行

```bash
go run ./cmd/agent
```

## 项目结构

```
.
├── cmd/agent/          # 主程序入口
├── internal/
│   ├── memory/         # 数据库与向量操作
│   ├── tools/          # ADK 工具定义
│   ├── llm/            # 模型包装器
│   └── service/        # 业务逻辑
└── migrations/         # SQL 迁移文件
```

## 使用示例

```
你: Main handler panic with concurrent map writes.

助手: 让我先搜索一下历史问题库...
[调用 search_past_issues 工具]

根据历史记录（Issue #42），这通常是因为在 Handler 中使用了全局 Map。
建议使用 `sync.Map` 或加锁。请检查以下代码...
```

## 许可证

Apache 2.0
