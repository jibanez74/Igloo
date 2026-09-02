package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

// errWatchRoomStreamDrift means the room's stored track ordinals no longer
// select the streams they were validated against: the movie file was replaced
// and rescanned since the room was created (audit H14).
var errWatchRoomStreamDrift = errors.New("this room's movie file was replaced and its selected tracks changed; delete the room and create it again")

// verifyWatchRoomAudioPin re-resolves the room's stored audio ordinal against
// the movie's current streams and compares it with the identity pinned at
// creation. A NULL pin (legacy room, silent movie) skips the check.
//
// It returns the audio streams it loaded so the caller can hand them to
// GetOrCreateRoomHLSSession as preloadedAudio; on a session miss that branch
// would otherwise re-run this exact query. A nil slice (pin check skipped)
// simply degrades to the fetch inside createHLSSession.
//
// The streams come from MovieStreamsCache because this runs on every manifest
// refresh; a rescan invalidates it, so drift is still caught.
func (app *Application) verifyWatchRoomAudioPin(ctx context.Context, room database.WatchRoom) ([]database.AudioStream, error) {
	if !room.AudioStreamIndex.Valid {
		return nil, nil
	}

	streams, err := app.movieStreamsFor(ctx, room.MovieID)
	if err != nil {
		return nil, fmt.Errorf("failed to load audio streams for pin check: %w", err)
	}
	audioStreams := streams.Audio
	if room.AudioTrack >= int64(len(audioStreams)) {
		return nil, errWatchRoomStreamDrift
	}
	current := audioStreams[room.AudioTrack]
	if current.StreamIndex != room.AudioStreamIndex.Int64 {
		return nil, errWatchRoomStreamDrift
	}
	if room.AudioLanguage.Valid && current.Language.String != room.AudioLanguage.String {
		return nil, errWatchRoomStreamDrift
	}
	return audioStreams, nil
}

// verifyWatchRoomSubtitlePin is the subtitle counterpart of
// verifyWatchRoomAudioPin. It is also the only pin a direct-mode room checks:
// direct playback serves the container's own default audio, so only the
// subtitle ordinal can silently repoint after a rescan. A direct-play client
// calls this once per byte-range request, which is why it reads the cache.
func (app *Application) verifyWatchRoomSubtitlePin(ctx context.Context, room database.WatchRoom) error {
	if !room.SubtitleStreamIndex.Valid || !room.SubtitleTrack.Valid {
		return nil
	}

	streams, err := app.movieStreamsFor(ctx, room.MovieID)
	if err != nil {
		return fmt.Errorf("failed to load subtitles for pin check: %w", err)
	}
	subtitles := streams.Subtitles
	if room.SubtitleTrack.Int64 >= int64(len(subtitles)) {
		return errWatchRoomStreamDrift
	}
	current := subtitles[room.SubtitleTrack.Int64]
	if current.StreamIndex != room.SubtitleStreamIndex.Int64 {
		return errWatchRoomStreamDrift
	}
	if room.SubtitleLanguage.Valid && current.Language.String != room.SubtitleLanguage.String {
		return errWatchRoomStreamDrift
	}
	return nil
}

func (app *Application) verifyWatchRoomStreamPins(ctx context.Context, room database.WatchRoom) ([]database.AudioStream, error) {
	audioStreams, err := app.verifyWatchRoomAudioPin(ctx, room)
	if err != nil {
		return nil, err
	}
	return audioStreams, app.verifyWatchRoomSubtitlePin(ctx, room)
}

func (app *Application) rejectDriftedWatchRoom(w http.ResponseWriter, room database.WatchRoom, err error) bool {
	if errors.Is(err, errWatchRoomStreamDrift) {
		helpers.ErrorJSON(w, err, http.StatusConflict)
		return true
	}
	if err != nil {
		app.Logger.Error("failed to verify watch room stream pins", "error", err, "room_id", room.ID, "movie_id", room.MovieID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return true
	}
	return false
}

func (app *Application) WatchRoomHLSManifest(w http.ResponseWriter, r *http.Request) {
	room, _, ok := app.loadAuthorizedWatchRoomForRequest(w, r)
	if !ok {
		return
	}

	if room.PlaybackMode == watchRoomPlaybackModeDirect {
		helpers.ErrorJSON(w, errors.New("this room uses direct playback"), http.StatusBadRequest)
		return
	}

	audioStreams, pinErr := app.verifyWatchRoomStreamPins(r.Context(), room)
	if app.rejectDriftedWatchRoom(w, room, pinErr) {
		return
	}

	session, err := app.GetOrCreateRoomHLSSession(r.Context(), room.ID, room.MovieID, room.PlaybackMode, int(room.AudioTrack), nil, audioStreams)
	if err != nil {
		app.Logger.Error("watch room hls session failed", "error", err, "room_id", room.ID)
		writeHLSSessionError(w, err)
		return
	}

	baseURL := strings.TrimSuffix(r.URL.Path, helpers.HLS_PLAYLIST_FILENAME)
	audioTrack := int(room.AudioTrack)
	querySuffix := buildHLSAssetQuerySuffix(hlsAssetQueryParams{AudioTrack: &audioTrack})

	playlist, err := buildHLSPlaylistBody(r.Context(), session, session.DurationSec, baseURL, querySuffix)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		app.Logger.Error("watch room hls playlist unavailable", "error", err, "room_id", room.ID)
		writeHLSSessionError(w, err)
		return
	}

	writeHLSPlaylistHeaders(w, session)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(playlist))
}

func (app *Application) WatchRoomHLSSegment(w http.ResponseWriter, r *http.Request) {
	room, _, ok := app.loadAuthorizedWatchRoomForRequest(w, r)
	if !ok {
		return
	}

	filename := chi.URLParam(r, "filename")
	if !isAllowedHLSFilename(filename) {
		helpers.ErrorJSON(w, errors.New("invalid segment filename"), http.StatusBadRequest)
		return
	}

	key := RoomHLSSessionKey(room.ID)
	session, found, err := app.getActiveRoomHLSSession(room.ID, key)
	if err != nil {
		app.Logger.Error("watch room hls session fetch failed", "error", err, "room_id", room.ID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage), http.StatusInternalServerError)
		return
	}
	if !found {
		helpers.ErrorJSON(w, errors.New("session not found; request the manifest first"), http.StatusNotFound)
		return
	}

	serveReadyHLSSegment(w, r, session, filename)
}

func (app *Application) StreamWatchRoomMovie(w http.ResponseWriter, r *http.Request) {
	room, _, ok := app.loadAuthorizedWatchRoomForRequest(w, r)
	if !ok {
		return
	}

	if room.PlaybackMode != watchRoomPlaybackModeDirect {
		helpers.ErrorJSON(w, errors.New("this room uses HLS playback"), http.StatusBadRequest)
		return
	}

	if app.rejectDriftedWatchRoom(w, room, app.verifyWatchRoomSubtitlePin(r.Context(), room)) {
		return
	}

	movie, err := app.movieStreamFile(r.Context(), room.MovieID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusNotFound)
			return
		}

		app.Logger.Error("failed to get watch room movie for streaming", "error", err, "room_id", room.ID, "movie_id", room.MovieID)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie from server"))
		return
	}

	err = serveMediaFile(w, r, movie.Path, movie.Name, movie.ContentType)
	if err != nil {
		app.Logger.Error("failed to stream watch room movie file", "error", err, "path", movie.Path, "room_id", room.ID, "movie_id", room.MovieID)
	}
}
