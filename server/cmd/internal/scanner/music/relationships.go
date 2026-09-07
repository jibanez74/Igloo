package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
)

func (s *Scanner) processSpotifyGenres(ctx context.Context, qtx *database.Queries, scan *musicScanContext, musicianID int64, genres []string) error {
	err := qtx.DeleteMusicArtistSpotifyGenres(ctx, musicianID)
	if err != nil {
		return err
	}
	return s.processSpotifyEntityGenres(ctx, qtx, scan, genres, func(genreID int64) error {
		return qtx.SaveMusicArtistSpotifyGenre(ctx, database.SaveMusicArtistSpotifyGenreParams{MusicianID: musicianID, GenreID: genreID})
	})
}

func (s *Scanner) processSpotifyAlbumGenres(ctx context.Context, qtx *database.Queries, scan *musicScanContext, albumID int64, genres []string) error {
	err := qtx.DeleteMusicAlbumSpotifyGenres(ctx, albumID)
	if err != nil {
		return err
	}
	return s.processSpotifyEntityGenres(ctx, qtx, scan, genres, func(genreID int64) error {
		return qtx.SaveMusicAlbumSpotifyGenre(ctx, database.SaveMusicAlbumSpotifyGenreParams{AlbumID: albumID, GenreID: genreID})
	})
}

func (s *Scanner) processSpotifyEntityGenres(ctx context.Context, qtx *database.Queries, scan *musicScanContext, genres []string, createRelationship func(int64) error) error {
	if len(genres) == 0 {
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
	return nil
}

func (s *Scanner) getOrCreateMusicGenreID(ctx context.Context, qtx *database.Queries, scan *musicScanContext, tag string) (int64, error) {
	cacheKey := scanner.NormalizedScanCacheKey(tag, "music")
	genreID, ok := scan.genreIDs.Get(cacheKey)
	if ok {
		return genreID, nil
	}

	genre, err := qtx.FindMusicGenreIdentity(ctx, cacheKey)
	if err == nil {
		scan.genreIDs.Set(cacheKey, genre.ID)
		return genre.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	genre, err = qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
		Tag:       tag,
		GenreType: "music",
	})
	if err != nil {
		return 0, err
	}

	err = qtx.SaveMusicGenreIdentity(ctx, database.SaveMusicGenreIdentityParams{IdentityKey: cacheKey, GenreID: genre.ID})
	if err != nil {
		return 0, err
	}
	scan.genreIDs.Set(cacheKey, genre.ID)
	return genre.ID, nil
}
