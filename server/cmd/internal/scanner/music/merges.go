package music

import (
	"context"
	"database/sql"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
)

func (s *Scanner) mergeMusicArtist(ctx context.Context, qtx *database.Queries, scan *musicScanContext, redundant, owner int64) error {
	ids, err := qtx.MusicArtistTrackIDs(ctx, redundant)
	if err != nil {
		return err
	}
	for _, id := range ids {
		scan.invalidatedTracks[id] = true
	}
	err = qtx.MoveMusicArtistAliases(ctx, database.MoveMusicArtistAliasesParams{Owner: owner, Redundant: redundant})
	if err != nil {
		return err
	}
	err = qtx.MoveMusicArtistTracks(ctx, database.MoveMusicArtistTracksParams{Owner: sql.NullInt64{Int64: owner, Valid: true}, Redundant: sql.NullInt64{Int64: redundant, Valid: true}})
	if err != nil {
		return err
	}
	err = qtx.MoveMusicArtistGenres(ctx, database.MoveMusicArtistGenresParams{Owner: owner, Redundant: redundant})
	if err != nil {
		return err
	}
	err = qtx.MoveMusicArtistCredits(ctx, database.MoveMusicArtistCreditsParams{Owner: owner, Redundant: redundant})
	if err != nil {
		return err
	}
	err = qtx.MoveMusicArtistContributions(ctx, database.MoveMusicArtistContributionsParams{Owner: owner, Redundant: redundant})
	if err != nil {
		return err
	}
	err = qtx.DeleteMergedMusicMatch(ctx, database.DeleteMergedMusicMatchParams{EntityType: "musician", EntityID: redundant})
	if err != nil {
		return err
	}
	err = qtx.DeleteMergedMusicArtist(ctx, redundant)
	if err != nil {
		return err
	}
	scan.merged = true
	scan.musicianIDs = scanner.NewScanCache[string, int64]()
	scan.albumIDs = scanner.NewScanCache[string, int64]()
	return qtx.ReconcileMusicArtistSort(ctx, owner)
}

func (s *Scanner) mergeMusicAlbum(ctx context.Context, qtx *database.Queries, scan *musicScanContext, redundant, owner int64) error {
	ids, err := qtx.MusicAlbumTrackIDs(ctx, sql.NullInt64{Int64: redundant, Valid: true})
	if err != nil {
		return err
	}
	for _, id := range ids {
		scan.invalidatedTracks[id] = true
	}
	err = qtx.MoveMusicAlbumAliases(ctx, database.MoveMusicAlbumAliasesParams{Owner: owner, Redundant: redundant})
	if err != nil {
		return err
	}
	err = qtx.MoveMusicAlbumTracks(ctx, database.MoveMusicAlbumTracksParams{Owner: sql.NullInt64{Int64: owner, Valid: true}, Redundant: sql.NullInt64{Int64: redundant, Valid: true}})
	if err != nil {
		return err
	}
	err = qtx.MoveMusicAlbumGenres(ctx, database.MoveMusicAlbumGenresParams{Owner: owner, Redundant: redundant})
	if err != nil {
		return err
	}
	err = qtx.MoveMusicAlbumFallback(ctx, database.MoveMusicAlbumFallbackParams{Owner: owner, Redundant: redundant})
	if err != nil {
		return err
	}
	err = qtx.DeleteMergedMusicMatch(ctx, database.DeleteMergedMusicMatchParams{EntityType: "album", EntityID: redundant})
	if err != nil {
		return err
	}
	err = qtx.DeleteMergedMusicAlbum(ctx, redundant)
	if err != nil {
		return err
	}
	scan.merged = true
	scan.musicianIDs = scanner.NewScanCache[string, int64]()
	scan.albumIDs = scanner.NewScanCache[string, int64]()
	return reconcileAlbum(ctx, qtx, owner)
}
