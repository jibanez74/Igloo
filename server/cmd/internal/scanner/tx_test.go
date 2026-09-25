package scanner

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"

	"igloo/cmd/internal/database"

	_ "github.com/mattn/go-sqlite3"
)

// A transaction that cannot even begin is reported as such, and neither the
// unit of work nor the committed hook runs.
func TestTxRunnerReportsBeginFailure(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	err = db.Close()
	if err != nil {
		t.Fatal(err)
	}
	runner := TxRunner{DB: db, Mu: &sync.Mutex{}}
	ran, committed := false, false
	err = runner.Run(context.Background(), func(*database.Queries) error {
		ran = true
		return nil
	}, func() { committed = true })
	if err == nil || !strings.HasPrefix(err.Error(), "failed to start transaction: ") || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("begin failure error = %v", err)
	}
	if ran || committed {
		t.Fatalf("work ran=%v committed=%v without a transaction", ran, committed)
	}
}
