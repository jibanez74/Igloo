package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"igloo/cmd/internal/database"
)

// TestLibraryAndStatisticsHandlers_ConformToOpenAPI validates every list
// operation against an empty library, where the item schema is never
// exercised. These run with one seeded movie, show, track, album, musician and
// playlist of each kind so the array elements are validated too.
func TestListHandlers_ConformToOpenAPIWithRows(t *testing.T) {
	app := setupSessionTestApp(t)

	user := createTestUser(t, app, "List User", "list-contract@example.com", false)
	collaborator := createTestUser(t, app, "List Collaborator", "list-collaborator@example.com", false)

	movieID := createTestMovie(t, app, "Contract List Movie", "/movies/contract-list.mkv")
	_, err := app.DB.Exec(
		"UPDATE movies SET certification = ?, year = ?, poster_path = ? WHERE id = ?",
		"PG-13", 2026, "/contract-list.jpg", movieID,
	)
	if err != nil {
		t.Fatalf("seed movie metadata: %v", err)
	}
	movieGenreID := createMovieGenre(t, app, movieID, "Contract Genre")
	moviePlaylistID := createMoviePlaylist(t, app, user.ID, movieID)
	_, err = app.DB.Exec("INSERT INTO user_liked_movies (user_id, movie_id) VALUES (?, ?)", user.ID, movieID)
	if err != nil {
		t.Fatalf("like movie: %v", err)
	}

	showID := seedContractShow(t, app)
	var showGenreID int64
	err = app.DB.QueryRow("SELECT genre_id FROM show_genres WHERE show_id = ?", showID).Scan(&showGenreID)
	if err != nil {
		t.Fatalf("read seeded show genre: %v", err)
	}

	musicianID := createSearchMusician(t, app, "Contract List Artist")
	albumID := createSearchAlbum(t, app, "Contract List Album", "Contract List Artist")
	trackID := createSearchTrack(t, app, "Contract List Track", "/music/contract-list.flac", albumID, musicianID)
	err = app.Queries.LikeTrack(t.Context(), database.LikeTrackParams{UserID: user.ID, TrackID: trackID})
	if err != nil {
		t.Fatalf("like track: %v", err)
	}
	trackPlaylistID := createTrackPlaylistWithTrack(t, app, user.ID, trackID)
	_, err = app.Queries.AddCollaborator(t.Context(), database.AddCollaboratorParams{
		PlaylistID: trackPlaylistID,
		UserID:     collaborator.ID,
		CanEdit:    true,
	})
	if err != nil {
		t.Fatalf("add playlist collaborator: %v", err)
	}

	handler := authenticatedRouter(t, app, user.ID)
	moviePlaylist := strconv.FormatInt(moviePlaylistID, 10)
	trackPlaylist := strconv.FormatInt(trackPlaylistID, 10)

	operations := []struct {
		operationID string
		path        string
		dataKey     string
	}{
		{operationID: "getLatestMovies", path: "/api/movies/latest", dataKey: "movies"},
		{operationID: "getMoviesLibrary", path: "/api/movies/library", dataKey: "movies"},
		{operationID: "getLikedMovies", path: "/api/movies/liked", dataKey: "movies"},
		{operationID: "getMoviesByGenreLibrary", path: "/api/movies/genres/" + strconv.FormatInt(movieGenreID, 10) + "/movies", dataKey: "movies"},
		{operationID: "getMoviePlaylistMovies", path: "/api/movies/playlists/" + moviePlaylist + "/movies", dataKey: "movies"},
		{operationID: "getMoviePlaylists", path: "/api/movies/playlists", dataKey: "playlists"},
		{operationID: "getMovieGenresList", path: "/api/movies/genres", dataKey: "genres"},
		{operationID: "getLatestShows", path: "/api/shows/latest", dataKey: "shows"},
		{operationID: "getShowsLibrary", path: "/api/shows/library", dataKey: "shows"},
		{operationID: "getShowsByGenreLibrary", path: "/api/shows/genres/" + strconv.FormatInt(showGenreID, 10) + "/shows", dataKey: "shows"},
		{operationID: "getShowGenresList", path: "/api/shows/genres", dataKey: "genres"},
		{operationID: "getTracksAlphabetical", path: "/api/music/tracks", dataKey: "tracks"},
		{operationID: "getShuffleTracks", path: "/api/music/tracks/shuffle", dataKey: "tracks"},
		{operationID: "getLikedTracks", path: "/api/music/tracks/liked", dataKey: "tracks"},
		{operationID: "getLikedTrackIDsForUser", path: "/api/music/tracks/liked-ids", dataKey: "liked_track_ids"},
		{operationID: "getAlbumsAlphabetical", path: "/api/music/albums", dataKey: "albums"},
		{operationID: "getLatestAlbums", path: "/api/music/albums/latest", dataKey: "albums"},
		{operationID: "getMusiciansAlphabetical", path: "/api/music/musicians", dataKey: "musicians"},
		{operationID: "getPlaylists", path: "/api/music/playlists", dataKey: "playlists"},
		{operationID: "getPlaylistTracks", path: "/api/music/playlists/" + trackPlaylist + "/tracks", dataKey: "tracks"},
		{operationID: "getPlaylistCollaborators", path: "/api/music/playlists/" + trackPlaylist + "/collaborators", dataKey: "collaborators"},
	}

	for _, operation := range operations {
		t.Run(operation.operationID, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, operation.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("%s status = %d, want %d, body = %s", operation.operationID, response.Code, http.StatusOK, response.Body.String())
			}

			assertResponseListNotEmpty(t, operation.operationID, response.Body.Bytes(), operation.dataKey)
			assertOpenAPIExchange(t, operation.operationID, request, response)
		})
	}
}

func createMovieGenre(t *testing.T, app *Application, movieID int64, name string) int64 {
	t.Helper()

	genreID, err := app.Queries.GetOrCreateGenre(t.Context(), database.GetOrCreateGenreParams{
		Tag:       name,
		GenreType: "movie",
	})
	if err != nil {
		t.Fatalf("create genre %q: %v", name, err)
	}
	err = app.Queries.CreateMovieGenre(t.Context(), database.CreateMovieGenreParams{
		MovieID: movieID,
		GenreID: genreID,
	})
	if err != nil {
		t.Fatalf("link genre %q: %v", name, err)
	}
	return genreID
}

func createMoviePlaylist(t *testing.T, app *Application, userID, movieID int64) int64 {
	t.Helper()

	playlist, err := app.Queries.CreateMoviePlaylist(t.Context(), database.CreateMoviePlaylistParams{
		UserID:   userID,
		Name:     "Contract Playlist",
		IsPublic: false,
	})
	if err != nil {
		t.Fatalf("create movie playlist: %v", err)
	}
	_, err = app.DB.Exec(
		"INSERT INTO playlist_movies (playlist_id, movie_id, position, added_by) VALUES (?, ?, 1, ?)",
		playlist.ID, movieID, userID,
	)
	if err != nil {
		t.Fatalf("add movie to playlist: %v", err)
	}
	return playlist.ID
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
