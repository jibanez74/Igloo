package main

import (
	"context"
	"database/sql"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

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

func (app *Application) getOrCreateArtist(
	ctx context.Context,
	qtx *database.Queries,
	scan *movieScanContext,
	tmdbID int,
	name string,
	profilePath string,
) (*database.Artist, error) {
	if scan != nil {
		if artistID, ok := scan.artistIDs[tmdbID]; ok {
			return &database.Artist{
				ID:     artistID,
				Name:   name,
				TmdbID: int64(tmdbID),
			}, nil
		}
	}

	profile := helpers.NullString(profilePath)

	upserted, err := qtx.UpsertArtist(ctx, database.UpsertArtistParams{
		Name:    name,
		TmdbID:  int64(tmdbID),
		Profile: profile,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert artist failed: %w", err)
	}

	if scan != nil {
		scan.artistIDs[tmdbID] = upserted.ID
	}

	return &upserted, nil
}

// Savepoints let one scanner item fail without rolling back its batch.
func manageSavepoint(
	ctx context.Context,
	tx *sql.Tx,
	savepointName string,
	fn func() error,
) error {
	_, err := tx.ExecContext(ctx, fmt.Sprintf("SAVEPOINT %s", savepointName))
	if err != nil {
		return fmt.Errorf("failed to create savepoint %s: %w", savepointName, err)
	}

	err = fn()
	if err != nil {
		_, rollbackErr := tx.ExecContext(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", savepointName))
		if rollbackErr != nil {
			return fmt.Errorf("failed to rollback savepoint %s (original error: %w): %w", savepointName, err, rollbackErr)
		}

		return err
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf("RELEASE SAVEPOINT %s", savepointName))
	if err != nil {
		return fmt.Errorf("failed to release savepoint %s: %w", savepointName, err)
	}

	return nil
}
