package music

import (
	"context"
	"database/sql"
	"errors"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
)

func (s *Scanner) getOrCreateMusicGenreID(ctx context.Context, qtx *database.Queries, scan *musicScanContext, tag string) (int64, error) {
	cacheKey := scanner.NormalizedScanCacheKey(tag, "music")
	genreID, ok := scan.genreIDs.Get(cacheKey)
	if ok {
		return genreID, nil
	}

	genreID, err := qtx.FindMusicGenreIdentity(ctx, cacheKey)
	if err == nil {
		scan.genreIDs.Set(cacheKey, genreID)
		return genreID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	genreID, err = qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
		Tag:       tag,
		GenreType: "music",
	})
	if err != nil {
		return 0, err
	}

	err = qtx.SaveMusicGenreIdentity(ctx, database.SaveMusicGenreIdentityParams{IdentityKey: cacheKey, GenreID: genreID})
	if err != nil {
		return 0, err
	}
	scan.genreIDs.Set(cacheKey, genreID)
	return genreID, nil
}
