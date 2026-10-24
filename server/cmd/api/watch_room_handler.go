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

const watchRoomPlaybackModeDirect = "direct"

type watchRoomListItem struct {
	ID           int64         `json:"id"`
	MovieID      int64         `json:"movie_id"`
	MovieTitle   string        `json:"movie_title"`
	MoviePoster  *string       `json:"movie_poster"`
	Owner        userSummary   `json:"owner"`
	Members      []userSummary `json:"members"`
	PlaybackMode string        `json:"playback_mode"`
	IsOwner      bool          `json:"is_owner"`
	CreatedAt    string        `json:"created_at"`
}

type watchRoomDetail struct {
	ID            int64         `json:"id"`
	MovieID       int64         `json:"movie_id"`
	MovieTitle    string        `json:"movie_title"`
	MoviePoster   *string       `json:"movie_poster"`
	Owner         userSummary   `json:"owner"`
	Members       []userSummary `json:"members"`
	PlaybackMode  string        `json:"playback_mode"`
	AudioTrack    int64         `json:"audio_track"`
	SubtitleTrack *int64        `json:"subtitle_track"`
	IsOwner       bool          `json:"is_owner"`
	CreatedAt     string        `json:"created_at"`
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

func (app *Application) loadRoomMembers(ctx context.Context, roomID int64) ([]userSummary, error) {
	rows, err := app.Queries.GetWatchRoomMembers(ctx, roomID)
	if err != nil {
		return nil, err
	}
	members := make([]userSummary, len(rows))
	for i, row := range rows {
		members[i] = userSummary{ID: row.ID, Name: row.Name, Avatar: helpers.StringPtrFromNull(row.Avatar)}
	}
	return members, nil
}

func findMemberByID(members []userSummary, id int64) (userSummary, bool) {
	for _, m := range members {
		if m.ID == id {
			return m, true
		}
	}
	return userSummary{}, false
}

func (app *Application) GetWatchRooms(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	rooms, err := app.Queries.GetWatchRoomsForUser(r.Context(), userID)
	if err != nil {
		app.serverError(w, err, "failed to fetch watch rooms", internalServerErrorMessage, "user_id", userID)
		return
	}

	roomIDs := make([]int64, 0, len(rooms))
	for _, room := range rooms {
		roomIDs = append(roomIDs, room.ID)
	}

	memberRows, err := app.Queries.GetWatchRoomMembersByRoomIDs(r.Context(), roomIDs)
	if err != nil {
		app.serverError(w, err, "failed to fetch members for watch rooms", internalServerErrorMessage, "user_id", userID)
		return
	}

	membersByRoomID := make(map[int64][]userSummary, len(roomIDs))
	for _, row := range memberRows {
		membersByRoomID[row.RoomID] = append(membersByRoomID[row.RoomID], userSummary{
			ID:     row.ID,
			Name:   row.Name,
			Avatar: helpers.StringPtrFromNull(row.Avatar),
		})
	}

	items := make([]watchRoomListItem, 0, len(rooms))
	for _, room := range rooms {
		members := membersByRoomID[room.ID]
		owner, _ := findMemberByID(members, room.OwnerUserID)

		items = append(items, watchRoomListItem{
			ID:           room.ID,
			MovieID:      room.MovieID,
			MovieTitle:   room.MovieTitle,
			MoviePoster:  helpers.StringPtrFromNull(room.MoviePosterPath),
			Owner:        owner,
			Members:      members,
			PlaybackMode: room.PlaybackMode,
			IsOwner:      room.OwnerUserID == userID,
			CreatedAt:    room.CreatedAt,
		})
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: map[string]any{"rooms": items}})
}

// One joined query authorizes the member and returns the room, the same way
// the media handlers and the WebSocket upgrade do. An unknown room and a room
// the caller is not a member of both come back as 403, as they do on those
// routes: the room id is guessable. Join and delete answer 404 for an unknown
// room instead, as their contract states.
func (app *Application) GetWatchRoom(w http.ResponseWriter, r *http.Request) {
	room, userID, ok := app.loadAuthorizedWatchRoomForRequest(w, r)
	if !ok {
		return
	}

	roomID := room.ID

	movie, err := app.Queries.GetMovieByID(r.Context(), room.MovieID)
	if err != nil {
		app.serverError(w, err, "failed to fetch movie for watch room", internalServerErrorMessage,
			"room_id", roomID, "movie_id", room.MovieID)
		return
	}

	members, err := app.loadRoomMembers(r.Context(), roomID)
	if err != nil {
		app.serverError(w, err, "failed to fetch members for watch room", internalServerErrorMessage, "room_id", roomID)
		return
	}

	owner, _ := findMemberByID(members, room.OwnerUserID)

	var subtitleTrack *int64
	if room.SubtitleTrack.Valid {
		subtitleTrack = &room.SubtitleTrack.Int64
	}

	detail := watchRoomDetail{
		ID:            room.ID,
		MovieID:       room.MovieID,
		MovieTitle:    movie.Title,
		MoviePoster:   helpers.StringPtrFromNull(movie.PosterPath),
		Owner:         owner,
		Members:       members,
		PlaybackMode:  room.PlaybackMode,
		AudioTrack:    room.AudioTrack,
		SubtitleTrack: subtitleTrack,
		IsOwner:       room.OwnerUserID == userID,
		CreatedAt:     room.CreatedAt,
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: map[string]any{"room": detail}})
}

func (app *Application) CreateWatchRoom(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	var req createWatchRoomRequest
	if err := helpers.ReadJSON(w, r, &req); err != nil {
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
			helpers.ErrorJSON(w, errors.New(movieNotFoundMessage), http.StatusBadRequest)
			return
		}
		app.serverError(w, err, "failed to verify movie for watch room", internalServerErrorMessage, "movie_id", req.MovieID)
		return
	}

	// audio_track is an ordinal into the movie's audio streams, so it has to be
	// validated against the movie. Without this the room is created and members
	// invited before playback fails on the first manifest request.
	audioStreams, err := app.Queries.GetAudioStreamsByMovieID(r.Context(), req.MovieID)
	if err != nil {
		app.serverError(w, err, "failed to load audio streams for watch room", internalServerErrorMessage,
			"movie_id", req.MovieID)
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
			app.serverError(w, err, "failed to load subtitles for watch room", internalServerErrorMessage,
				"movie_id", req.MovieID)
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
	// audio track no matter what the room stores. The remaining checks mirror
	// the web client's static direct-play rules: browsers only direct-play MP4,
	// and 10-bit / 4:2:2 / 4:4:4 H.264 does not decode even though the codec
	// name passes.
	if req.Mode == watchRoomPlaybackModeDirect {
		if req.AudioTrack != 0 {
			helpers.ErrorJSON(w, errors.New("direct playback always uses the first audio track; choose another playback mode to pick a different one"), http.StatusBadRequest)
			return
		}

		if !directPlayAudioSelectionUnambiguous(audioStreams) {
			helpers.ErrorJSON(w, errors.New("direct playback cannot guarantee which audio track this movie plays; choose another playback mode"), http.StatusBadRequest)
			return
		}

		if videoContentType(movie.Container, movie.MimeType) != mp4ContentType {
			helpers.ErrorJSON(w, errors.New("direct playback is only available for MP4 movies; choose another playback mode"), http.StatusBadRequest)
			return
		}

		videoStreams, err := app.Queries.GetVideoStreamsByMovieID(r.Context(), req.MovieID)
		if err != nil {
			app.serverError(w, err, "failed to load video streams for watch room", internalServerErrorMessage,
				"movie_id", req.MovieID)
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
			app.serverError(w, err, "failed to validate invited user IDs", internalServerErrorMessage)
			return
		}
		if count != int64(len(invitedIDs)) {
			helpers.ErrorJSON(w, errors.New("one or more invited users do not exist"), http.StatusBadRequest)
			return
		}
	}

	// Track ordinals and stream indices both start at 0, so every sql.NullInt64
	// below keeps an explicit Valid: true. helpers.NullInt64 maps 0 to NULL and
	// would record the first track as "no track at all".
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
		app.serverError(w, err, "failed to begin transaction for watch room creation", internalServerErrorMessage)
		return
	}

	defer func() { _ = tx.Rollback() }()

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
		app.serverError(w, err, "failed to create watch room", internalServerErrorMessage)
		return
	}

	// The owner is a member too.
	for _, memberID := range append([]int64{userID}, invitedIDs...) {
		err = qtx.AddWatchRoomMember(r.Context(), database.AddWatchRoomMemberParams{
			RoomID: room.ID,
			UserID: memberID,
		})
		if err != nil {
			app.serverError(w, err, "failed to add watch room member", internalServerErrorMessage,
				"room_id", room.ID, "user_id", memberID)
			return
		}
	}

	if err = tx.Commit(); err != nil {
		app.serverError(w, err, "failed to commit watch room transaction", internalServerErrorMessage)
		return
	}

	// HLS rooms warm up immediately; failures roll back the room.
	if req.Mode != watchRoomPlaybackModeDirect {
		_, warmErr := app.GetOrCreateRoomHLSSession(context.Background(), room.ID, req.MovieID, req.Mode, int(req.AudioTrack), &movie, audioStreams)
		if warmErr != nil {
			deleteErr := app.deleteWatchRoom(context.Background(), room.ID)
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
			// A room start never parks for a permit, so a full pool refuses it
			// at once; that is a reason to try again shortly, not a failure.
			atCapacity := isHLSCapacityError(warmErr)
			if atCapacity {
				app.Logger.Warn("hls warm-up refused for capacity; rolled back watch room", "warm_up_error", warmErr, "room_id", room.ID)
				w.Header().Set("Retry-After", strconv.Itoa(hlsTranscodeBusyRetryAfterSec))
				helpers.ErrorJSON(w, errors.New("the server has no room to start this stream right now; try again in a moment"), http.StatusServiceUnavailable)
				return
			}
			app.Logger.Error("hls warm-up failed; rolled back watch room", "warm_up_error", warmErr, "room_id", room.ID)
			helpers.ErrorJSON(w, errors.New("failed to start playback session for this room"), http.StatusInternalServerError)
			return
		}
	}

	helpers.WriteJSON(w, http.StatusCreated, helpers.JSONResponse{Data: map[string]any{"room_id": room.ID}})
}

func (app *Application) JoinWatchRoom(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

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
		app.serverError(w, err, "failed to fetch watch room for join", internalServerErrorMessage, "room_id", roomID)
		return
	}

	isMember, err := app.Queries.IsWatchRoomMember(r.Context(), database.IsWatchRoomMemberParams{
		RoomID: roomID,
		UserID: userID,
	})
	if err != nil {
		app.serverError(w, err, "failed to check room membership for join", internalServerErrorMessage,
			"room_id", roomID, "user_id", userID)
		return
	}
	if !isMember {
		helpers.ErrorJSON(w, errors.New(accessDeniedMessage), http.StatusForbidden)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: map[string]any{"room_id": roomID, "joined": true}})
}

func (app *Application) DeleteWatchRoom(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	roomID, err := parseRoomID(r)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	ownerUserID, err := app.Queries.GetWatchRoomByID(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("room not found"), http.StatusNotFound)
			return
		}
		app.serverError(w, err, "failed to fetch watch room for delete", internalServerErrorMessage, "room_id", roomID)
		return
	}

	// The room row already carries the owner; no second query needed.
	if ownerUserID != userID {
		helpers.ErrorJSON(w, errors.New("only the room owner can delete this room"), http.StatusForbidden)
		return
	}

	err = app.deleteWatchRoom(r.Context(), roomID)
	if err != nil {
		app.serverError(w, err, "failed to delete watch room", internalServerErrorMessage, "room_id", roomID)
		return
	}

	// No-op for direct-play rooms or rooms that never warmed up.
	app.CleanupRoomHLSSession(roomID)
	app.WatchRoomHub.deleteRoom(roomID)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: map[string]any{"deleted": true}})
}
