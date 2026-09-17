package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

// GetShowEpisode serves the player header for one episode: the episode row
// plus the season number and the show identity the page titles itself with
// and navigates back to, and the episode the player advances to when this
// one ends. Playable media is described by GetShowEpisodeTechnicalDetails,
// exactly as movies split details from technical details.
func (app *Application) GetShowEpisode(w http.ResponseWriter, r *http.Request) {
	media, err := parseMediaID(chi.URLParam(r, "id"), mediaKindEpisode)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	row, err := app.Queries.GetShowEpisodePlaybackDetails(r.Context(), media.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(media.notFoundMessage()), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get episode", "error", err, "media", media.String())
		helpers.ErrorJSON(w, errors.New("failed to fetch episode from server"))
		return
	}

	// The next episode belongs to the caller: its progress decides where the
	// player resumes it, and the row itself is nil after the show's last
	// episode.
	var nextEpisode any
	next, err := app.Queries.GetShowNextEpisode(r.Context(), database.GetShowNextEpisodeParams{
		UserID:    app.userIDFromRequest(r),
		EpisodeID: media.ID,
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		app.Logger.Error("failed to get next episode", "error", err, "media", media.String())
		helpers.ErrorJSON(w, errors.New("failed to fetch episode from server"))
		return
	}
	if err == nil {
		nextEpisode = next
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"show": map[string]any{
				"id":            row.ShowID,
				"name":          row.ShowName,
				"poster_path":   row.ShowPosterPath,
				"backdrop_path": row.ShowBackdropPath,
			},
			"next_episode": nextEpisode,
			"season": map[string]any{
				"season_number": row.SeasonNumber,
				"name":          row.SeasonName,
			},
			"episode": map[string]any{
				"id":             row.ID,
				"episode_number": row.EpisodeNumber,
				"name":           row.Name,
				"overview":       row.Overview,
				"air_date":       row.AirDate,
				"still_path":     row.StillPath,
				"tmdb_runtime":   row.TmdbRuntime,
				"vote_average":   row.VoteAverage,
				"vote_count":     row.VoteCount,
			},
		},
	})
}

// showChapterResponse is a show chapter with its start time normalized into
// the file's duration, the way movie chapters are before they are served.
type showChapterResponse struct {
	ID        int64          `json:"id"`
	Title     string         `json:"title"`
	StartTime int64          `json:"start_time"`
	Thumb     sql.NullString `json:"thumb"`
	FileID    int64          `json:"file_id"`
}

// GetShowEpisodeTechnicalDetails is the TV twin of GetMovieTechnicalDetails:
// the playback-relevant subset of the file behind the episode and its probed
// streams. The rows are the show tables' own (keyed by file_id), and a
// combined file answers identically for every episode it backs.
func (app *Application) GetShowEpisodeTechnicalDetails(w http.ResponseWriter, r *http.Request) {
	media, err := parseMediaID(chi.URLParam(r, "id"), mediaKindEpisode)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	tx, err := app.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		app.Logger.Error(beginTransactionLogMessage, "error", err)
		helpers.ErrorJSON(w, errors.New(fetchTechnicalDetailsMessage))
		return
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)

	file, err := qtx.GetShowFileForEpisode(ctx, media.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(media.notFoundMessage()), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get episode file", "error", err, "media", media.String())
		helpers.ErrorJSON(w, errors.New(fetchTechnicalDetailsMessage))
		return
	}

	videoStreams, err := qtx.GetShowVideoStreamsByFileID(ctx, file.ID)
	if err != nil {
		app.Logger.Error("failed to get video streams", "error", err, "media", media.String(), "file_id", file.ID)
		helpers.ErrorJSON(w, errors.New("failed to fetch video streams"))
		return
	}

	audioStreams, err := qtx.GetShowAudioStreamsByFileID(ctx, file.ID)
	if err != nil {
		app.Logger.Error("failed to get audio streams", "error", err, "media", media.String(), "file_id", file.ID)
		helpers.ErrorJSON(w, errors.New("failed to fetch audio streams"))
		return
	}

	subtitles, err := qtx.GetShowSubtitlesByFileID(ctx, file.ID)
	if err != nil {
		app.Logger.Error(getSubtitlesLogMessage, "error", err, "media", media.String(), "file_id", file.ID)
		helpers.ErrorJSON(w, errors.New(fetchSubtitlesMessage))
		return
	}

	chapters, err := qtx.GetShowChaptersByFileID(ctx, file.ID)
	if err != nil {
		app.Logger.Error("failed to get chapters", "error", err, "media", media.String(), "file_id", file.ID)
		helpers.ErrorJSON(w, errors.New("failed to fetch chapters"))
		return
	}

	durationSec := 0.0
	if file.Duration.Valid {
		durationSec = file.Duration.Float64
	}
	normalizedChapters := make([]showChapterResponse, len(chapters))
	for i, chapter := range chapters {
		normalizedChapters[i] = showChapterResponse{
			ID:        chapter.ID,
			Title:     chapter.Title,
			StartTime: normalizeChapterStartTimeSeconds(chapter.StartTime, durationSec),
			Thumb:     chapter.Thumb,
			FileID:    chapter.FileID,
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"file": map[string]any{
				"file_name": file.FileName,
				"size":      file.Size,
				"container": file.Container,
				// The value the client's direct-play container gate reads.
				"mime_type": videoContentType(file.Container, file.MimeType),
				"duration":  file.Duration,
			},
			"video_streams": videoStreams,
			"audio_streams": audioStreams,
			"subtitles":     subtitles,
			"chapters":      normalizedChapters,
		},
	})
}

// StreamEpisode direct-plays the file behind a TV episode.
func (app *Application) StreamEpisode(w http.ResponseWriter, r *http.Request) {
	app.serveStreamFile(w, r, mediaKindEpisode)
}

// showEpisodeWatchProgressStore is the episode adapter for the shared
// watch-progress handlers.
type showEpisodeWatchProgressStore struct {
	q *database.Queries
}

func (s showEpisodeWatchProgressStore) exists(ctx context.Context, id int64) (bool, error) {
	return s.q.ShowEpisodeExists(ctx, id)
}

func (s showEpisodeWatchProgressStore) get(ctx context.Context, userID int64, id int64) (watchProgressRow, error) {
	row, err := s.q.GetShowEpisodeWatchProgress(ctx, database.GetShowEpisodeWatchProgressParams{UserID: userID, EpisodeID: id})
	if err != nil {
		return watchProgressRow{}, err
	}
	return watchProgressRow{ProgressSec: row.ProgressSec, DurationSec: row.DurationSec, Watched: row.Watched, UpdatedAt: row.UpdatedAt}, nil
}

func (s showEpisodeWatchProgressStore) upsert(ctx context.Context, write watchProgressWrite) error {
	return s.q.UpsertShowEpisodeWatchProgress(ctx, database.UpsertShowEpisodeWatchProgressParams{
		UserID:        write.UserID,
		EpisodeID:     write.MediaID,
		ProgressSec:   write.ProgressSec,
		DurationSec:   write.DurationSec,
		SaveSessionID: write.SaveSessionID,
		SaveSequence:  write.SaveSequence,
	})
}

func (s showEpisodeWatchProgressStore) remove(ctx context.Context, userID int64, id int64) error {
	return s.q.DeleteShowEpisodeWatchProgress(ctx, database.DeleteShowEpisodeWatchProgressParams{UserID: userID, EpisodeID: id})
}

func (s showEpisodeWatchProgressStore) markWatched(ctx context.Context, userID int64, id int64) error {
	return s.q.MarkShowEpisodeWatched(ctx, database.MarkShowEpisodeWatchedParams{UserID: userID, EpisodeID: id})
}

func (s showEpisodeWatchProgressStore) markWatchedFromProgress(ctx context.Context, write watchProgressWrite) error {
	return s.q.MarkShowEpisodeWatchedFromProgress(ctx, database.MarkShowEpisodeWatchedFromProgressParams{
		UserID:        write.UserID,
		EpisodeID:     write.MediaID,
		SaveSessionID: write.SaveSessionID,
		SaveSequence:  write.SaveSequence,
	})
}

func (s showEpisodeWatchProgressStore) markUnwatched(ctx context.Context, userID int64, id int64) error {
	return s.q.MarkShowEpisodeUnwatched(ctx, database.MarkShowEpisodeUnwatchedParams{UserID: userID, EpisodeID: id})
}

// GetEpisodeWatchProgress returns the caller's progress on one episode.
func (app *Application) GetEpisodeWatchProgress(w http.ResponseWriter, r *http.Request) {
	app.getWatchProgress(w, r, mediaKindEpisode)
}

// UpdateEpisodeWatchProgress records a playback position for one episode.
func (app *Application) UpdateEpisodeWatchProgress(w http.ResponseWriter, r *http.Request) {
	app.updateWatchProgress(w, r, mediaKindEpisode)
}

// DeleteEpisodeWatchProgress clears the caller's progress on one episode.
func (app *Application) DeleteEpisodeWatchProgress(w http.ResponseWriter, r *http.Request) {
	app.deleteWatchProgress(w, r, mediaKindEpisode)
}

// SetEpisodeWatched marks one episode watched or unwatched for the caller.
func (app *Application) SetEpisodeWatched(w http.ResponseWriter, r *http.Request) {
	app.setWatched(w, r, mediaKindEpisode)
}
