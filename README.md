# Oido PostgreSQL Plugin — PostgreSQL MCP Extension

This is a **PostgreSQL extension plugin** for Oido Studio. It provides tools for querying PostgreSQL databases, listing tables, and describing schemas via the MCP protocol.

## Features

- **`execute_sql`** — Execute SELECT queries against PostgreSQL with safety blocking of destructive operations
- **`list_tables`** — List all tables in the current database
- **`describe_table`** — Show column names, types, and constraints for a table

## Building

```bash
make build
# or
go build -o oido-postgres-mcp .
```

## Installing

Place the plugin in `plugins/oido-postgres/`. The plugin manager discovers it automatically on startup.

## Environment Variables

Set these before starting Oido Studio:

| Variable | Required | Description |
|----------|----------|-------------|
| `POSTGRES_HOST` | Yes | PostgreSQL server hostname/IP |
| `POSTGRES_PORT` | No | PostgreSQL port (default: 5432) |
| `POSTGRES_DATABASE` | Yes | Database name |
| `POSTGRES_USER` | Yes | Database username |
| `POSTGRES_PASSWORD` | No | Database password |

## Testing

Start Oido Studio, then in chat:

```
List all tables in the database
```

Or run commands:

```bash
/list-tables
/describe-table table=users
/sql-query query="SELECT * FROM public.users LIMIT 10"
```

## Architecture

```
oido-postgres/
├── plugin.json              # Plugin manifest
├── qwen-extension.json      # Qwen CLI extension config
├── oido-postgres-mcp        # Compiled binary
├── main.go                  # Entry point
├── mcp_server.go            # MCP tool handlers
├── postgres.go              # PostgreSQL client & query logic
├── Makefile                 # Build helper
├── QWEN.md                  # LLM context file
├── commands/                # Custom CLI commands
│   ├── sql-query.toml
│   ├── list-tables.toml
│   └── describe-table.toml
├── skills/
│   └── oido-postgres/
│       └── SKILL.md         # Skill documentation
└── README.md                # This file
```

## Safety

- **Destructive operations blocked**: DROP, DELETE, TRUNCATE, ALTER, CREATE, UPDATE, INSERT
- **SELECT queries only**: Ensures read-only access
- **Row limits**: Default 100 rows for SELECT queries
- **Connection pooling**: 10 max open, 5 idle connections

## Creating Your Own Database Plugin

Copy this directory and modify:

1. `plugin.json` — Update `id`, `name`, `description`, `binary`, `capabilities`, `config_schema`
2. `go.mod` — Update module path and database driver
3. `postgres.go` — Replace with your database client logic
4. `mcp_server.go` — Update tool names and handlers
5. Build: `make build`
