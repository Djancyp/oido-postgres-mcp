package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// SQLFunctionSettings holds configuration for which SQL operations are allowed.
type SQLFunctionSettings struct {
	AllowSelect   bool
	AllowInsert   bool
	AllowUpdate   bool
	AllowDelete   bool
	AllowCreate   bool
	AllowAlter    bool
	AllowDrop     bool
	AllowTruncate bool
}

// DefaultSQLFunctionSettings returns settings with only SELECT enabled by default.
func DefaultSQLFunctionSettings() *SQLFunctionSettings {
	return &SQLFunctionSettings{
		AllowSelect:   true,
		AllowInsert:   false,
		AllowUpdate:   false,
		AllowDelete:   false,
		AllowCreate:   false,
		AllowAlter:    false,
		AllowDrop:     false,
		AllowTruncate: false,
	}
}

// parseSQLFunctionSettings reads environment variables to configure SQL function permissions.
func parseSQLFunctionSettings() *SQLFunctionSettings {
	settings := DefaultSQLFunctionSettings()

	if val := os.Getenv("POSTGRES_ALLOW_SELECT"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			settings.AllowSelect = b
		}
	}
	if val := os.Getenv("POSTGRES_ALLOW_INSERT"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			settings.AllowInsert = b
		}
	}
	if val := os.Getenv("POSTGRES_ALLOW_UPDATE"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			settings.AllowUpdate = b
		}
	}
	if val := os.Getenv("POSTGRES_ALLOW_DELETE"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			settings.AllowDelete = b
		}
	}
	if val := os.Getenv("POSTGRES_ALLOW_CREATE"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			settings.AllowCreate = b
		}
	}
	if val := os.Getenv("POSTGRES_ALLOW_ALTER"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			settings.AllowAlter = b
		}
	}
	if val := os.Getenv("POSTGRES_ALLOW_DROP"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			settings.AllowDrop = b
		}
	}
	if val := os.Getenv("POSTGRES_ALLOW_TRUNCATE"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			settings.AllowTruncate = b
		}
	}

	return settings
}

// PostgresClient manages PostgreSQL database connections and queries.
type PostgresClient struct {
	connStr  string
	db       *sql.DB
	settings *SQLFunctionSettings
}

// NewPostgresClient creates a new PostgreSQL client from environment variables.
// If config is missing, returns client without connection — tools return errors until configured.
func NewPostgresClient() (*PostgresClient, error) {
	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")
	database := os.Getenv("POSTGRES_DATABASE")
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")

	settings := parseSQLFunctionSettings()

	if host == "" || port == "" || database == "" || user == "" {
		log.Println("Warning: POSTGRES_HOST, POSTGRES_PORT, POSTGRES_DATABASE, or POSTGRES_USER not set. Tools will return errors until configured.")
		return &PostgresClient{settings: settings}, nil
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=disable",
		host, port, user, database)

	if password != "" {
		connStr += fmt.Sprintf(" password=%s", password)
	}

	return &PostgresClient{connStr: connStr, settings: settings}, nil
}

// ensureConnected opens the DB connection on first use. Safe to call multiple times.
func (c *PostgresClient) ensureConnected() error {
	if c.db != nil {
		return nil
	}
	if c.connStr == "" {
		return fmt.Errorf("PostgreSQL not configured: set POSTGRES_HOST, POSTGRES_PORT, POSTGRES_DATABASE, POSTGRES_USER")
	}
	db, err := sql.Open("postgres", c.connStr)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	c.db = db
	return nil
}

// Close closes the database connection.
func (c *PostgresClient) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// writesEnabled reports whether any write operation is permitted. When nothing
// is, every statement can run inside a READ ONLY transaction and the database
// enforces it for us.
func (s *SQLFunctionSettings) writesEnabled() bool {
	return s.AllowInsert || s.AllowUpdate || s.AllowDelete ||
		s.AllowCreate || s.AllowAlter || s.AllowDrop || s.AllowTruncate
}

// limitClause matches a row cap already present at the end of the query, so a
// LIMIT inside a subquery does not stop us capping the outer result.
var limitClause = regexp.MustCompile(`(?is)\bLIMIT\s+\d+\s*(OFFSET\s+\d+\s*)?$`)

// prepareQuery trims the statement and appends a row cap to an uncapped SELECT.
// The trailing semicolon has to go first: a model emits "SELECT 1;" by default,
// and pasting " LIMIT 100" after it is a syntax error.
func prepareQuery(query string, limit int, s *SQLFunctionSettings) string {
	q := strings.TrimRight(strings.TrimSpace(query), "; \t\n\r")
	if !s.AllowSelect || !strings.HasPrefix(strings.ToUpper(q), "SELECT") {
		return q
	}
	if limitClause.MatchString(q) {
		return q
	}
	if limit <= 0 {
		limit = 100
	}
	return fmt.Sprintf("%s LIMIT %d", q, limit)
}

// checkAdvisory rejects a disallowed operation early so the caller gets a
// message naming the setting to flip.
//
// It is NOT the security boundary and must never be treated as one: it matches
// the statement's leading keyword, so "WITH gone AS (DELETE ...) SELECT" matches
// nothing here and sails through. Enforcement is the READ ONLY transaction in
// ExecuteSQL, which the server applies to CTEs and every other shape alike.
func (s *SQLFunctionSettings) checkAdvisory(query string) error {
	upper := strings.ToUpper(strings.TrimSpace(query))
	operations := []struct {
		prefix  string
		allowed bool
	}{
		{"SELECT", s.AllowSelect},
		{"INSERT", s.AllowInsert},
		{"UPDATE", s.AllowUpdate},
		{"DELETE", s.AllowDelete},
		{"CREATE", s.AllowCreate},
		{"ALTER", s.AllowAlter},
		{"DROP", s.AllowDrop},
		{"TRUNCATE", s.AllowTruncate},
	}
	for _, op := range operations {
		if strings.HasPrefix(upper, op.prefix) && !op.allowed {
			return fmt.Errorf("blocked: %s operations are not allowed (enable with POSTGRES_ALLOW_%s=true)",
				op.prefix, op.prefix)
		}
	}
	return nil
}

// ExecuteSQL executes a single SQL statement and returns results as a formatted
// string.
//
// Two server-side properties do the enforcing, because inspecting the query
// string cannot:
//
//   - PrepareContext puts the statement on the extended query protocol, which
//     carries exactly one command. "SELECT 1 LIMIT 1; DROP TABLE t" is rejected
//     by postgres instead of being waved through on its SELECT prefix. lib/pq
//     sends argument-less queries over the simple protocol otherwise, and that
//     one accepts a whole batch.
//   - With no write operation enabled, the statement runs in a READ ONLY
//     transaction. Postgres then refuses every write regardless of how it is
//     spelled, including inside a CTE.
//
// Per-verb permissions (INSERT yes, DROP no) cannot be expressed this way. Once
// any write is enabled only checkAdvisory stands between the model and the other
// write verbs, so grant those permissions with a database role scoped to what
// this connection should reach.
func (c *PostgresClient) ExecuteSQL(ctx context.Context, query string, limit int) (string, error) {
	if err := c.ensureConnected(); err != nil {
		return "", err
	}

	query = prepareQuery(query, limit, c.settings)
	if err := c.settings.checkAdvisory(query); err != nil {
		return "", err
	}

	tx, err := c.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: !c.settings.writesEnabled()})
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit has run

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return "", fmt.Errorf("query rejected: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return "", fmt.Errorf("query execution failed: %w", err)
	}

	out, err := formatRows(rows)
	rows.Close()
	if err != nil {
		return out, err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit: %w", err)
	}
	return out, nil
}

// formatRows renders a result set as a text table.
func formatRows(rows *sql.Rows) (string, error) {
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
	return result.String(), rows.Err()
}

// ListTables returns all tables in the current database.
func (c *PostgresClient) ListTables(ctx context.Context) (string, error) {
	if err := c.ensureConnected(); err != nil {
		return "", err
	}
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
	if err := c.ensureConnected(); err != nil {
		return "", err
	}
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
