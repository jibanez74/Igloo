package main

import (
	"database/sql"
	"fmt"
	"slices"
	"testing"

	"igloo/cmd/internal/database"
)

// Timestamps in the schema have one-second resolution, so rows written in the
// same second tie on them. These lists promise an order in the contract, and
// an id (or rowid) term keeps that promise within one second. Where an index
// the planner could fall back on yields a different order, the case seeds its
// rows so that order differs from the id's; where every such index already
// ends in the id (album tracks, musician albums), the case pins the
// guarantee, and passes without the id term too.

const sameSecond = "2026-11-18 12:00:00"

func execOrFatal(t *testing.T, app *Application, query string, args ...any) int64 {
	t.Helper()

	result, err := app.DB.ExecContext(t.Context(), query, args...)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return id
}

func assertOrder(t *testing.T, what string, got, want []int64) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Fatalf("%s order = %v, want %v", what, got, want)
	}
}

func TestLikedTracks_MostRecentlyLikedFirstWithinOneSecond(t *testing.T) {
	app := setupTestApp(t)
	user := createTestUser(t, app, "Listener", "listener@example.com", false)
	musicianID := createTestMusician(t, app, "Artist")
	albumID := createTestAlbum(t, app, "Album", "Artist")
	first := createTestTrack(t, app, "One", "/music/one.flac", albumID, musicianID)
	second := createTestTrack(t, app, "Two", "/music/two.flac", albumID, musicianID)
	third := createTestTrack(t, app, "Three", "/music/three.flac", albumID, musicianID)

	// Liked out of track-id order, all in one second.
	for _, trackID := range []int64{second, third, first} {
		execOrFatal(t, app, "INSERT INTO user_liked_tracks (user_id, track_id, created_at) VALUES (?, ?, ?)",
			user.ID, trackID, sameSecond)
	}

	rows, err := app.Queries.GetLikedTracksForUser(t.Context(), database.GetLikedTracksForUserParams{
		UserID: user.ID, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, row := range rows {
		got = append(got, row.ID)
	}
	assertOrder(t, "liked tracks", got, []int64{first, third, second})
}

func TestRecentlyPlayed_NewestFirstWithinOneSecond(t *testing.T) {
	app := setupTestApp(t)
	user := createTestUser(t, app, "Listener", "listener@example.com", false)
	musicianID := createTestMusician(t, app, "Artist")
	albumID := createTestAlbum(t, app, "Album", "Artist")
	first := createTestTrack(t, app, "One", "/music/one.flac", albumID, musicianID)
	second := createTestTrack(t, app, "Two", "/music/two.flac", albumID, musicianID)
	third := createTestTrack(t, app, "Three", "/music/three.flac", albumID, musicianID)

	for _, trackID := range []int64{second, third, first} {
		execOrFatal(t, app, "INSERT INTO user_play_history (user_id, track_id, played_at) VALUES (?, ?, ?)",
			user.ID, trackID, sameSecond)
	}

	rows, err := app.Queries.GetUserRecentlyPlayed(t.Context(), database.GetUserRecentlyPlayedParams{
		UserID: user.ID, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, row := range rows {
		got = append(got, row.ID)
	}
	assertOrder(t, "recently played", got, []int64{first, third, second})
}

func TestLatestAlbums_NewestFirstWithinOneSecond(t *testing.T) {
	app := setupTestApp(t)
	var want []int64
	for _, title := range []string{"Alpha", "Bravo", "Charlie"} {
		want = append([]int64{createTestAlbum(t, app, title, "Artist")}, want...)
	}
	execOrFatal(t, app, "UPDATE albums SET created_at = ?", sameSecond)

	rows, err := app.Queries.GetLatestAlbums(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, row := range rows {
		got = append(got, row.ID)
	}
	assertOrder(t, "latest albums", got, want)
}

func TestWatchRooms_NewestFirstAndMembersInJoinOrderWithinOneSecond(t *testing.T) {
	app := setupTestApp(t)
	// Created in reverse, so the owner has the highest user id: ordering the
	// members by user id would put the owner last.
	invitee := createTestUser(t, app, "Invitee", "invitee@example.com", false)
	owner := createTestUser(t, app, "Owner", "owner@example.com", false)
	movieID := createTestMovie(t, app, "Room Movie", "/movies/room.mkv")

	var rooms []int64
	for range 3 {
		roomID := execOrFatal(t, app,
			"INSERT INTO watch_rooms (owner_user_id, movie_id, playback_mode, created_at) VALUES (?, ?, 'direct', ?)",
			owner.ID, movieID, sameSecond)
		rooms = append(rooms, roomID)
		for _, userID := range []int64{owner.ID, invitee.ID} {
			execOrFatal(t, app, "INSERT INTO watch_room_members (room_id, user_id, created_at) VALUES (?, ?, ?)",
				roomID, userID, sameSecond)
		}
	}

	listed, err := app.Queries.GetWatchRoomsForUser(t.Context(), invitee.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, row := range listed {
		got = append(got, row.ID)
	}
	assertOrder(t, "watch rooms", got, []int64{rooms[2], rooms[1], rooms[0]})

	members, err := app.Queries.GetWatchRoomMembers(t.Context(), rooms[0])
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	for _, row := range members {
		got = append(got, row.ID)
	}
	assertOrder(t, "room members", got, []int64{owner.ID, invitee.ID})

	batched, err := app.Queries.GetWatchRoomMembersByRoomIDs(t.Context(), rooms)
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	for _, row := range batched {
		if row.RoomID == rooms[1] {
			got = append(got, row.ID)
		}
	}
	assertOrder(t, "batched room members", got, []int64{owner.ID, invitee.ID})
}

func TestPlaylists_StableWithinOneSecond(t *testing.T) {
	app := setupTestApp(t)
	second := createTestUser(t, app, "Second", "second@example.com", false)
	first := createTestUser(t, app, "First", "first@example.com", false)
	owner := createTestUser(t, app, "Owner", "owner@example.com", false)

	var trackPlaylists, moviePlaylists []int64
	for range 3 {
		trackPlaylists = append([]int64{execOrFatal(t, app,
			"INSERT INTO playlists (user_id, name, content_type) VALUES (?, 'Tracks', 'track')", owner.ID)},
			trackPlaylists...)
		moviePlaylists = append([]int64{execOrFatal(t, app,
			"INSERT INTO playlists (user_id, name, content_type) VALUES (?, 'Movies', 'movie')", owner.ID)},
			moviePlaylists...)
	}
	execOrFatal(t, app, "UPDATE playlists SET created_at = ?, updated_at = ?", sameSecond, sameSecond)

	tracks, err := app.Queries.GetPlaylistsWithCollaboratorAccess(t.Context(), owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, row := range tracks {
		got = append(got, row.ID)
	}
	assertOrder(t, "music playlists", got, trackPlaylists)

	movies, err := app.Queries.GetMoviePlaylistsWithCollaboratorAccess(t.Context(), owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	for _, row := range movies {
		got = append(got, row.ID)
	}
	assertOrder(t, "movie playlists", got, moviePlaylists)

	// Added first to second in one second; first has the higher user id, so
	// ordering by user would reverse them.
	playlistID := trackPlaylists[0]
	for _, userID := range []int64{first.ID, second.ID} {
		execOrFatal(t, app, "INSERT INTO playlist_collaborators (playlist_id, user_id, created_at) VALUES (?, ?, ?)",
			playlistID, userID, sameSecond)
	}
	collaborators, err := app.Queries.GetPlaylistCollaborators(t.Context(), playlistID)
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	for _, row := range collaborators {
		got = append(got, row.UserID)
	}
	assertOrder(t, "collaborators", got, []int64{first.ID, second.ID})
}

func TestTopListeningStats_TiesInIDOrder(t *testing.T) {
	app := setupTestApp(t)
	user := createTestUser(t, app, "Listener", "listener@example.com", false)

	var tracks, musicians, albums, genres []int64
	for _, name := range []string{"One", "Two", "Three"} {
		musicianID := createTestMusician(t, app, "Artist "+name)
		albumID := createTestAlbum(t, app, "Album "+name, "Artist "+name)
		trackID := createTestTrack(t, app, name, "/music/"+name+".flac", albumID, musicianID)
		genreID, err := app.Queries.GetOrCreateGenre(t.Context(), database.GetOrCreateGenreParams{
			Tag: "genre " + name, GenreType: "music",
		})
		if err != nil {
			t.Fatal(err)
		}
		err = app.Queries.CreateTrackGenre(t.Context(), database.CreateTrackGenreParams{
			TrackID: trackID, GenreID: genreID,
		})
		if err != nil {
			t.Fatal(err)
		}
		tracks = append(tracks, trackID)
		musicians = append(musicians, musicianID)
		albums = append(albums, albumID)
		genres = append(genres, genreID)
	}

	// Equal play counts, stored out of track-id order so the stats rowid
	// order differs from the id order the lists promise.
	for _, trackID := range []int64{tracks[2], tracks[0], tracks[1]} {
		execOrFatal(t, app,
			"INSERT INTO user_track_stats (user_id, track_id, play_count, total_time_played) VALUES (?, ?, 5, 900)",
			user.ID, trackID)
	}

	topTracks, err := app.Queries.GetUserTopTracks(t.Context(), database.GetUserTopTracksParams{
		UserID: user.ID, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, row := range topTracks {
		got = append(got, row.ID)
	}
	assertOrder(t, "top tracks", got, tracks)

	// A second page starts where the first ended.
	page, err := app.Queries.GetUserTopTracks(t.Context(), database.GetUserTopTracksParams{
		UserID: user.ID, Limit: 2, Offset: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	for _, row := range page {
		got = append(got, row.ID)
	}
	assertOrder(t, "top tracks second page", got, tracks[2:])

	topMusicians, err := app.Queries.GetUserTopMusicians(t.Context(), database.GetUserTopMusiciansParams{
		UserID: user.ID, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	for _, row := range topMusicians {
		got = append(got, row.ID)
	}
	assertOrder(t, "top musicians", got, musicians)

	topAlbums, err := app.Queries.GetUserTopAlbums(t.Context(), database.GetUserTopAlbumsParams{
		UserID: user.ID, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	for _, row := range topAlbums {
		got = append(got, row.ID)
	}
	assertOrder(t, "top albums", got, albums)

	topGenres, err := app.Queries.GetUserTopGenres(t.Context(), database.GetUserTopGenresParams{
		UserID: user.ID, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	for _, row := range topGenres {
		got = append(got, row.ID)
	}
	assertOrder(t, "top genres", got, genres)
}

func TestAlbumAndMusicianPages_TiesInIDOrder(t *testing.T) {
	app := setupTestApp(t)
	musicianID := createTestMusician(t, app, "Artist")
	albumID := createTestAlbum(t, app, "Album", "Artist")
	var want []int64
	for _, name := range []string{"Charlie", "Alpha", "Bravo"} {
		want = append(want, createTestTrack(t, app, name, "/music/"+name+".flac", albumID, musicianID))
	}
	execOrFatal(t, app, "UPDATE tracks SET disc = 1, track_index = 0 WHERE album_id = ?", albumID)

	rows, err := app.Queries.GetTracksByAlbumID(t.Context(), sql.NullInt64{Int64: albumID, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, row := range rows {
		got = append(got, row.ID)
	}
	assertOrder(t, "album tracks", got, want)

	// The musician page's albums: undated last, then the id among albums
	// sharing a date, year and sort title.
	var albums []int64
	for _, title := range []string{"Same Title (1)", "Same Title (2)", "Same Title (3)"} {
		albums = append(albums, createTestAlbum(t, app, title, "Artist"))
	}
	execOrFatal(t, app,
		"UPDATE albums SET sort_title = 'same title', release_date = '2001-01-01', year = 2001 WHERE id IN (?, ?, ?)",
		albums[0], albums[1], albums[2])
	for _, id := range []int64{albums[2], albumID, albums[0], albums[1]} {
		execOrFatal(t, app, "INSERT INTO musician_albums (musician_id, album_id) VALUES (?, ?) ON CONFLICT DO NOTHING",
			musicianID, id)
	}
	musicianAlbums, err := app.Queries.GetAlbumsByMusicianID(t.Context(), musicianID)
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	for _, row := range musicianAlbums {
		got = append(got, row.ID)
	}
	assertOrder(t, "musician albums", got, append(albums, albumID))
}

// The continue-watching row merges two halves that each stop at the limit, so
// each half has to break same-second ties as the merge does (lowest id first),
// or the limit could keep rows the merged row then would not show.
func TestContinueWatchingHalves_KeepLowestIDsWithinOneSecond(t *testing.T) {
	app := setupTestApp(t)
	user := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	count := continueWatchingLimit + 1

	var movies []int64
	for i := range count {
		movies = append(movies, createTestMovie(t, app, fmt.Sprintf("Movie %02d", i), fmt.Sprintf("/movies/%02d.mkv", i)))
	}
	// Saved in reverse, so rowid order is the opposite of movie-id order.
	for _, movieID := range slices.Backward(movies) {
		execOrFatal(t, app,
			"INSERT INTO movie_watch_progress (user_id, movie_id, progress_sec, duration_sec, updated_at) VALUES (?, ?, 600, 5400, ?)",
			user.ID, movieID, sameSecond)
	}

	movieRows, err := app.Queries.GetContinueWatchingMovies(t.Context(), database.GetContinueWatchingMoviesParams{
		UserID: user.ID, Limit: continueWatchingLimit,
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, row := range movieRows {
		got = append(got, row.ID)
	}
	assertOrder(t, "continue watching movies", got, movies[:continueWatchingLimit])

	// Shows are created first and their episodes in reverse, so show-id order
	// (the GROUP BY's) is the opposite of episode-id order.
	var seasons []int64
	for i := range count {
		showID := execOrFatal(t, app, "INSERT INTO shows (directory_path, local_name, name) VALUES (?, ?, ?)",
			fmt.Sprintf("/shows/Show %02d", i), fmt.Sprintf("Show %02d", i), fmt.Sprintf("Show %02d", i))
		seasons = append(seasons, execOrFatal(t, app,
			"INSERT INTO show_seasons (show_id, season_number, name) VALUES (?, 1, 'Season 1')", showID))
	}
	var episodes []int64
	for i, seasonID := range slices.Backward(seasons) {
		fileID := execOrFatal(t, app,
			"INSERT INTO show_files (season_id, file_path, file_name, size, container, mime_type) VALUES (?, ?, ?, 1, 'mkv', 'video/x-matroska')",
			seasonID, fmt.Sprintf("/shows/Show %02d/S01E01.mkv", i), "S01E01.mkv")
		episodeID := execOrFatal(t, app,
			"INSERT INTO show_episodes (season_id, episode_number, name) VALUES (?, 1, 'Pilot')", seasonID)
		execOrFatal(t, app,
			"INSERT INTO show_episode_files (episode_id, file_id, season_id, episode_order) VALUES (?, ?, ?, 0)",
			episodeID, fileID, seasonID)
		execOrFatal(t, app,
			"INSERT INTO show_episode_watch_progress (user_id, episode_id, progress_sec, duration_sec, updated_at) VALUES (?, ?, 600, 5400, ?)",
			user.ID, episodeID, sameSecond)
		episodes = append(episodes, episodeID)
	}

	episodeRows, err := app.Queries.GetContinueWatchingEpisodes(t.Context(), database.GetContinueWatchingEpisodesParams{
		UserID: user.ID, Limit: continueWatchingLimit,
	})
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	for _, row := range episodeRows {
		got = append(got, row.ID)
	}
	assertOrder(t, "continue watching episodes", got, episodes[:continueWatchingLimit])
}

func TestContinueWatchingEpisodes_OneShowTieKeepsLowestEpisodeID(t *testing.T) {
	app := setupTestApp(t)
	user := createTestUser(t, app, "Viewer", "viewer@example.com", false)

	showID := execOrFatal(t, app, "INSERT INTO shows (directory_path, local_name, name) VALUES (?, ?, ?)",
		"/shows/Show", "Show", "Show")
	seasonID := execOrFatal(t, app,
		"INSERT INTO show_seasons (show_id, season_number, name) VALUES (?, 1, 'Season 1')", showID)
	var episodes []int64
	for i := range 3 {
		fileID := execOrFatal(t, app,
			"INSERT INTO show_files (season_id, file_path, file_name, size, container, mime_type) VALUES (?, ?, ?, 1, 'mkv', 'video/x-matroska')",
			seasonID, fmt.Sprintf("/shows/Show/S01E%02d.mkv", i+1), fmt.Sprintf("S01E%02d.mkv", i+1))
		episodeID := execOrFatal(t, app,
			"INSERT INTO show_episodes (season_id, episode_number, name) VALUES (?, ?, ?)",
			seasonID, i+1, fmt.Sprintf("Episode %d", i+1))
		execOrFatal(t, app,
			"INSERT INTO show_episode_files (episode_id, file_id, season_id, episode_order) VALUES (?, ?, ?, 0)",
			episodeID, fileID, seasonID)
		episodes = append(episodes, episodeID)
	}
	// Saved in reverse, so the table's order is the opposite of episode-id
	// order, and all in one second: only the id can choose the show's card.
	for _, episodeID := range slices.Backward(episodes) {
		execOrFatal(t, app,
			"INSERT INTO show_episode_watch_progress (user_id, episode_id, progress_sec, duration_sec, updated_at) VALUES (?, ?, 600, 5400, ?)",
			user.ID, episodeID, sameSecond)
	}

	rows, err := app.Queries.GetContinueWatchingEpisodes(t.Context(), database.GetContinueWatchingEpisodesParams{
		UserID: user.ID, Limit: continueWatchingLimit,
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, row := range rows {
		got = append(got, row.ID)
	}
	assertOrder(t, "continue watching episodes of one show", got, episodes[:1])
}
