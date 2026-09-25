package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"igloo/cmd/internal/database"
)

// The music list operations are otherwise only validated against an empty
// library, where the item schema is never exercised. These run with a seeded
// track, album, musician and playlist so the array elements are validated
// against the contract. The movie twin of this file is
// movie_list_contract_test.go.
func TestMusicListHandlers_ConformToOpenAPIWithRows(t *testing.T) {
	app := setupSessionTestApp(t)

	user := createTestUser(t, app, "List User", "music-list-contract@example.com", false)
	collaborator := createTestUser(t, app, "List Collaborator", "music-list-collaborator@example.com", false)

	musicianID := createSearchMusician(t, app, "Contract List Artist")
	albumID := createSearchAlbum(t, app, "Contract List Album", "Contract List Artist")
	trackID := createSearchTrack(t, app, "Contract List Track", "/music/contract-list.flac", albumID, musicianID)

	err := app.Queries.LikeTrack(t.Context(), database.LikeTrackParams{
		UserID:  user.ID,
		TrackID: trackID,
	})
	if err != nil {
		t.Fatalf("like track: %v", err)
	}

	playlistID := createTrackPlaylistWithTrack(t, app, user.ID, trackID)
	_, err = app.Queries.AddCollaborator(t.Context(), database.AddCollaboratorParams{
		PlaylistID: playlistID,
		UserID:     collaborator.ID,
		CanEdit:    true,
	})
	if err != nil {
		t.Fatalf("add playlist collaborator: %v", err)
	}

	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	playlistIDString := strconv.FormatInt(playlistID, 10)

	operations := []struct {
		operationID string
		path        string
		dataKey     string
	}{
		{operationID: "getTracksAlphabetical", path: "/api/music/tracks", dataKey: "tracks"},
		{operationID: "getShuffleTracks", path: "/api/music/tracks/shuffle", dataKey: "tracks"},
		{operationID: "getLikedTracks", path: "/api/music/tracks/liked", dataKey: "tracks"},
		{operationID: "getLikedTrackIDsForUser", path: "/api/music/tracks/liked-ids", dataKey: "liked_track_ids"},
		{operationID: "getAlbumsAlphabetical", path: "/api/music/albums", dataKey: "albums"},
		{operationID: "getLatestAlbums", path: "/api/music/albums/latest", dataKey: "albums"},
		{operationID: "getMusiciansAlphabetical", path: "/api/music/musicians", dataKey: "musicians"},
		{operationID: "getPlaylists", path: "/api/music/playlists", dataKey: "playlists"},
		{operationID: "getPlaylistTracks", path: "/api/music/playlists/" + playlistIDString + "/tracks", dataKey: "tracks"},
		{operationID: "getPlaylistCollaborators", path: "/api/music/playlists/" + playlistIDString + "/collaborators", dataKey: "collaborators"},
	}

	for _, operation := range operations {
		t.Run(operation.operationID, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, operation.path, nil)
			request.AddCookie(cookie)
			response := httptest.NewRecorder()
			app.Router.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("%s status = %d, want %d, body = %s", operation.operationID, response.Code, http.StatusOK, response.Body.String())
			}

			assertResponseListNotEmpty(t, operation.operationID, response.Body.Bytes(), operation.dataKey)
			assertOpenAPIExchange(t, operation.operationID, request, response)
		})
	}
}

func createTrackPlaylistWithTrack(t *testing.T, app *Application, userID, trackID int64) int64 {
	t.Helper()

	playlist, err := app.Queries.CreatePlaylist(t.Context(), database.CreatePlaylistParams{
		UserID:   userID,
		Name:     "Contract List Playlist",
		IsPublic: false,
	})
	if err != nil {
		t.Fatalf("create track playlist: %v", err)
	}

	_, err = app.Queries.AddTrackToPlaylist(t.Context(), database.AddTrackToPlaylistParams{
		PlaylistID: playlist.ID,
		TrackID:    trackID,
		AddedBy:    sql.NullInt64{Int64: userID, Valid: true},
	})
	if err != nil {
		t.Fatalf("add track to playlist: %v", err)
	}

	return playlist.ID
}
