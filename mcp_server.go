package main

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewMCPHandler creates a new MCP handler for PostgreSQL tools.
func NewMCPHandler(pg *PostgresClient) *MCPHandler {
	return &MCPHandler{pg: pg}
}

// MCPHandler implements MCP tool handlers.
type MCPHandler struct {
	pg *PostgresClient
}

// ExecuteSQLArgs represents the arguments for execute_sql tool.
type ExecuteSQLArgs struct {
	Query string `json:"query" jsonschema:"SQL query to execute (SELECT only for safety)"`
	Limit int    `json:"limit" jsonschema:"Maximum rows to return for SELECT queries (default: 100)"`
}

// ListTablesArgs represents the arguments for list_tables tool.
type ListTablesArgs struct{}

// DescribeTableArgs represents the arguments for describe_table tool.
type DescribeTableArgs struct {
	Schema string `json:"schema" jsonschema:"Database schema name (default: public)"`
	Table  string `json:"table" jsonschema:"Table name to describe"`
}

// RunMCPServer starts the MCP server using stdio transport.
func RunMCPServer() {
	pgClient, err := NewPostgresClient()
	if err != nil {
		log.Printf("Warning: Postgres client init failed (tools will return errors): %v", err)
		pgClient = &PostgresClient{settings: DefaultSQLFunctionSettings()}
	}
	defer pgClient.Close()

	handler := NewMCPHandler(pgClient)

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "oido-postgres",
		Version: "1.0.0",
	}, nil)

	// Register execute_sql tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "oido-postgres_execute_sql",
		Description: "Execute a SQL query against the PostgreSQL database. Only SELECT queries are allowed for safety. Returns formatted results with column headers.",
	}, handler.HandleExecuteSQL)

	// Register list_tables tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "oido-postgres_list_tables",
		Description: "List all tables in the PostgreSQL database. Returns schema.table names.",
	}, handler.HandleListTables)

	// Register describe_table tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "oido-postgres_describe_table",
		Description: "Describe a table's columns, types, and constraints. Requires schema and table name.",
	}, handler.HandleDescribeTable)

	// Run server using stdio transport
	ctx := context.Background()
	log.Println("Oido PostgreSQL MCP Server starting on stdio...")

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}

// HandleExecuteSQL executes a SQL query.
func (h *MCPHandler) HandleExecuteSQL(ctx context.Context, req *mcp.CallToolRequest, args ExecuteSQLArgs) (*mcp.CallToolResult, any, error) {
	if args.Query == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: query parameter is required"},
			},
			IsError: true,
		}, nil, nil
	}

	result, err := h.pg.ExecuteSQL(ctx, args.Query, args.Limit)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("SQL Error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil, nil
}

// HandleListTables lists all tables.
func (h *MCPHandler) HandleListTables(ctx context.Context, req *mcp.CallToolRequest, args ListTablesArgs) (*mcp.CallToolResult, any, error) {
	result, err := h.pg.ListTables(ctx)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil, nil
}

// HandleDescribeTable describes a table.
func (h *MCPHandler) HandleDescribeTable(ctx context.Context, req *mcp.CallToolRequest, args DescribeTableArgs) (*mcp.CallToolResult, any, error) {
	if args.Table == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: table parameter is required"},
			},
			IsError: true,
		}, nil, nil
	}

	schema := args.Schema
	if schema == "" {
		schema = "public"
	}

	result, err := h.pg.DescribeTable(ctx, schema, args.Table)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, nil, nil
}
