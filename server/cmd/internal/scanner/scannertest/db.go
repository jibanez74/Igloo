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
	// One transaction: on a file database in the default rollback-journal mode,
	// each schema statement would otherwise commit, and fsync, on its own,
	// which took seconds per database.
	tx, err := db.Begin()
	if err != nil {
		db.Close()
		t.Fatalf("begin schema transaction: %v", err)
	}
	_, err = tx.Exec(sqlc.Schema)
	if err != nil {
		tx.Rollback()
		db.Close()
		t.Fatalf("initialize schema: %v", err)
	}
	err = tx.Commit()
	if err != nil {
		db.Close()
		t.Fatalf("commit schema: %v", err)
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
