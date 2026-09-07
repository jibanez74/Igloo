package music

import (
	"context"
	"fmt"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
)

func (s *Scanner) processSpotifyGenres(ctx context.Context, qtx *database.Queries, scan *musicScanContext, musicianID int64, genres []string) error {
	return s.processSpotifyEntityGenres(ctx, qtx, scan, musicianID, genres, scan.spotifyMusicianGenresHandled, func(genreID int64) error {
		return s.createMusicianGenreIfNeeded(ctx, qtx, scan, musicianID, genreID)
	})
}

func (s *Scanner) processSpotifyAlbumGenres(ctx context.Context, qtx *database.Queries, scan *musicScanContext, albumID int64, genres []string) error {
	return s.processSpotifyEntityGenres(ctx, qtx, scan, albumID, genres, scan.spotifyAlbumGenresHandled, func(genreID int64) error {
		return s.createAlbumGenreIfNeeded(ctx, qtx, scan, albumID, genreID)
	})
}

func (s *Scanner) processSpotifyEntityGenres(ctx context.Context, qtx *database.Queries, scan *musicScanContext, entityID int64, genres []string, handled scanner.ScanCache[int64, struct{}], createRelationship func(int64) error) error {
	alreadyHandled := handled.Has(entityID)
	if len(genres) == 0 || alreadyHandled {
		return nil
	}
	for _, tag := range genres {
		genreID, err := s.getOrCreateMusicGenreID(ctx, qtx, scan, tag)
		if err != nil {
			return fmt.Errorf("Spotify genre %q failed: %w", tag, err)
		}
		err = createRelationship(genreID)
		if err != nil {
			return fmt.Errorf("Spotify genre %q relationship failed: %w", tag, err)
		}
	}
	handled.Set(entityID, struct{}{})
	return nil
}

func (s *Scanner) getOrCreateMusicGenreID(ctx context.Context, qtx *database.Queries, scan *musicScanContext, tag string) (int64, error) {
	cacheKey := scanner.NormalizedScanCacheKey(tag, "music")
	genreID, ok := scan.genreIDs.Get(cacheKey)
	if ok {
		return genreID, nil
	}

	genre, err := qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
		Tag:       tag,
		GenreType: "music",
	})
	if err != nil {
		return 0, err
	}

	scan.genreIDs.Set(cacheKey, genre.ID)
	return genre.ID, nil
}

func (s *Scanner) createMusicianAlbumIfNeeded(ctx context.Context, qtx *database.Queries, scan *musicScanContext, musicianID, albumID int64) error {
	return createCachedMusicRelationshipIfNeeded(scan.musicianAlbums, musicianID, albumID, func() error {
		return qtx.CreateMusicianAlbum(ctx, database.CreateMusicianAlbumParams{
			MusicianID: musicianID,
			AlbumID:    albumID,
		})
	})
}

func (s *Scanner) createMusicianGenreIfNeeded(ctx context.Context, qtx *database.Queries, scan *musicScanContext, musicianID, genreID int64) error {
	return createCachedMusicRelationshipIfNeeded(scan.musicianGenres, musicianID, genreID, func() error {
		return qtx.UpsertMusicianGenre(ctx, database.UpsertMusicianGenreParams{
			MusicianID: musicianID,
			GenreID:    genreID,
		})
	})
}

func (s *Scanner) createAlbumGenreIfNeeded(ctx context.Context, qtx *database.Queries, scan *musicScanContext, albumID, genreID int64) error {
	return createCachedMusicRelationshipIfNeeded(scan.albumGenres, albumID, genreID, func() error {
		return qtx.UpsertAlbumGenre(ctx, database.UpsertAlbumGenreParams{
			AlbumID: albumID,
			GenreID: genreID,
		})
	})
}

func createCachedMusicRelationshipIfNeeded(cache scanner.ScanCache[musicIDPair, struct{}], leftID, rightID int64, create func() error) error {
	cacheKey := musicIDPair{left: leftID, right: rightID}
	exists := cache.Has(cacheKey)
	if exists {
		return nil
	}

	err := create()
	if err != nil {
		return err
	}

	cache.Set(cacheKey, struct{}{})
	return nil
}
