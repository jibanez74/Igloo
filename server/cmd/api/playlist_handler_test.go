package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

func TestValidatePlaylistMetadataCountsUnicodeCodePoints(t *testing.T) {
	tests := []struct {
		name         string
		playlistName string
		description  string
		wantError    string
	}{
		{
			name:      "empty name",
			wantError: "playlist name is required",
		},
		{
			name:         "ASCII name at limit",
			playlistName: strings.Repeat("a", playlistNameMaxLength),
		},
		{
			name:         "ASCII name over limit",
			playlistName: strings.Repeat("a", playlistNameMaxLength+1),
			wantError:    "playlist name is too long (max 255 characters)",
		},
		{
			name:         "multibyte name at limit",
			playlistName: strings.Repeat("😀", playlistNameMaxLength),
		},
		{
			name:         "multibyte name over limit",
			playlistName: strings.Repeat("😀", playlistNameMaxLength+1),
			wantError:    "playlist name is too long (max 255 characters)",
		},
		{
			name:         "ASCII description at limit",
			playlistName: "Playlist",
			description:  strings.Repeat("a", playlistDescriptionMaxLength),
		},
		{
			name:         "ASCII description over limit",
			playlistName: "Playlist",
			description:  strings.Repeat("a", playlistDescriptionMaxLength+1),
			wantError:    "description is too long (max 1000 characters)",
		},
		{
			name:         "multibyte description at limit",
			playlistName: "Playlist",
			description:  strings.Repeat("界", playlistDescriptionMaxLength),
		},
		{
			name:         "multibyte description over limit",
			playlistName: "Playlist",
			description:  strings.Repeat("界", playlistDescriptionMaxLength+1),
			wantError:    "description is too long (max 1000 characters)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePlaylistMetadata(tt.playlistName, tt.description)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("validatePlaylistMetadata returned error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("validatePlaylistMetadata returned nil error, want %q", tt.wantError)
			}
			if err.Error() != tt.wantError {
				t.Fatalf("validatePlaylistMetadata error = %q, want %q", err.Error(), tt.wantError)
			}
		})
	}
}

func TestPlaylistMutationHandlersCountUnicodeCodePoints(t *testing.T) {
	app := setupSessionTestApp(t)

	user := createTestUser(t, app, "Playlist Tester", "playlist-tester@example.com", false)

	trackPlaylist, err := app.Queries.CreatePlaylist(context.Background(), database.CreatePlaylistParams{
		UserID:      user.ID,
		Name:        "Track Playlist",
		Description: sql.NullString{},
		CoverImage:  sql.NullString{},
		IsPublic:    false,
	})
	if err != nil {
		t.Fatalf("create track playlist: %v", err)
	}

	moviePlaylist, err := app.Queries.CreateMoviePlaylist(context.Background(), database.CreateMoviePlaylistParams{
		UserID:      user.ID,
		Name:        "Movie Playlist",
		Description: sql.NullString{},
		CoverImage:  sql.NullString{},
		IsPublic:    false,
		MovieID:     sql.NullInt64{},
	})
	if err != nil {
		t.Fatalf("create movie playlist: %v", err)
	}

	handler := authenticatedRouter(t, app, user.ID)

	validUnicodeName := strings.Repeat("😀", playlistNameMaxLength)
	overLimitUnicodeName := strings.Repeat("😀", playlistNameMaxLength+1)
	tests := []struct {
		name       string
		method     string
		path       string
		value      string
		wantStatus int
	}{
		{
			name:       "create music playlist accepts Unicode at limit",
			method:     http.MethodPost,
			path:       "/api/music/playlists/",
			value:      validUnicodeName,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "create music playlist rejects Unicode over limit",
			method:     http.MethodPost,
			path:       "/api/music/playlists/",
			value:      overLimitUnicodeName,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "update music playlist accepts Unicode at limit",
			method:     http.MethodPut,
			path:       "/api/music/playlists/" + strconv.FormatInt(trackPlaylist.ID, 10),
			value:      validUnicodeName,
			wantStatus: http.StatusOK,
		},
		{
			name:       "update music playlist rejects Unicode over limit",
			method:     http.MethodPut,
			path:       "/api/music/playlists/" + strconv.FormatInt(trackPlaylist.ID, 10),
			value:      overLimitUnicodeName,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "create movie playlist accepts Unicode at limit",
			method:     http.MethodPost,
			path:       "/api/movies/playlists/",
			value:      validUnicodeName,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "create movie playlist rejects Unicode over limit",
			method:     http.MethodPost,
			path:       "/api/movies/playlists/",
			value:      overLimitUnicodeName,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "update movie playlist accepts Unicode at limit",
			method:     http.MethodPut,
			path:       "/api/movies/playlists/" + strconv.FormatInt(moviePlaylist.ID, 10),
			value:      validUnicodeName,
			wantStatus: http.StatusOK,
		},
		{
			name:       "update movie playlist rejects Unicode over limit",
			method:     http.MethodPut,
			path:       "/api/movies/playlists/" + strconv.FormatInt(moviePlaylist.ID, 10),
			value:      overLimitUnicodeName,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]string{
				"name":        tt.value,
				"description": "Description",
			})
			if err != nil {
				t.Fatalf("marshal request body: %v", err)
			}

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

type playlistFixtures struct {
	owner         database.User
	editor        database.User
	viewer        database.User
	outsider      database.User
	trackPlaylist database.Playlist
	moviePlaylist database.Playlist
	trackID       int64
	movieID       int64
}

func createPlaylistFixtures(t *testing.T, app *Application) playlistFixtures {
	t.Helper()

	owner := createTestUser(t, app, "Playlist Owner", "playlist-owner@example.com", false)
	editor := createTestUser(t, app, "Playlist Editor", "playlist-editor@example.com", false)
	viewer := createTestUser(t, app, "Playlist Viewer", "playlist-viewer@example.com", false)
	outsider := createTestUser(t, app, "Playlist Outsider", "playlist-outsider@example.com", false)

	trackPlaylist, err := app.Queries.CreatePlaylist(context.Background(), database.CreatePlaylistParams{
		UserID:      owner.ID,
		Name:        "Track Playlist",
		Description: sql.NullString{},
		CoverImage:  sql.NullString{},
		IsPublic:    false,
	})
	if err != nil {
		t.Fatalf("create track playlist: %v", err)
	}
	moviePlaylist, err := app.Queries.CreateMoviePlaylist(context.Background(), database.CreateMoviePlaylistParams{
		UserID:      owner.ID,
		Name:        "Movie Playlist",
		Description: sql.NullString{},
		CoverImage:  sql.NullString{},
		IsPublic:    false,
		MovieID:     sql.NullInt64{},
	})
	if err != nil {
		t.Fatalf("create movie playlist: %v", err)
	}

	for _, playlistID := range []int64{trackPlaylist.ID, moviePlaylist.ID} {
		_, err = app.Queries.AddCollaborator(context.Background(), database.AddCollaboratorParams{
			PlaylistID: playlistID,
			UserID:     editor.ID,
			CanEdit:    true,
		})
		if err != nil {
			t.Fatalf("add editor to playlist %d: %v", playlistID, err)
		}
		_, err = app.Queries.AddCollaborator(context.Background(), database.AddCollaboratorParams{
			PlaylistID: playlistID,
			UserID:     viewer.ID,
			CanEdit:    false,
		})
		if err != nil {
			t.Fatalf("add viewer to playlist %d: %v", playlistID, err)
		}
	}

	musicianID := createTestMusician(t, app, "Playlist Artist")
	albumID := createTestAlbum(t, app, "Playlist Album", "Playlist Artist")
	trackID := createTestTrack(t, app, "Playlist Track", "/music/playlist-track.flac", albumID, musicianID)
	movieID := createTestMovie(t, app, "Playlist Movie", "/movies/playlist-movie.mkv")

	return playlistFixtures{
		owner:         owner,
		editor:        editor,
		viewer:        viewer,
		outsider:      outsider,
		trackPlaylist: trackPlaylist,
		moviePlaylist: moviePlaylist,
		trackID:       trackID,
		movieID:       movieID,
	}
}

func TestPlaylistAccessAndContentTypes(t *testing.T) {
	app := setupTestApp(t)
	fixtures := createPlaylistFixtures(t, app)

	type detailEnvelope struct {
		Data struct {
			IsOwner bool `json:"is_owner"`
			CanEdit bool `json:"can_edit"`
		} `json:"data"`
	}

	tests := []struct {
		name      string
		userID    int64
		path      string
		wantCode  int
		wantOwner bool
		wantEdit  bool
	}{
		{name: "owner has full track access", userID: fixtures.owner.ID, path: "/api/music/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10), wantCode: http.StatusOK, wantOwner: true, wantEdit: true},
		{name: "editor can edit track playlist", userID: fixtures.editor.ID, path: "/api/music/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10), wantCode: http.StatusOK, wantEdit: true},
		{name: "viewer can view track playlist", userID: fixtures.viewer.ID, path: "/api/music/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10), wantCode: http.StatusOK},
		{name: "outsider cannot view private track playlist", userID: fixtures.outsider.ID, path: "/api/music/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10), wantCode: http.StatusForbidden},
		{name: "track endpoint rejects movie playlist", userID: fixtures.owner.ID, path: "/api/music/playlists/" + strconv.FormatInt(fixtures.moviePlaylist.ID, 10), wantCode: http.StatusBadRequest},
		{name: "movie endpoint rejects track playlist", userID: fixtures.owner.ID, path: "/api/movies/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10), wantCode: http.StatusBadRequest},
		{name: "unauthenticated request is rejected", path: "/api/music/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10), wantCode: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := serveAs(t, app, tt.userID, http.MethodGet, tt.path, "")
			if w.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.wantCode, w.Body.String())
			}
			if tt.wantCode != http.StatusOK {
				return
			}

			var response detailEnvelope
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err != nil {
				t.Fatalf("decode detail response: %v", err)
			}
			if response.Data.IsOwner != tt.wantOwner || response.Data.CanEdit != tt.wantEdit {
				t.Fatalf("access = owner:%v edit:%v, want owner:%v edit:%v", response.Data.IsOwner, response.Data.CanEdit, tt.wantOwner, tt.wantEdit)
			}
		})
	}
}

func TestPlaylistListsExposeOwnerAndEditorAccess(t *testing.T) {
	app := setupTestApp(t)
	fixtures := createPlaylistFixtures(t, app)

	type playlistAccess struct {
		ID      int64 `json:"id"`
		IsOwner bool  `json:"is_owner"`
		CanEdit bool  `json:"can_edit"`
	}
	type listEnvelope struct {
		Data struct {
			Playlists []playlistAccess `json:"playlists"`
		} `json:"data"`
	}

	tests := []struct {
		name    string
		userID  int64
		path    string
		wantID  int64
		isOwner bool
		canEdit bool
	}{
		{name: "track owner", userID: fixtures.owner.ID, path: "/api/music/playlists/", wantID: fixtures.trackPlaylist.ID, isOwner: true, canEdit: true},
		{name: "track editor", userID: fixtures.editor.ID, path: "/api/music/playlists/", wantID: fixtures.trackPlaylist.ID, canEdit: true},
		{name: "track viewer", userID: fixtures.viewer.ID, path: "/api/music/playlists/", wantID: fixtures.trackPlaylist.ID},
		{name: "movie owner", userID: fixtures.owner.ID, path: "/api/movies/playlists/", wantID: fixtures.moviePlaylist.ID, isOwner: true, canEdit: true},
		{name: "movie editor", userID: fixtures.editor.ID, path: "/api/movies/playlists/", wantID: fixtures.moviePlaylist.ID, canEdit: true},
		{name: "movie viewer", userID: fixtures.viewer.ID, path: "/api/movies/playlists/", wantID: fixtures.moviePlaylist.ID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := serveAs(t, app, tt.userID, http.MethodGet, tt.path, "")
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
			}

			var response listEnvelope
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err != nil {
				t.Fatalf("decode list response: %v", err)
			}
			if len(response.Data.Playlists) != 1 {
				t.Fatalf("playlist count = %d, want 1", len(response.Data.Playlists))
			}
			playlist := response.Data.Playlists[0]
			if playlist.ID != tt.wantID || playlist.IsOwner != tt.isOwner || playlist.CanEdit != tt.canEdit {
				t.Fatalf("playlist access = %#v, want id=%d owner=%v edit=%v", playlist, tt.wantID, tt.isOwner, tt.canEdit)
			}
		})
	}
}

func TestPlaylistEditorsCanMutateContentButViewersCannot(t *testing.T) {
	app := setupTestApp(t)
	fixtures := createPlaylistFixtures(t, app)

	trackPath := "/api/music/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10) + "/tracks"
	moviePath := "/api/movies/playlists/" + strconv.FormatInt(fixtures.moviePlaylist.ID, 10) + "/movies"

	w := serveAs(t, app, fixtures.viewer.ID, http.MethodPost, trackPath, `{"track_ids":[`+strconv.FormatInt(fixtures.trackID, 10)+`]}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer add track status = %d, want 403: %s", w.Code, w.Body.String())
	}
	w = serveAs(t, app, fixtures.editor.ID, http.MethodPost, trackPath, `{"track_ids":[`+strconv.FormatInt(fixtures.trackID, 10)+`]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("editor add track status = %d, want 200: %s", w.Code, w.Body.String())
	}
	w = serveAs(t, app, fixtures.editor.ID, http.MethodDelete, trackPath+"/"+strconv.FormatInt(fixtures.trackID, 10), "")
	if w.Code != http.StatusOK {
		t.Fatalf("editor remove track status = %d, want 200: %s", w.Code, w.Body.String())
	}

	w = serveAs(t, app, fixtures.viewer.ID, http.MethodPost, moviePath, `{"movie_ids":[`+strconv.FormatInt(fixtures.movieID, 10)+`]}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer add movie status = %d, want 403: %s", w.Code, w.Body.String())
	}
	w = serveAs(t, app, fixtures.editor.ID, http.MethodPost, moviePath, `{"movie_ids":[`+strconv.FormatInt(fixtures.movieID, 10)+`]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("editor add movie status = %d, want 200: %s", w.Code, w.Body.String())
	}
	w = serveAs(t, app, fixtures.editor.ID, http.MethodDelete, moviePath+"/"+strconv.FormatInt(fixtures.movieID, 10), "")
	if w.Code != http.StatusOK {
		t.Fatalf("editor remove movie status = %d, want 200: %s", w.Code, w.Body.String())
	}
}

func TestMoviePlaylistCollaboratorManagement(t *testing.T) {
	app := setupTestApp(t)
	fixtures := createPlaylistFixtures(t, app)

	moviePlaylistID := strconv.FormatInt(fixtures.moviePlaylist.ID, 10)
	collaboratorsPath := "/api/movies/playlists/" + moviePlaylistID + "/collaborators"
	outsiderID := strconv.FormatInt(fixtures.outsider.ID, 10)

	w := serveAs(
		t,
		app,
		fixtures.owner.ID,
		http.MethodPost,
		collaboratorsPath,
		`{"user_id":`+outsiderID+`,"can_edit":true}`,
	)
	if w.Code != http.StatusCreated {
		t.Fatalf("owner add collaborator status = %d, want 201: %s", w.Code, w.Body.String())
	}

	w = serveAs(t, app, fixtures.owner.ID, http.MethodGet, collaboratorsPath, "")
	if w.Code != http.StatusOK {
		t.Fatalf("owner list collaborators status = %d, want 200: %s", w.Code, w.Body.String())
	}

	var listResponse struct {
		Data struct {
			Collaborators []database.GetPlaylistCollaboratorsRow `json:"collaborators"`
		} `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &listResponse)
	if err != nil {
		t.Fatalf("decode collaborators response: %v", err)
	}

	foundOutsider := false
	for _, collaborator := range listResponse.Data.Collaborators {
		if collaborator.UserID == fixtures.outsider.ID {
			foundOutsider = true
			if !collaborator.CanEdit {
				t.Fatal("new movie playlist collaborator should have edit access")
			}
		}
	}
	if !foundOutsider {
		t.Fatalf("owner collaborator list does not include user %d", fixtures.outsider.ID)
	}

	moviesPath := "/api/movies/playlists/" + moviePlaylistID + "/movies"
	w = serveAs(
		t,
		app,
		fixtures.outsider.ID,
		http.MethodPost,
		moviesPath,
		`{"movie_ids":[`+strconv.FormatInt(fixtures.movieID, 10)+`]}`,
	)
	if w.Code != http.StatusOK {
		t.Fatalf("editor add movie status = %d, want 200: %s", w.Code, w.Body.String())
	}

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		path := collaboratorsPath
		body := ""
		if method == http.MethodPost {
			body = `{"user_id":` + strconv.FormatInt(fixtures.outsider.ID, 10) + `,"can_edit":false}`
		}
		if method == http.MethodDelete {
			path += "/" + outsiderID
		}

		w = serveAs(t, app, fixtures.viewer.ID, method, path, body)
		if w.Code != http.StatusForbidden {
			t.Fatalf("viewer %s collaborators status = %d, want 403: %s", method, w.Code, w.Body.String())
		}
	}

	trackCollaboratorsPath := "/api/movies/playlists/" +
		strconv.FormatInt(fixtures.trackPlaylist.ID, 10) +
		"/collaborators"
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		path := trackCollaboratorsPath
		body := ""
		if method == http.MethodPost {
			body = `{"user_id":` + outsiderID + `,"can_edit":true}`
		}
		if method == http.MethodDelete {
			path += "/" + outsiderID
		}

		w = serveAs(t, app, fixtures.owner.ID, method, path, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("movie route %s track playlist status = %d, want 400: %s", method, w.Code, w.Body.String())
		}
	}

	w = serveAs(
		t,
		app,
		fixtures.owner.ID,
		http.MethodDelete,
		collaboratorsPath+"/"+outsiderID,
		"",
	)
	if w.Code != http.StatusOK {
		t.Fatalf("owner remove collaborator status = %d, want 200: %s", w.Code, w.Body.String())
	}

	w = serveAs(t, app, fixtures.outsider.ID, http.MethodGet, moviesPath, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("removed collaborator movie access status = %d, want 403: %s", w.Code, w.Body.String())
	}
}

func TestPlaylistHandlers_ConformToOpenAPI(t *testing.T) {
	app := setupTestApp(t)
	owner := createTestUser(t, app, "Contract Owner", "contract-owner@example.com", false)
	outsider := createTestUser(t, app, "Contract Outsider", "contract-outsider@example.com", false)
	handler := authenticatedRouter(t, app, owner.ID)

	request := func(operationID, method, path, body string, wantStatus int) *httptest.ResponseRecorder {
		t.Helper()
		var req *http.Request
		if body == "" {
			req = httptest.NewRequest(method, path, nil)
		} else {
			req = newOpenAPIJSONRequest(method, path, body)
		}
		return serveOpenAPIExchange(t, handler, operationID, req, wantStatus)
	}

	request("getPlaylists", http.MethodGet, "/api/music/playlists", "", http.StatusOK)
	request("getMoviePlaylists", http.MethodGet, "/api/movies/playlists", "", http.StatusOK)

	musicianID := createTestMusician(t, app, "Contract Playlist Artist")
	albumID := createTestAlbum(t, app, "Contract Playlist Album", "Contract Playlist Artist")
	trackIDValue := createTestTrack(t, app, "Contract Playlist Track", "/music/playlist-contract.flac", albumID, musicianID)
	movieIDValue := createTestMovie(t, app, "Contract Playlist Movie", "/movies/playlist-contract.mkv")
	trackID := strconv.FormatInt(trackIDValue, 10)
	movieID := strconv.FormatInt(movieIDValue, 10)
	outsiderID := strconv.FormatInt(outsider.ID, 10)

	createdTrackResponse := request("createPlaylist", http.MethodPost, "/api/music/playlists", `{"name":"Created Track Playlist"}`, http.StatusCreated)
	var createdTrack struct {
		Data struct {
			Playlist database.Playlist `json:"playlist"`
		} `json:"data"`
	}
	err := json.Unmarshal(createdTrackResponse.Body.Bytes(), &createdTrack)
	if err != nil {
		t.Fatalf("decode created track playlist: %v", err)
	}
	trackPlaylistID := strconv.FormatInt(createdTrack.Data.Playlist.ID, 10)

	request("getPlaylist", http.MethodGet, "/api/music/playlists/"+trackPlaylistID, "", http.StatusOK)
	request("getPlaylistTracks", http.MethodGet, "/api/music/playlists/"+trackPlaylistID+"/tracks", "", http.StatusOK)
	request("getPlaylistCollaborators", http.MethodGet, "/api/music/playlists/"+trackPlaylistID+"/collaborators", "", http.StatusOK)

	request("addTracksToPlaylist", http.MethodPost, "/api/music/playlists/"+trackPlaylistID+"/tracks", `{"track_ids":[`+trackID+`]}`, http.StatusOK)
	request("reorderPlaylistTracks", http.MethodPut, "/api/music/playlists/"+trackPlaylistID+"/tracks/reorder", `{"track_ids":[`+trackID+`]}`, http.StatusOK)
	request("removeTrackFromPlaylist", http.MethodDelete, "/api/music/playlists/"+trackPlaylistID+"/tracks/"+trackID, "", http.StatusOK)
	request("addCollaborator", http.MethodPost, "/api/music/playlists/"+trackPlaylistID+"/collaborators", `{"user_id":`+outsiderID+`,"can_edit":true}`, http.StatusCreated)
	request("removeCollaborator", http.MethodDelete, "/api/music/playlists/"+trackPlaylistID+"/collaborators/"+outsiderID, "", http.StatusOK)
	request("updatePlaylist", http.MethodPut, "/api/music/playlists/"+trackPlaylistID, `{"name":"Updated Track Playlist"}`, http.StatusOK)

	createdMovieResponse := request("createMoviePlaylist", http.MethodPost, "/api/movies/playlists", `{"name":"Created Movie Playlist"}`, http.StatusCreated)
	var createdMovie struct {
		Data struct {
			Playlist database.Playlist `json:"playlist"`
		} `json:"data"`
	}
	err = json.Unmarshal(createdMovieResponse.Body.Bytes(), &createdMovie)
	if err != nil {
		t.Fatalf("decode created movie playlist: %v", err)
	}
	moviePlaylistID := strconv.FormatInt(createdMovie.Data.Playlist.ID, 10)

	request("getMoviePlaylist", http.MethodGet, "/api/movies/playlists/"+moviePlaylistID, "", http.StatusOK)
	request("getMoviePlaylistMovies", http.MethodGet, "/api/movies/playlists/"+moviePlaylistID+"/movies", "", http.StatusOK)
	request("getMoviePlaylistCollaborators", http.MethodGet, "/api/movies/playlists/"+moviePlaylistID+"/collaborators", "", http.StatusOK)
	request("addMoviesToMoviePlaylist", http.MethodPost, "/api/movies/playlists/"+moviePlaylistID+"/movies", `{"movie_ids":[`+movieID+`]}`, http.StatusOK)
	request("removeMovieFromMoviePlaylist", http.MethodDelete, "/api/movies/playlists/"+moviePlaylistID+"/movies/"+movieID, "", http.StatusOK)
	request("addMoviePlaylistCollaborator", http.MethodPost, "/api/movies/playlists/"+moviePlaylistID+"/collaborators", `{"user_id":`+outsiderID+`,"can_edit":true}`, http.StatusCreated)
	request("removeMoviePlaylistCollaborator", http.MethodDelete, "/api/movies/playlists/"+moviePlaylistID+"/collaborators/"+outsiderID, "", http.StatusOK)
	request("updateMoviePlaylist", http.MethodPut, "/api/movies/playlists/"+moviePlaylistID, `{"name":"Updated Movie Playlist"}`, http.StatusOK)

	request("deletePlaylist", http.MethodDelete, "/api/music/playlists/"+trackPlaylistID, "", http.StatusOK)
	request("deleteMoviePlaylist", http.MethodDelete, "/api/movies/playlists/"+moviePlaylistID, "", http.StatusOK)
}

// playlistTrackIDs returns the track ids a playlist serves, in playlist order.
func playlistTrackIDs(t *testing.T, app *Application, userID, playlistID int64) []int64 {
	t.Helper()

	w := serveAs(t, app, userID, http.MethodGet, "/api/music/playlists/"+strconv.FormatInt(playlistID, 10)+"/tracks", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list tracks status = %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Tracks []struct {
				ID int64 `json:"id"`
			} `json:"tracks"`
		} `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("decode tracks: %v", err)
	}
	ids := make([]int64, 0, len(body.Data.Tracks))
	for _, track := range body.Data.Tracks {
		ids = append(ids, track.ID)
	}
	return ids
}

func TestReorderPlaylistTracks_AppliesTheRequestedOrderForEditors(t *testing.T) {
	app := setupTestApp(t)
	fixtures := createPlaylistFixtures(t, app)
	musicianID := createTestMusician(t, app, "Reorder Artist")
	albumID := createTestAlbum(t, app, "Reorder Album", "Reorder Artist")
	second := createTestTrack(t, app, "Reorder Two", "/music/reorder-2.flac", albumID, musicianID)
	third := createTestTrack(t, app, "Reorder Three", "/music/reorder-3.flac", albumID, musicianID)
	playlistPath := "/api/music/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10)

	w := serveAs(t, app, fixtures.owner.ID, http.MethodPost, playlistPath+"/tracks",
		fmt.Sprintf(`{"track_ids":[%d,%d,%d]}`, fixtures.trackID, second, third))
	if w.Code != http.StatusOK {
		t.Fatalf("add tracks status = %d: %s", w.Code, w.Body.String())
	}
	if got := playlistTrackIDs(t, app, fixtures.owner.ID, fixtures.trackPlaylist.ID); !slices.Equal(got, []int64{fixtures.trackID, second, third}) {
		t.Fatalf("initial order = %v, want insertion order", got)
	}

	reordered := fmt.Sprintf(`{"track_ids":[%d,%d,%d]}`, third, fixtures.trackID, second)
	w = serveAs(t, app, fixtures.viewer.ID, http.MethodPut, playlistPath+"/tracks/reorder", reordered)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer reorder status = %d, want 403: %s", w.Code, w.Body.String())
	}
	w = serveAs(t, app, fixtures.editor.ID, http.MethodPut, playlistPath+"/tracks/reorder", reordered)
	if w.Code != http.StatusOK {
		t.Fatalf("editor reorder status = %d: %s", w.Code, w.Body.String())
	}
	if got := playlistTrackIDs(t, app, fixtures.owner.ID, fixtures.trackPlaylist.ID); !slices.Equal(got, []int64{third, fixtures.trackID, second}) {
		t.Fatalf("order after reorder = %v, want %v", got, []int64{third, fixtures.trackID, second})
	}
}

func TestDeletePlaylists_OnlyTheOwnerMay(t *testing.T) {
	app := setupTestApp(t)
	fixtures := createPlaylistFixtures(t, app)
	paths := map[string]string{
		"track playlist": "/api/music/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10),
		"movie playlist": "/api/movies/playlists/" + strconv.FormatInt(fixtures.moviePlaylist.ID, 10),
	}
	for name, path := range paths {
		t.Run(name, func(t *testing.T) {
			for _, collaborator := range []database.User{fixtures.editor, fixtures.viewer} {
				w := serveAs(t, app, collaborator.ID, http.MethodDelete, path, "")
				if w.Code != http.StatusForbidden {
					t.Fatalf("%s delete status = %d, want 403: %s", collaborator.Name, w.Code, w.Body.String())
				}
			}
			w := serveAs(t, app, fixtures.owner.ID, http.MethodDelete, path, "")
			if w.Code != http.StatusOK {
				t.Fatalf("owner delete status = %d: %s", w.Code, w.Body.String())
			}
			w = serveAs(t, app, fixtures.owner.ID, http.MethodGet, path, "")
			if w.Code != http.StatusNotFound {
				t.Fatalf("status after delete = %d, want 404: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestMoviePlaylistUpdateAndRemove_ErrorPaths(t *testing.T) {
	app := setupTestApp(t)
	fixtures := createPlaylistFixtures(t, app)
	playlistPath := "/api/movies/playlists/" + strconv.FormatInt(fixtures.moviePlaylist.ID, 10)
	movieID := strconv.FormatInt(fixtures.movieID, 10)

	w := serveAs(t, app, fixtures.owner.ID, http.MethodPost, playlistPath+"/movies", `{"movie_ids":[`+movieID+`]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("add movie status = %d: %s", w.Code, w.Body.String())
	}

	tests := []struct {
		name        string
		userID      int64
		method      string
		path        string
		body        string
		wantStatus  int
		wantMessage string
	}{
		{"update with a non-numeric id", fixtures.owner.ID, http.MethodPut, "/api/movies/playlists/abc", `{"name":"Renamed"}`, http.StatusBadRequest, invalidPlaylistIDMessage},
		{"update an unknown playlist", fixtures.owner.ID, http.MethodPut, "/api/movies/playlists/999999", `{"name":"Renamed"}`, http.StatusNotFound, playlistNotFoundMessage},
		{"update by an editor", fixtures.editor.ID, http.MethodPut, playlistPath, `{"name":"Renamed"}`, http.StatusForbidden, "only the playlist owner can update metadata"},
		{"update with a malformed body", fixtures.owner.ID, http.MethodPut, playlistPath, `{"name":`, http.StatusBadRequest, invalidRequestBodyMessage},
		{"update with an empty name", fixtures.owner.ID, http.MethodPut, playlistPath, `{"name":""}`, http.StatusBadRequest, "playlist name is required"},
		{"update with an unknown movie", fixtures.owner.ID, http.MethodPut, playlistPath, `{"name":"Renamed","movie_id":999999}`, http.StatusBadRequest, movieNotFoundMessage},
		{"remove with a non-numeric playlist id", fixtures.owner.ID, http.MethodDelete, "/api/movies/playlists/abc/movies/" + movieID, "", http.StatusBadRequest, invalidPlaylistIDMessage},
		{"remove with a non-numeric movie id", fixtures.owner.ID, http.MethodDelete, playlistPath + "/movies/abc", "", http.StatusBadRequest, invalidMovieIDMessage},
		{"remove from an unknown playlist", fixtures.owner.ID, http.MethodDelete, "/api/movies/playlists/999999/movies/" + movieID, "", http.StatusNotFound, playlistNotFoundMessage},
		{"remove by a viewer", fixtures.viewer.ID, http.MethodDelete, playlistPath + "/movies/" + movieID, "", http.StatusForbidden, "you don't have permission to edit this playlist"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := serveAs(t, app, tt.userID, tt.method, tt.path, tt.body)
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}
			var resp helpers.JSONResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			if err != nil || !resp.Error || resp.Message != tt.wantMessage {
				t.Fatalf("response = %s (%v), want %q", w.Body.String(), err, tt.wantMessage)
			}
		})
	}

	// The owner can pin the playlist to a movie that exists.
	w = serveAs(t, app, fixtures.owner.ID, http.MethodPut, playlistPath, `{"name":"Renamed","movie_id":`+movieID+`}`)
	if w.Code != http.StatusOK {
		t.Fatalf("owner update status = %d: %s", w.Code, w.Body.String())
	}
	var updatedName string
	var updatedMovieID sql.NullInt64
	err := app.DB.QueryRowContext(context.Background(), "SELECT name, movie_id FROM playlists WHERE id = ?", fixtures.moviePlaylist.ID).Scan(&updatedName, &updatedMovieID)
	if err != nil {
		t.Fatalf("read updated playlist: %v", err)
	}
	if updatedName != "Renamed" || !updatedMovieID.Valid || updatedMovieID.Int64 != fixtures.movieID {
		t.Fatalf("updated playlist = (%q, %+v), want Renamed pinned to movie %d", updatedName, updatedMovieID, fixtures.movieID)
	}
}
