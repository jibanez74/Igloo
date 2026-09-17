package scanner

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"igloo/cmd/internal/database"
)

// TxRunner serializes scanner writes on the shared scanner mutex and runs
// each unit of work in one transaction. It is shared by the movie and music
// scanners so the lock/begin/rollback/commit shell exists once.
type TxRunner struct {
	DB      *sql.DB
	Mu      *sync.Mutex
	Queries *database.Queries
}

// Run commits only when fn returns nil; any error rolls the transaction back
// and is returned unchanged so callers can match sentinel errors. committed,
// when not nil, runs after a successful commit while the mutex is still held,
// so runtime caches are invalidated before any other scanner write can land.
func (r TxRunner) Run(ctx context.Context, fn func(*database.Queries) error, committed func()) error {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()
	err = fn(r.Queries.WithTx(tx))
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	if committed != nil {
		committed()
	}
	return nil
}
