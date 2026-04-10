package main

import (
	"context"
	"database/sql"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

// extractYearFromReleaseDate extracts the year from a TMDB release date string using helpers.ParseDate.
// Returns 0 if the date string cannot be parsed.
func extractYearFromReleaseDate(releaseDate string) int {
	if releaseDate == "" {
		return 0
	}

	parsed, err := helpers.ParseDate(releaseDate)
	if err != nil {
		return 0
	}

	return parsed.Year()
}

// getOrCreateArtist upserts an artist in the database and returns it.
// Returns the database artist and an error.
func (app *Application) getOrCreateArtist(
	ctx context.Context,
	qtx *database.Queries,
	tmdbID int,
	name string,
	profilePath string,
) (*database.Artist, error) {
	profile := helpers.NullString(profilePath)

	upserted, err := qtx.UpsertArtist(ctx, database.UpsertArtistParams{
		Name:    name,
		TmdbID:  int64(tmdbID),
		Profile: profile,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert artist failed: %w", err)
	}

	return &upserted, nil
}

// manageSavepoint creates a savepoint, executes a function, and handles rollback/release.
// If the function returns an error, the savepoint is rolled back.
// Returns the error from the function, or a savepoint management error.
func manageSavepoint(
	ctx context.Context,
	tx *sql.Tx,
	savepointName string,
	fn func() error,
) error {
	// Create savepoint
	_, err := tx.ExecContext(ctx, fmt.Sprintf("SAVEPOINT %s", savepointName))
	if err != nil {
		return fmt.Errorf("failed to create savepoint %s: %w", savepointName, err)
	}

	// Execute function
	err = fn()
	if err != nil {
		// Rollback to savepoint on error
		_, rollbackErr := tx.ExecContext(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", savepointName))
		if rollbackErr != nil {
			return fmt.Errorf("failed to rollback savepoint %s (original error: %w): %w", savepointName, err, rollbackErr)
		}

		return err
	}

	// Release savepoint on success
	_, err = tx.ExecContext(ctx, fmt.Sprintf("RELEASE SAVEPOINT %s", savepointName))
	if err != nil {
		return fmt.Errorf("failed to release savepoint %s: %w", savepointName, err)
	}

	return nil
}
