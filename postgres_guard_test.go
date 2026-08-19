package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

// These tests need a live throwaway PostgreSQL. Set POSTGRES_HOST/PORT/DATABASE/
// USER/PASSWORD to a database you do not mind losing tables in — the point of the
// test is that it drops one.
func liveClient(t *testing.T) *PostgresClient {
	t.Helper()
	if os.Getenv("POSTGRES_HOST") == "" {
		t.Skip("set POSTGRES_* env to run guard tests against a throwaway database")
	}
	c, err := NewPostgresClient()
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if err := c.ensureConnected(); err != nil {
		t.Skipf("no database reachable: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func tableExists(t *testing.T, c *PostgresClient, name string) bool {
	t.Helper()
	var n int
	err := c.db.QueryRow(
		`SELECT count(*) FROM information_schema.tables WHERE table_name = $1`, name,
	).Scan(&n)
	if err != nil {
		t.Fatalf("existence check: %v", err)
	}
	return n > 0
}

// The allowlist matches on the query's leading keyword only. lib/pq sends
// argument-less queries over the simple query protocol, which permits multiple
// statements in one round trip — so a query that merely STARTS with SELECT
// carries anything after the semicolon past the guard.
func TestMultiStatementBypassesAllowlist(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	if _, err := c.db.Exec(`DROP TABLE IF EXISTS guard_canary; CREATE TABLE guard_canary (id int)`); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if !tableExists(t, c, "guard_canary") {
		t.Fatal("setup did not create the canary table")
	}

	// Defaults: SELECT allowed, DROP denied.
	c.settings = DefaultSQLFunctionSettings()

	// The query already contains LIMIT, so the appender leaves it alone and the
	// statement reaches the server intact. Without this the appended "LIMIT 10"
	// lands after the DROP and postgres rejects the whole batch on syntax —
	// which blocks the attack by accident, not by the guard.
	out, err := c.ExecuteSQL(ctx, "SELECT 1 LIMIT 1; DROP TABLE guard_canary", 10)

	if tableExists(t, c, "guard_canary") {
		t.Log("guard held: the trailing DROP did not execute")
		c.db.Exec(`DROP TABLE IF EXISTS guard_canary`) //nolint:errcheck
		return
	}
	t.Errorf("BYPASS: DROP executed with AllowDrop=false via a SELECT-prefixed query.\n  err=%v\n  out=%s", err, out)
}

// A leading CTE never matches any allowlisted prefix, so the operations loop
// falls through without blocking and the statement runs whatever it likes.
func TestCTEWriteBypassesAllowlist(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	if _, err := c.db.Exec(`DROP TABLE IF EXISTS guard_cte; CREATE TABLE guard_cte (id int); INSERT INTO guard_cte VALUES (1),(2),(3)`); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Cleanup(func() { c.db.Exec(`DROP TABLE IF EXISTS guard_cte`) }) //nolint:errcheck

	c.settings = DefaultSQLFunctionSettings() // DELETE denied

	out, err := c.ExecuteSQL(ctx, "WITH gone AS (DELETE FROM guard_cte RETURNING *) SELECT count(*) FROM gone", 10)

	var remaining int
	if scanErr := c.db.QueryRow(`SELECT count(*) FROM guard_cte`).Scan(&remaining); scanErr != nil {
		t.Fatalf("count: %v", scanErr)
	}
	if remaining == 3 {
		t.Log("guard held: the CTE DELETE did not execute")
		return
	}
	t.Errorf("BYPASS: DELETE executed with AllowDelete=false via a leading CTE; %d/3 rows left.\n  err=%v\n  out=%s", remaining, err, out)
}

// A trailing semicolon is what a model emits by default, and the LIMIT appender
// pastes after it unconditionally.
func TestTrailingSemicolonBreaksLimitAppend(t *testing.T) {
	c := liveClient(t)
	c.settings = DefaultSQLFunctionSettings()

	_, err := c.ExecuteSQL(context.Background(), "SELECT 1;", 10)
	if err != nil && strings.Contains(err.Error(), "syntax error") {
		t.Errorf("appending LIMIT after a trailing semicolon produced: %v", err)
	}
}

// The enforcement changes must not break ordinary use: reads work, the cap is
// applied, and an enabled write actually commits rather than being rolled back
// with the read-only transaction.
func TestReadPathStillWorks(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()
	c.settings = DefaultSQLFunctionSettings()

	if _, err := c.db.Exec(`DROP TABLE IF EXISTS happy; CREATE TABLE happy (id int); INSERT INTO happy SELECT generate_series(1,50)`); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Cleanup(func() { c.db.Exec(`DROP TABLE IF EXISTS happy`) }) //nolint:errcheck

	out, err := c.ExecuteSQL(ctx, "SELECT id FROM happy", 10)
	if err != nil {
		t.Fatalf("plain SELECT failed: %v", err)
	}
	if !strings.Contains(out, "Total rows: 10") {
		t.Errorf("row cap not applied, got:\n%s", out)
	}

	// A trailing semicolon is the common model output and must be accepted.
	if _, err := c.ExecuteSQL(ctx, "SELECT id FROM happy;", 5); err != nil {
		t.Errorf("trailing semicolon rejected: %v", err)
	}

	// An explicit LIMIT is respected rather than doubled.
	out, err = c.ExecuteSQL(ctx, "SELECT id FROM happy LIMIT 3", 100)
	if err != nil {
		t.Fatalf("explicit LIMIT failed: %v", err)
	}
	if !strings.Contains(out, "Total rows: 3") {
		t.Errorf("explicit LIMIT not respected, got:\n%s", out)
	}
}

func TestEnabledWriteCommits(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	if _, err := c.db.Exec(`DROP TABLE IF EXISTS writable; CREATE TABLE writable (id int)`); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Cleanup(func() { c.db.Exec(`DROP TABLE IF EXISTS writable`) }) //nolint:errcheck

	s := DefaultSQLFunctionSettings()
	s.AllowInsert = true
	c.settings = s

	if _, err := c.ExecuteSQL(ctx, "INSERT INTO writable VALUES (7)", 10); err != nil {
		t.Fatalf("permitted INSERT failed: %v", err)
	}

	var n int
	if err := c.db.QueryRow(`SELECT count(*) FROM writable`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("permitted INSERT did not persist: %d rows, want 1", n)
	}
}
