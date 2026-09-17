package scannertest

import (
	"context"
	"database/sql"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/sqlc"
)

// OpenDB opens a SQLite database at dsn with the production schema and the
// single-connection pool the scanners assume, and closes both on cleanup.
func OpenDB(t testing.TB, dsn string) (*sql.DB, *database.Queries) {
	t.Helper()
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	_, err = db.Exec(sqlc.Schema)
	if err != nil {
		db.Close()
		t.Fatalf("initialize schema: %v", err)
	}
	queries, err := database.Prepare(context.Background(), db)
	if err != nil {
		db.Close()
		t.Fatalf("prepare queries: %v", err)
	}
	t.Cleanup(func() {
		queries.Close()
		db.Close()
	})
	return db, queries
}
