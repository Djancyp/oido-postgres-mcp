---
name: oido-postgres
description: Execute SQL queries, list tables, describe PostgreSQL schemas, and explore database structure
---

# Oido PostgreSQL Extension

## Overview

The Oido PostgreSQL extension provides tools to query and explore PostgreSQL databases. Use these tools when users ask about database content, schema structure, or want to run SQL queries.

## Available Tools

### `oido-postgres_execute_sql`

Execute a SQL query against the PostgreSQL database.

**Parameters:**
- `query` (string, required): SQL query to execute (SELECT only for safety)
- `limit` (number, optional): Maximum rows to return (default: 100)

**When to use:**
- User asks to query the database
- User wants to see data from a specific table
- User asks for data analysis or reporting
- User wants to filter or aggregate data

**Example Usage:**
```
User: "Show me all active users"
→ Call oido-postgres_execute_sql with query: "SELECT * FROM public.users WHERE status = 'active'"

User: "How many orders were placed last month?"
→ Call oido-postgres_execute_sql with query: "SELECT COUNT(*) FROM public.orders WHERE created_at >= ..."
```

**Response Format:**
Returns formatted table results with:
- Column headers
- Row data
- Total row count

### `oido-postgres_list_tables`

List all tables in the PostgreSQL database.

**Parameters:** None

**When to use:**
- User asks "what tables are in the database?"
- User wants to explore database structure
- First step before querying unknown database

**Example Usage:**
```
User: "What tables do we have?"
→ Call oido-postgres_list_tables
```

**Response Format:**
Returns list of schema.table names.

### `oido-postgres_describe_table`

Describe a table's columns, types, and constraints.

**Parameters:**
- `schema` (string, optional): Database schema name (default: public)
- `table` (string, required): Table name to describe

**When to use:**
- User asks about table structure
- User wants to know column names or types
- User needs schema documentation

**Example Usage:**
```
User: "What columns are in the users table?"
→ Call oido-postgres_describe_table with schema: "public", table: "users"
```

**Response Format:**
Returns column details:
- Column name
- Data type
- Nullable status
- Default value

## Best Practices

1. **Use FULL table names**: Always use `DATABASE_NAME.SCHEMA_NAME.TABLE_NAME` format (e.g., `mydb.public.users`)
2. **Default to public schema**: If schema not specified, use `public`
3. **SELECT only**: Only SELECT queries allowed. Destructive ops blocked
4. **Limit results**: Default 100 rows, respect user-specified limits
5. **Context**: Mention results are from PostgreSQL database
6. **Safety**: Never attempt DROP, DELETE, TRUNCATE, ALTER, CREATE, UPDATE, INSERT

## Example Interactions

### Listing Tables

```
User: "What tables are in the database?"

Assistant: Let me check your PostgreSQL database structure.
[Calls oido-postgres_list_tables]

Tables (5):

public.users
public.orders
public.products
public.categories
public.order_items

Would you like me to describe any of these tables?
```

### Describing a Table

```
User: "Tell me about the users table"

Assistant: Let me get the structure of the users table.
[Calls oido-postgres_describe_table with schema: "public", table: "users"]

Table: public.users

Column Name          | Data Type            | Nullable | Default
---------------------|----------------------|----------|--------
id                   | integer              | NO       | nextval(...)
name                 | character varying    | NO       | NULL
email                | character varying    | NO       | NULL
created_at           | timestamp            | YES      | NULL
status               | character varying    | YES      | active
```

### Querying Data

```
User: "Show me the first 5 orders"

Assistant: I'll fetch the first 5 orders for you.
[Calls oido-postgres_execute_sql with query: "SELECT * FROM public.orders", limit: 5]

Query returned 6 columns

id | user_id | total | status | created_at | updated_at
-------------------------------------------------------
1 | 1 | 99.99 | completed | 2024-01-15 | 2024-01-16
2 | 2 | 149.50 | pending | 2024-01-17 | 2024-01-17
...

Total rows: 5
```

## Limitations

- Only SELECT queries allowed (safety block on destructive operations)
- Default row limit: 100 rows for SELECT queries
- Requires environment variables: POSTGRES_HOST, POSTGRES_PORT, POSTGRES_DATABASE, POSTGRES_USER
- Password optional (POSTGRES_PASSWORD)
- No authentication or private content beyond DB permissions
- Uses connection pooling: 10 max open, 5 idle, 5 min lifetime

## Related Commands

- `/sql-query` - Execute a SQL query (custom command)
- `/list-tables` - List all tables (custom command)
- `/describe-table` - Describe table structure (custom command)

## Triggers

Use these tools when you see:
- "query" or "SQL" or "SELECT"
- "tables" or "database structure"
- "columns" or "schema" or "table structure"
- "show me data" or "fetch records"
- PostgreSQL, Postgres, database questions