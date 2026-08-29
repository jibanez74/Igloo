package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

const (
	watchRoomPlaybackModeDirect = "direct"
	maxWatchRoomRequestSize     = 1024 * 1024 // 1 MB
)

// background is a package-level background context used for operations that must
// outlive the originating HTTP request (e.g. HLS warm-up after room creation).
var background = context.Background()

type watchRoomMemberSummary struct {
	ID     int64   `json:"id"`
	Name   string  `json:"name"`
	Avatar *string `json:"avatar"`
}

type watchRoomListItem struct {
	ID           int64                    `json:"id"`
	MovieID      int64                    `json:"movie_id"`
	MovieTitle   string                   `json:"movie_title"`
	MoviePoster  *string                  `json:"movie_poster"`
	Owner        watchRoomMemberSummary   `json:"owner"`
	Members      []watchRoomMemberSummary `json:"members"`
	PlaybackMode string                   `json:"playback_mode"`
	IsOwner      bool                     `json:"is_owner"`
	CreatedAt    string                   `json:"created_at"`
}

type watchRoomDetail struct {
	ID            int64                    `json:"id"`
	MovieID       int64                    `json:"movie_id"`
	MovieTitle    string                   `json:"movie_title"`
	MoviePoster   *string                  `json:"movie_poster"`
	Owner         watchRoomMemberSummary   `json:"owner"`
	Members       []watchRoomMemberSummary `json:"members"`
	PlaybackMode  string                   `json:"playback_mode"`
	AudioTrack    int64                    `json:"audio_track"`
	SubtitleTrack *int64                   `json:"subtitle_track"`
	IsOwner       bool                     `json:"is_owner"`
	CreatedAt     string                   `json:"created_at"`
}

type createWatchRoomRequest struct {
	MovieID        int64   `json:"movie_id"`
	Mode           string  `json:"mode"`
	AudioTrack     int64   `json:"audio_track"`
	SubtitleTrack  *int64  `json:"subtitle_track"`
	InvitedUserIDs []int64 `json:"invited_user_ids"`
}

func parseRoomID(r *http.Request) (int64, error) {
	idParam := chi.URLParam(r, "id")
	roomID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil || roomID <= 0 {
		return 0, errors.New("invalid room id")
	}
	return roomID, nil
}

func isValidPlaybackMode(mode string) bool {
	if mode == watchRoomPlaybackModeDirect {
		return true
	}
	return helpers.IsAllowedHLSProfile(mode)
}

// directPlayAudioSelectionUnambiguous reports whether direct play can
// guarantee which audio stream a browser decodes: refuse on ambiguity, not on
// absence. Mirrors directPlayAudioSelectionEligible in web/src/lib/playback.ts
// — keep the two in sync. Background: "Direct Play Eligibility and Fallback"
// in docs/ffmpeg.md.
func directPlayAudioSelectionUnambiguous(audioStreams []database.AudioStream) bool {
	if len(audioStreams) <= 1 {
		return true
	}

	defaultCount := 0
	for _, stream := range audioStreams {
		if stream.IsDefault {
			defaultCount++
		}
	}
	if defaultCount == 0 {
		// No flags at all: browsers follow container track order, so the
		// first stream is the one that plays.
		return true
	}

	return defaultCount == 1 && audioStreams[0].IsDefault
}

func deduplicateAndFilterUserIDs(ids []int64, ownerID int64) []int64 {
	seen := make(map[int64]bool)
	result := []int64{}
	for _, id := range ids {
		if id == ownerID || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

func (app *Application) loadRoomMembers(ctx context.Context, roomID int64) ([]watchRoomMemberSummary, error) {
	rows, err := app.Queries.GetWatchRoomMembers(ctx, roomID)
	if err != nil {
		return nil, err
	}
	members := make([]watchRoomMemberSummary, len(rows))
	for i, row := range rows {
		var avatar *string
		if row.Avatar.Valid {
			avatar = &row.Avatar.String
		}
		members[i] = watchRoomMemberSummary{
			ID:     row.ID,
			Name:   row.Name,
			Avatar: avatar,
		}
	}
	return members, nil
}

func findMemberByID(members []watchRoomMemberSummary, id int64) (watchRoomMemberSummary, bool) {
	for _, m := range members {
		if m.ID == id {
			return m, true
		}
	}
	return watchRoomMemberSummary{}, false
}

func (app *Application) GetWatchRooms(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	rooms, err := app.Queries.GetWatchRoomsForUser(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to fetch watch rooms", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	roomIDs := make([]int64, 0, len(rooms))
	movieIDs := make([]int64, 0, len(rooms))
	seenMovieIDs := make(map[int64]bool)
	for _, room := range rooms {
		roomIDs = append(roomIDs, room.ID)
		if seenMovieIDs[room.MovieID] {
			continue
		}
		seenMovieIDs[room.MovieID] = true
		movieIDs = append(movieIDs, room.MovieID)
	}

	movies, err := app.Queries.GetMoviesByIDs(r.Context(), movieIDs)
	if err != nil {
		app.Logger.Error("failed to fetch movies for watch rooms", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	memberRows, err := app.Queries.GetWatchRoomMembersByRoomIDs(r.Context(), roomIDs)
	if err != nil {
		app.Logger.Error("failed to fetch members for watch rooms", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	movieByID := make(map[int64]database.GetMoviesByIDsRow, len(movies))
	for _, movie := range movies {
		movieByID[movie.ID] = movie
	}

	membersByRoomID := make(map[int64][]watchRoomMemberSummary, len(roomIDs))
	for _, row := range memberRows {
		var avatar *string
		if row.Avatar.Valid {
			avatar = &row.Avatar.String
		}
		membersByRoomID[row.RoomID] = append(membersByRoomID[row.RoomID], watchRoomMemberSummary{
			ID:     row.ID,
			Name:   row.Name,
			Avatar: avatar,
		})
	}

	items := make([]watchRoomListItem, 0, len(rooms))
	for _, room := range rooms {
		movie, movieOK := movieByID[room.MovieID]
		if !movieOK {
			app.Logger.Error("failed to find movie for watch room", "room_id", room.ID, "movie_id", room.MovieID)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
			return
		}

		members := membersByRoomID[room.ID]
		owner, _ := findMemberByID(members, room.OwnerUserID)

		var moviePoster *string
		if movie.PosterPath.Valid {
			moviePoster = &movie.PosterPath.String
		}

		items = append(items, watchRoomListItem{
			ID:           room.ID,
			MovieID:      room.MovieID,
			MovieTitle:   movie.Title,
			MoviePoster:  moviePoster,
			Owner:        owner,
			Members:      members,
			PlaybackMode: room.PlaybackMode,
			IsOwner:      room.OwnerUserID == userID,
			CreatedAt:    room.CreatedAt,
		})
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"rooms": items},
	})
}

// One joined query authorizes the member and returns the room, the same way
// the media handlers and the WebSocket upgrade do. An unknown room and a room
// the caller is not a member of both come back as 403, which is deliberate:
// the room id is guessable, and the other room endpoints already refuse to
// distinguish the two.
func (app *Application) GetWatchRoom(w http.ResponseWriter, r *http.Request) {
	room, userID, ok := app.loadAuthorizedWatchRoomForRequest(w, r)
	if !ok {
		return
	}

	roomID := room.ID

	movie, err := app.Queries.GetMovieByID(r.Context(), room.MovieID)
	if err != nil {
		app.Logger.Error("failed to fetch movie for watch room", "error", err, "room_id", roomID, "movie_id", room.MovieID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	members, err := app.loadRoomMembers(r.Context(), roomID)
	if err != nil {
		app.Logger.Error("failed to fetch members for watch room", "error", err, "room_id", roomID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	owner, _ := findMemberByID(members, room.OwnerUserID)

	var moviePoster *string
	if movie.PosterPath.Valid {
		moviePoster = &movie.PosterPath.String
	}

	var subtitleTrack *int64
	if room.SubtitleTrack.Valid {
		subtitleTrack = &room.SubtitleTrack.Int64
	}

	detail := watchRoomDetail{
		ID:            room.ID,
		MovieID:       room.MovieID,
		MovieTitle:    movie.Title,
		MoviePoster:   moviePoster,
		Owner:         owner,
		Members:       members,
		PlaybackMode:  room.PlaybackMode,
		AudioTrack:    room.AudioTrack,
		SubtitleTrack: subtitleTrack,
		IsOwner:       room.OwnerUserID == userID,
		CreatedAt:     room.CreatedAt,
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"room": detail},
	})
}

func (app *Application) CreateWatchRoom(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	var req createWatchRoomRequest
	if err := helpers.ReadJSON(w, r, &req, maxWatchRoomRequestSize); err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if req.MovieID <= 0 {
		helpers.ErrorJSON(w, errors.New("movie_id is required"), http.StatusBadRequest)
		return
	}

	if req.Mode == "" {
		helpers.ErrorJSON(w, errors.New("mode is required"), http.StatusBadRequest)
		return
	}

	if !isValidPlaybackMode(req.Mode) {
		helpers.ErrorJSON(w, errors.New("invalid playback mode"), http.StatusBadRequest)
		return
	}

	if req.AudioTrack < 0 {
		helpers.ErrorJSON(w, errors.New("audio_track must be non-negative"), http.StatusBadRequest)
		return
	}

	if req.SubtitleTrack != nil && *req.SubtitleTrack < 0 {
		helpers.ErrorJSON(w, errors.New("subtitle_track must be non-negative"), http.StatusBadRequest)
		return
	}

	movie, err := app.Queries.GetMovieByID(r.Context(), req.MovieID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusBadRequest)
			return
		}
		app.Logger.Error("failed to verify movie for watch room", "error", err, "movie_id", req.MovieID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	// audio_track is an ordinal into the movie's audio streams, so it has to be
	// validated against the movie. Without this the room is created and members
	// invited before playback fails on the first manifest request.
	audioStreams, err := app.Queries.GetAudioStreamsByMovieID(r.Context(), req.MovieID)
	if err != nil {
		app.Logger.Error("failed to load audio streams for watch room", "error", err, "movie_id", req.MovieID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	movieHasAudio := len(audioStreams) > 0
	if !movieHasAudio && req.AudioTrack != 0 {
		helpers.ErrorJSON(w, errors.New("audio_track is not valid for a movie without audio"), http.StatusBadRequest)
		return
	}

	audioTrackOutOfRange := movieHasAudio && req.AudioTrack >= int64(len(audioStreams))
	if audioTrackOutOfRange {
		helpers.ErrorJSON(w, fmt.Errorf("audio track %d out of range (0-%d)", req.AudioTrack, len(audioStreams)-1), http.StatusBadRequest)
		return
	}

	// subtitle_track is an ordinal too; validate it against the movie so a
	// room can never be created pointing at a subtitle that does not exist.
	var subtitleStreams []database.Subtitle
	if req.SubtitleTrack != nil {
		subtitleStreams, err = app.Queries.GetSubtitlesByMovieID(r.Context(), req.MovieID)
		if err != nil {
			app.Logger.Error("failed to load subtitles for watch room", "error", err, "movie_id", req.MovieID)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
			return
		}
		if len(subtitleStreams) == 0 {
			helpers.ErrorJSON(w, errors.New("subtitle_track is not valid for a movie without subtitles"), http.StatusBadRequest)
			return
		}
		if *req.SubtitleTrack >= int64(len(subtitleStreams)) {
			helpers.ErrorJSON(w, fmt.Errorf("subtitle track %d out of range (0-%d)", *req.SubtitleTrack, len(subtitleStreams)-1), http.StatusBadRequest)
			return
		}
	}

	// Direct playback serves the raw container, so every member hears its first
	// audio track no matter what the room stores.
	directWithNonFirstAudio := req.Mode == watchRoomPlaybackModeDirect && req.AudioTrack != 0
	if directWithNonFirstAudio {
		helpers.ErrorJSON(w, errors.New("direct playback always uses the first audio track; choose another playback mode to pick a different one"), http.StatusBadRequest)
		return
	}

	directWithAmbiguousAudio := req.Mode == watchRoomPlaybackModeDirect &&
		!directPlayAudioSelectionUnambiguous(audioStreams)
	if directWithAmbiguousAudio {
		helpers.ErrorJSON(w, errors.New("direct playback cannot guarantee which audio track this movie plays; choose another playback mode"), http.StatusBadRequest)
		return
	}

	// Mirror the web client's remaining static direct-play rules so a room can
	// never be created in a mode the members' browsers cannot play: browsers
	// only direct-play MP4 containers, and 10-bit / 4:2:2 / 4:4:4 H.264 does
	// not decode even though the codec name passes.
	if req.Mode == watchRoomPlaybackModeDirect {
		if movieContentType(movie.Container, movie.MimeType) != "video/mp4" {
			helpers.ErrorJSON(w, errors.New("direct playback is only available for MP4 movies; choose another playback mode"), http.StatusBadRequest)
			return
		}

		videoStreams, err := app.Queries.GetVideoStreamsByMovieID(r.Context(), req.MovieID)
		if err != nil {
			app.Logger.Error("failed to load video streams for watch room", "error", err, "movie_id", req.MovieID)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
			return
		}
		// Zero streams means the scan found no playable video, not "unknown" —
		// the web client refuses direct play for the same shape (audit D17).
		primaryVideo := primaryVideoStream(videoStreams)
		if primaryVideo == nil {
			helpers.ErrorJSON(w, errors.New("this movie has no playable video stream; direct playback is unavailable"), http.StatusBadRequest)
			return
		}

		browserSafe, _ := isBrowserSafeH264RemuxCandidate(primaryVideo)
		if !browserSafe {
			helpers.ErrorJSON(w, errors.New("this movie's video cannot be played directly by browsers; choose another playback mode"), http.StatusBadRequest)
			return
		}
	}

	invitedIDs := deduplicateAndFilterUserIDs(req.InvitedUserIDs, userID)

	if len(invitedIDs) > 0 {
		count, err := app.Queries.CountUsersByIDs(r.Context(), invitedIDs)
		if err != nil {
			app.Logger.Error("failed to validate invited user IDs", "error", err)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
			return
		}
		if count != int64(len(invitedIDs)) {
			helpers.ErrorJSON(w, errors.New("one or more invited users do not exist"), http.StatusBadRequest)
			return
		}
	}

	var subtitleTrack sql.NullInt64
	if req.SubtitleTrack != nil {
		subtitleTrack = sql.NullInt64{Int64: *req.SubtitleTrack, Valid: true}
	}

	// Pin the selected tracks' identity (absolute stream index + language) so
	// a rescan of a replaced file is detected at playback time instead of
	// silently playing a different track (audit H14).
	var audioStreamIndex sql.NullInt64
	var audioLanguage sql.NullString
	if movieHasAudio {
		selectedAudio := audioStreams[req.AudioTrack]
		audioStreamIndex = sql.NullInt64{Int64: selectedAudio.StreamIndex, Valid: true}
		audioLanguage = selectedAudio.Language
	}

	var subtitleStreamIndex sql.NullInt64
	var subtitleLanguage sql.NullString
	if req.SubtitleTrack != nil {
		selectedSubtitle := subtitleStreams[*req.SubtitleTrack]
		subtitleStreamIndex = sql.NullInt64{Int64: selectedSubtitle.StreamIndex, Valid: true}
		subtitleLanguage = selectedSubtitle.Language
	}

	tx, err := app.DB.BeginTx(r.Context(), nil)
	if err != nil {
		app.Logger.Error("failed to begin transaction for watch room creation", "error", err)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	qtx := app.Queries.WithTx(tx)

	room, err := qtx.CreateWatchRoom(r.Context(), database.CreateWatchRoomParams{
		OwnerUserID:         userID,
		MovieID:             req.MovieID,
		PlaybackMode:        req.Mode,
		AudioTrack:          req.AudioTrack,
		SubtitleTrack:       subtitleTrack,
		AudioStreamIndex:    audioStreamIndex,
		AudioLanguage:       audioLanguage,
		SubtitleStreamIndex: subtitleStreamIndex,
		SubtitleLanguage:    subtitleLanguage,
	})
	if err != nil {
		_ = tx.Rollback()
		app.Logger.Error("failed to create watch room", "error", err)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	err = qtx.AddWatchRoomMember(r.Context(), database.AddWatchRoomMemberParams{
		RoomID: room.ID,
		UserID: userID,
	})
	if err != nil {
		_ = tx.Rollback()
		app.Logger.Error("failed to add owner as room member", "error", err, "room_id", room.ID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	for _, invitedID := range invitedIDs {
		err = qtx.AddWatchRoomMember(r.Context(), database.AddWatchRoomMemberParams{
			RoomID: room.ID,
			UserID: invitedID,
		})
		if err != nil {
			_ = tx.Rollback()
			app.Logger.Error("failed to add invited user as room member", "error", err, "room_id", room.ID, "user_id", invitedID)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
			return
		}
	}

	if err = tx.Commit(); err != nil {
		app.Logger.Error("failed to commit watch room transaction", "error", err)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	// HLS rooms warm up immediately; failures roll back the room.
	if req.Mode != watchRoomPlaybackModeDirect {
		warmErr := app.WarmUpRoomHLSSession(background, room.ID, req.MovieID, req.Mode, int(req.AudioTrack), &movie, audioStreams)
		if warmErr != nil {
			deleteErr := app.Queries.DeleteWatchRoom(background, room.ID)
			if deleteErr != nil {
				app.Logger.Error(
					"hls warm-up failed and watch room rollback delete failed",
					"warm_up_error", warmErr,
					"delete_error", deleteErr,
					"room_id", room.ID,
				)
				helpers.ErrorJSON(w, errors.New("failed to start playback session for this room and failed to roll back the room"), http.StatusInternalServerError)
				return
			}
			app.Logger.Error("hls warm-up failed; rolled back watch room", "warm_up_error", warmErr, "room_id", room.ID)
			helpers.ErrorJSON(w, errors.New("failed to start playback session for this room"), http.StatusInternalServerError)
			return
		}
	}

	helpers.WriteJSON(w, http.StatusCreated, helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"room_id": room.ID},
	})
}

func (app *Application) JoinWatchRoom(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	roomID, err := parseRoomID(r)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	_, err = app.Queries.GetWatchRoomByID(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("room not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to fetch watch room for join", "error", err, "room_id", roomID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	isMember, err := app.Queries.IsWatchRoomMember(r.Context(), database.IsWatchRoomMemberParams{
		RoomID: roomID,
		UserID: userID,
	})
	if err != nil {
		app.Logger.Error("failed to check room membership for join", "error", err, "room_id", roomID, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}
	if !isMember {
		helpers.ErrorJSON(w, errors.New("access denied"), http.StatusForbidden)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"room_id": roomID, "joined": true},
	})
}

func (app *Application) DeleteWatchRoom(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	roomID, err := parseRoomID(r)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	room, err := app.Queries.GetWatchRoomByID(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("room not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to fetch watch room for delete", "error", err, "room_id", roomID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	// The room row already carries the owner; no second query needed.
	if room.OwnerUserID != userID {
		helpers.ErrorJSON(w, errors.New("only the room owner can delete this room"), http.StatusForbidden)
		return
	}

	err = app.Queries.DeleteWatchRoom(r.Context(), roomID)
	if err != nil {
		app.Logger.Error("failed to delete watch room", "error", err, "room_id", roomID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	// No-op for direct-play rooms or rooms that never warmed up.
	app.CleanupRoomHLSSession(roomID)
	app.WatchRoomHub.deleteRoom(roomID)
	app.forgetWatchRoom(roomID)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"deleted": true},
	})
}
