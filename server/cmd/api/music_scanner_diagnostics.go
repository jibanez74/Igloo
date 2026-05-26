package main

import (
	"context"
	"database/sql"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	spotifyapi "igloo/cmd/internal/spotify"
)

const (
	spotifyMatchEntityAlbum     = "album"
	spotifyMatchEntityMusician  = "musician"
	spotifyMatchStatusMatched   = "matched"
	spotifyMatchStatusFailed    = "failed"
	spotifyMatchStatusUnmatched = "unmatched"
)

func (app *Application) recordSpotifyMatch(
	ctx context.Context,
	qtx *database.Queries,
	entityType string,
	entityID int64,
	spotifyID string,
) {
	if entityID == 0 {
		return
	}

	err := qtx.UpsertMusicSpotifyMatch(ctx, database.UpsertMusicSpotifyMatchParams{
		EntityType: entityType,
		EntityID:   entityID,
		SpotifyID:  helpers.NullString(spotifyID),
		Status:     spotifyMatchStatusMatched,
	})
	if err != nil && app.Logger != nil {
		app.Logger.Warn("failed to record Spotify match", "entity_type", entityType, "entity_id", entityID, "error", err)
	}
}

func (app *Application) recordSpotifyMatchFailure(
	ctx context.Context,
	qtx *database.Queries,
	entityType string,
	entityID int64,
	err error,
) {
	if entityID == 0 || err == nil {
		return
	}

	params := database.UpsertMusicSpotifyMatchParams{
		EntityType: entityType,
		EntityID:   entityID,
		Status:     spotifyMatchStatusFailed,
		Error:      sql.NullString{String: err.Error(), Valid: true},
	}

	matchErr, ok := spotifyapi.AsMatchError(err)
	if ok {
		info := matchErr.Info
		params.Status = spotifyMatchStatusFromReason(info.Reason)
		params.Reason = helpers.NullString(info.Reason)
		params.Score = helpers.NullInt64(int64(info.Score))
		params.ThresholdValue = helpers.NullInt64(int64(info.Threshold))
		params.CandidateName = helpers.NullString(info.CandidateName)
		params.CandidateArtist = helpers.NullString(info.CandidateArtist)
		params.SearchQuery = helpers.NullString(info.SearchQuery)
		params.Strategy = helpers.NullString(info.Strategy)
		if matchErr.Err != nil {
			params.Error = sql.NullString{String: matchErr.Err.Error(), Valid: true}
		} else {
			params.Error = sql.NullString{Valid: false}
		}
	}

	upsertErr := qtx.UpsertMusicSpotifyMatch(ctx, params)
	if upsertErr != nil && app.Logger != nil {
		app.Logger.Warn("failed to record Spotify match failure", "entity_type", entityType, "entity_id", entityID, "error", upsertErr)
	}
}

func spotifyMatchStatusFromReason(reason string) string {
	switch reason {
	case "no_results", "score_below_threshold", "track_mismatch", "empty_query":
		return spotifyMatchStatusUnmatched
	default:
		return spotifyMatchStatusFailed
	}
}
