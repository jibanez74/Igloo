package main

import (
	"context"
	"database/sql"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	_ "github.com/mattn/go-sqlite3"
)

func init() {
	var _ *database.Queries
}

func TestExtractYearFromReleaseDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "valid date format YYYY-MM-DD",
			input:    "2023-12-25",
			expected: 2023,
		},
		{
			name:     "valid date with single digit month/day",
			input:    "2023-1-5",
			expected: 2023,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "too short string",
			input:    "202",
			expected: 0,
		},
		{
			name:     "invalid format",
			input:    "invalid",
			expected: 0,
		},
		{
			name:     "only year",
			input:    "2023",
			expected: 2023,
		},
		{
			name:     "year with trailing text",
			input:    "2023-12-25T00:00:00",
			expected: 2023,
		},
		{
			name:     "year 2000",
			input:    "2000-01-01",
			expected: 2000,
		},
		{
			name:     "year 1999",
			input:    "1999-12-31",
			expected: 1999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractYearFromReleaseDate(tt.input)
			if result != tt.expected {
				t.Errorf("extractYearFromReleaseDate(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseBitRate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			name:     "valid bitrate",
			input:    "5000000",
			expected: 5000000,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "invalid format",
			input:    "invalid",
			expected: 0,
		},
		{
			name:     "zero",
			input:    "0",
			expected: 0,
		},
		{
			name:     "very large number",
			input:    "999999999999",
			expected: 999999999999,
		},
		{
			name:     "negative number",
			input:    "-1000",
			expected: -1000,
		},
		{
			name:     "bitrate with decimal",
			input:    "5000.5",
			expected: 0,
		},
		{
			name:     "bitrate with spaces",
			input:    " 5000000 ",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helpers.ParseBitRate(tt.input)
			if result != tt.expected {
				t.Errorf("ParseBitRate(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetOrCreateArtist(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()

	t.Run("creates new artist", func(t *testing.T) {
		tmdbID := 12345
		name := "Test Artist"
		profilePath := "/test/profile.jpg"

		artist, err := app.getOrCreateArtist(ctx, app.Queries, nil, tmdbID, name, profilePath)
		if err != nil {
			t.Fatalf("getOrCreateArtist failed: %v", err)
		}

		if artist == nil {
			t.Fatal("Expected non-nil artist")
		}

		if artist.Name != name {
			t.Errorf("Expected artist name %q, got %q", name, artist.Name)
		}

		if artist.TmdbID != int64(tmdbID) {
			t.Errorf("Expected TMDB ID %d, got %d", tmdbID, artist.TmdbID)
		}
	})

	t.Run("idempotent for same TMDB ID", func(t *testing.T) {
		tmdbID := 67890
		name := "Idempotent Artist"
		profilePath := "/idempotent/profile.jpg"

		firstArtist, err := app.getOrCreateArtist(ctx, app.Queries, nil, tmdbID, name, profilePath)
		if err != nil {
			t.Fatalf("First getOrCreateArtist failed: %v", err)
		}
		if firstArtist == nil {
			t.Fatal("First getOrCreateArtist returned nil artist")
		}

		secondArtist, err := app.getOrCreateArtist(ctx, app.Queries, nil, tmdbID, name, profilePath)
		if err != nil {
			t.Fatalf("Second getOrCreateArtist failed: %v", err)
		}
		if secondArtist == nil {
			t.Fatal("Second getOrCreateArtist returned nil artist")
		}

		if secondArtist.ID != firstArtist.ID {
			t.Errorf("Expected same artist ID %d, got %d", firstArtist.ID, secondArtist.ID)
		}
	})

	t.Run("cached artist refreshes mutable metadata", func(t *testing.T) {
		tmdbID := 22222
		scan := newMovieScanContext(nil)

		firstArtist, err := app.getOrCreateArtist(ctx, app.Queries, scan, tmdbID, "Old Artist", "")
		if err != nil {
			t.Fatalf("first getOrCreateArtist failed: %v", err)
		}
		if firstArtist == nil {
			t.Fatal("first getOrCreateArtist returned nil artist")
		}
		if scan.artistIDs[tmdbID] != firstArtist.ID {
			t.Fatalf("cached artist ID = %d, want %d", scan.artistIDs[tmdbID], firstArtist.ID)
		}

		secondArtist, err := app.getOrCreateArtist(ctx, app.Queries, scan, tmdbID, "New Artist", "/new/profile.jpg")
		if err != nil {
			t.Fatalf("second getOrCreateArtist failed: %v", err)
		}
		if secondArtist == nil {
			t.Fatal("second getOrCreateArtist returned nil artist")
		}
		if secondArtist.ID != firstArtist.ID {
			t.Fatalf("artist ID = %d, want cached ID %d", secondArtist.ID, firstArtist.ID)
		}
		if secondArtist.Name != "New Artist" || !secondArtist.Profile.Valid || secondArtist.Profile.String != "/new/profile.jpg" {
			t.Fatalf("artist = %+v, want refreshed name/profile", secondArtist)
		}

		var name string
		var profile sql.NullString
		err = app.DB.QueryRow("SELECT name, profile FROM artist WHERE tmdb_id = ?", tmdbID).Scan(&name, &profile)
		if err != nil {
			t.Fatalf("query artist: %v", err)
		}
		if name != "New Artist" || !profile.Valid || profile.String != "/new/profile.jpg" {
			t.Fatalf("stored artist name/profile = %q/%+v, want refreshed metadata", name, profile)
		}
	})

	t.Run("empty profile path handles null", func(t *testing.T) {
		tmdbID := 11111
		name := "No Profile Artist"
		profilePath := ""

		artist, err := app.getOrCreateArtist(ctx, app.Queries, nil, tmdbID, name, profilePath)
		if err != nil {
			t.Fatalf("getOrCreateArtist failed: %v", err)
		}

		if artist.Profile.Valid {
			t.Error("Expected profile to be invalid for empty path")
		}
	})
}

func TestManageSavepoint(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, value TEXT)")
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	ctx := context.Background()

	t.Run("successful function execution releases savepoint", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		savepointName := "test_savepoint"
		executed := false

		err = manageSavepoint(ctx, tx, savepointName, func() error {
			executed = true
			_, err := tx.ExecContext(ctx, "INSERT INTO test_table (value) VALUES ('test')")
			return err
		})

		if err != nil {
			t.Errorf("manageSavepoint returned error: %v", err)
		}

		if !executed {
			t.Error("Function was not executed")
		}

		var count int
		err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_table").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query count: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 row, got %d", count)
		}
	})

	t.Run("function error rolls back savepoint", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		savepointName := "test_savepoint_error"
		testError := sql.ErrNoRows

		err = manageSavepoint(ctx, tx, savepointName, func() error {
			_, err := tx.ExecContext(ctx, "INSERT INTO test_table (value) VALUES ('before error')")
			if err != nil {
				return err
			}
			return testError
		})

		if err == nil {
			t.Error("Expected error from manageSavepoint")
		}

		if err != testError {
			t.Errorf("Expected error %v, got %v", testError, err)
		}

		var count int
		err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_table").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query count: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 rows after rollback, got %d", count)
		}
	})

	t.Run("savepoint creation failure returns error", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		tx.Rollback()

		savepointName := "test_savepoint_fail"

		err = manageSavepoint(ctx, tx, savepointName, func() error {
			return nil
		})

		if err == nil {
			t.Error("Expected error when savepoint creation fails")
		}
	})
}
