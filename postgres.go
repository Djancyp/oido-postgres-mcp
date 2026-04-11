package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// PostgresClient manages PostgreSQL database connections and queries.
type PostgresClient struct {
	connStr string
	db      *sql.DB
}

// NewPostgresClient creates a new PostgreSQL client from environment variables.
func NewPostgresClient() (*PostgresClient, error) {
	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")
	database := os.Getenv("POSTGRES_DATABASE")
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")

	if host == "" || port == "" || database == "" || user == "" {
		return nil, fmt.Errorf("missing required env vars: POSTGRES_HOST, POSTGRES_PORT, POSTGRES_DATABASE, POSTGRES_USER")
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=disable",
		host, port, user, database)

	if password != "" {
		connStr += fmt.Sprintf(" password=%s", password)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresClient{connStr: connStr, db: db}, nil
}

// Close closes the database connection.
func (c *PostgresClient) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// ExecuteSQL executes a raw SQL query and returns results as a formatted string.
func (c *PostgresClient) ExecuteSQL(ctx context.Context, query string, limit int) (string, error) {
	// Safety: block destructive operations
	upperQuery := strings.ToUpper(strings.TrimSpace(query))
	destructive := []string{"DROP ", "DELETE ", "TRUNCATE ", "ALTER ", "CREATE ", "UPDATE ", "INSERT "}
	for _, d := range destructive {
		if strings.HasPrefix(upperQuery, d) {
			return "", fmt.Errorf("blocked: %s operations are not allowed for safety", strings.TrimSpace(d))
		}
	}

	// Append LIMIT if SELECT and not already present
	if strings.HasPrefix(upperQuery, "SELECT") && !strings.Contains(upperQuery, "LIMIT") {
		if limit <= 0 {
			limit = 100
		}
		query = fmt.Sprintf("%s LIMIT %d", query, limit)
	}

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return "", fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("failed to get columns: %w", err)
	}

	if len(columns) == 0 {
		return "Query executed successfully. No columns returned.", nil
	}

	// Build result table
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Query returned %d columns\n\n", len(columns)))

	// Header
	header := strings.Join(columns, " | ")
	result.WriteString(header + "\n")
	result.WriteString(strings.Repeat("-", len(header)) + "\n")

	// Rows
	rowCount := 0
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return result.String(), fmt.Errorf("failed to scan row: %w", err)
		}

		rowVals := make([]string, len(columns))
		for i, v := range values {
			if v == nil {
				rowVals[i] = "NULL"
			} else {
				rowVals[i] = fmt.Sprintf("%v", v)
			}
		}

		result.WriteString(strings.Join(rowVals, " | ") + "\n")
		rowCount++
	}

	result.WriteString(fmt.Sprintf("\nTotal rows: %d", rowCount))
	return result.String(), nil
}

// ListTables returns all tables in the current database.
func (c *PostgresClient) ListTables(ctx context.Context) (string, error) {
	query := `
		SELECT schemaname, tablename 
		FROM pg_catalog.pg_tables 
		WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY schemaname, tablename;
	`

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return "", fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var schema, table string
		if err := rows.Scan(&schema, &table); err != nil {
			return "", fmt.Errorf("failed to scan table: %w", err)
		}
		tables = append(tables, fmt.Sprintf("%s.%s", schema, table))
	}

	if len(tables) == 0 {
		return "No tables found in the database.", nil
	}

	return fmt.Sprintf("Tables (%d):\n\n%s", len(tables), strings.Join(tables, "\n")), nil
}

// DescribeTable returns column info for a table.
func (c *PostgresClient) DescribeTable(ctx context.Context, schema, table string) (string, error) {
	query := `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position;
	`

	rows, err := c.db.QueryContext(ctx, query, schema, table)
	if err != nil {
		return "", fmt.Errorf("failed to describe table: %w", err)
	}
	defer rows.Close()

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Table: %s.%s\n\n", schema, table))
	result.WriteString("Column Name          | Data Type            | Nullable | Default\n")
	result.WriteString(strings.Repeat("-", 80) + "\n")

	count := 0
	for rows.Next() {
		var colName, dataType, isNullable string
		var defaultVal sql.NullString

		if err := rows.Scan(&colName, &dataType, &isNullable, &defaultVal); err != nil {
			return "", fmt.Errorf("failed to scan column: %w", err)
		}

		defaultStr := "NULL"
		if defaultVal.Valid {
			defaultStr = defaultVal.String
		}

		result.WriteString(fmt.Sprintf("%-20s | %-20s | %-10s | %s\n",
			colName, dataType, isNullable, defaultStr))
		count++
	}

	if count == 0 {
		return fmt.Sprintf("Table %s.%s not found or has no columns.", schema, table), nil
	}

	return result.String(), nil
}
