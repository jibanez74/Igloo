package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
)

func TestPickGenreBackdrops(t *testing.T) {
	tests := []struct {
		name       string
		candidates [][]string
		want       []string
	}{
		{
			name:       "genres sharing their newest title get different backdrops",
			candidates: [][]string{{"/new.jpg", "/a.jpg"}, {"/new.jpg", "/b.jpg"}, {"/new.jpg", "/a.jpg", "/c.jpg"}},
			want:       []string{"/new.jpg", "/b.jpg", "/a.jpg"},
		},
		{
			name:       "a genre with fewer candidates chooses before a larger one listed earlier",
			candidates: [][]string{{"/shared.jpg", "/a.jpg"}, {"/shared.jpg"}},
			want:       []string{"/a.jpg", "/shared.jpg"},
		},
		{
			name:       "a genre whose candidates are all taken repeats its first",
			candidates: [][]string{{"/only.jpg"}, {"/only.jpg"}},
			want:       []string{"/only.jpg", "/only.jpg"},
		},
		{
			name:       "a genre without candidates gets none and takes nothing",
			candidates: [][]string{{}, {"/a.jpg"}},
			want:       []string{"", "/a.jpg"},
		},
		{
			name:       "no genres",
			candidates: [][]string{},
			want:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			picked := pickGenreBackdrops(tt.candidates)

			got := make([]string, len(picked))
			for i, backdrop := range picked {
				isSet := backdrop.String != ""
				if backdrop.Valid != isSet {
					t.Errorf("genre %d: Valid = %v for %q", i, backdrop.Valid, backdrop.String)
				}
				got[i] = backdrop.String
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("picked %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDistinctImages(t *testing.T) {
	tests := []struct {
		name   string
		images []string
		want   []string
	}{
		{name: "none", images: []string{}, want: []string{}},
		{name: "repeats collapse in first-seen order", images: []string{"a", "a", "b", "a", "c"}, want: []string{"a", "b", "c"}},
		{name: "stops at the limit", images: []string{"a", "b", "c", "d", "e"}, want: []string{"a", "b", "c", "d"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := distinctImages(tt.images, maxPlaylistArtwork)
			if got == nil {
				t.Fatal("distinctImages returned nil; the response needs []")
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecodeImageList(t *testing.T) {
	images, err := decodeImageList(`["/a.jpg","/b.jpg"]`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/a.jpg", "/b.jpg"}; !slices.Equal(images, want) {
		t.Errorf("got %q, want %q", images, want)
	}

	empty, err := decodeImageList(`[]`)
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Errorf("empty list = %#v, want a non-nil empty slice", empty)
	}

	_, err = decodeImageList(`not json`)
	if err == nil {
		t.Error("malformed text must be an error")
	}
}

type genreTile struct {
	GenreTag     string         `json:"genre_tag"`
	BackdropPath sql.NullString `json:"backdrop_path"`
}

func genreTilesByTag(t *testing.T, body []byte) map[string]sql.NullString {
	t.Helper()

	var resp struct {
		Data struct {
			Genres []genreTile `json:"genres"`
		} `json:"data"`
	}
	err := json.Unmarshal(body, &resp)
	if err != nil {
		t.Fatal(err)
	}

	byTag := map[string]sql.NullString{}
	for _, genre := range resp.Data.Genres {
		byTag[genre.GenreTag] = genre.BackdropPath
	}
	return byTag
}

func TestGetMovieGenresList_GivesEachGenreItsOwnBackdrop(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Viewer", "genre-tiles@example.com", false)

	older := createTestMovie(t, app, "Older", "/movies/genre-older.mkv")
	newest := createTestMovie(t, app, "Newest", "/movies/genre-newest.mkv")
	bare := createTestMovie(t, app, "Bare", "/movies/genre-bare.mkv")
	for movieID, backdrop := range map[int64]string{older: "/older.jpg", newest: "/newest.jpg"} {
		_, err := app.DB.Exec(`UPDATE movies SET backdrop_path_source = ? WHERE id = ?`, backdrop, movieID)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Action and Adventure share their newest movie; Western has no backdrop.
	createMovieGenre(t, app, older, "Action")
	createMovieGenre(t, app, newest, "Action")
	createMovieGenre(t, app, older, "Adventure")
	createMovieGenre(t, app, newest, "Adventure")
	createMovieGenre(t, app, bare, "Adventure")
	createMovieGenre(t, app, bare, "Western")

	w := serveAs(t, app, user.ID, http.MethodGet, "/api/movies/genres", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	tiles := genreTilesByTag(t, w.Body.Bytes())
	if want := (sql.NullString{String: "/newest.jpg", Valid: true}); tiles["Action"] != want {
		t.Errorf("Action backdrop = %+v, want %+v", tiles["Action"], want)
	}
	if want := (sql.NullString{String: "/older.jpg", Valid: true}); tiles["Adventure"] != want {
		t.Errorf("Adventure backdrop = %+v, want %+v", tiles["Adventure"], want)
	}
	if tiles["Western"].Valid {
		t.Errorf("Western backdrop = %+v, want null", tiles["Western"])
	}
	if strings.Contains(w.Body.String(), "backdrop_candidates") {
		t.Errorf("the candidates column must not reach the response: %s", w.Body.String())
	}
}

func TestGetShowGenresList_CarriesABackdrop(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Viewer", "show-genre-tiles@example.com", false)

	_, err := app.DB.Exec(`
		INSERT INTO shows (id, directory_path, local_name, name, backdrop_path_source) VALUES
			(1, '/shows/Lit', 'Lit', 'Lit', '/lit.jpg'),
			(2, '/shows/Dark', 'Dark', 'Dark', NULL);
		INSERT INTO genres (id, tag, genre_type) VALUES (1, 'Drama', 'show'), (2, 'Mystery', 'show');
		INSERT INTO show_genres (show_id, genre_id) VALUES (1, 1), (2, 1), (2, 2);
	`)
	if err != nil {
		t.Fatal(err)
	}

	w := serveAs(t, app, user.ID, http.MethodGet, "/api/shows/genres", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	tiles := genreTilesByTag(t, w.Body.Bytes())
	if want := (sql.NullString{String: "/lit.jpg", Valid: true}); tiles["Drama"] != want {
		t.Errorf("Drama backdrop = %+v, want %+v", tiles["Drama"], want)
	}
	if tiles["Mystery"].Valid {
		t.Errorf("Mystery backdrop = %+v, want null", tiles["Mystery"])
	}
}

func playlistArtworkByID(t *testing.T, body []byte) map[int64][]string {
	t.Helper()

	var resp struct {
		Data struct {
			Playlists []struct {
				ID      int64    `json:"id"`
				Artwork []string `json:"artwork"`
			} `json:"playlists"`
		} `json:"data"`
	}
	err := json.Unmarshal(body, &resp)
	if err != nil {
		t.Fatal(err)
	}

	byID := map[int64][]string{}
	for _, playlist := range resp.Data.Playlists {
		byID[playlist.ID] = playlist.Artwork
	}
	return byID
}

func TestGetPlaylists_CarriesDistinctCoversInPlaylistOrder(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Listener", "playlist-covers@example.com", false)
	musicianID := createTestMusician(t, app, "Collage Band")

	// Five albums with covers and one without; tracks are added so the first
	// album repeats and the coverless one sits in the middle.
	albumOrder := []string{"one", "one", "bare", "two", "three", "one", "four", "five"}
	albumIDs := map[string]int64{}
	for _, name := range []string{"one", "two", "three", "four", "five", "bare"} {
		albumIDs[name] = createTestAlbum(t, app, "Album "+name, "Collage Band")
		if name == "bare" {
			continue
		}
		_, err := app.DB.Exec(`UPDATE albums SET cover_source = ? WHERE id = ?`, "https://covers.example/"+name+".jpg", albumIDs[name])
		if err != nil {
			t.Fatal(err)
		}
	}

	filled, err := app.Queries.CreatePlaylist(t.Context(), database.CreatePlaylistParams{UserID: user.ID, Name: "Filled"})
	if err != nil {
		t.Fatal(err)
	}
	empty, err := app.Queries.CreatePlaylist(t.Context(), database.CreatePlaylistParams{UserID: user.ID, Name: "Empty"})
	if err != nil {
		t.Fatal(err)
	}
	for index, name := range albumOrder {
		trackID := createTestTrack(t, app, fmt.Sprintf("Track %d", index), fmt.Sprintf("/music/collage-%d.flac", index), albumIDs[name], musicianID)
		_, err = app.Queries.AddTrackToPlaylist(t.Context(), database.AddTrackToPlaylistParams{
			PlaylistID: filled.ID,
			TrackID:    trackID,
			AddedBy:    sql.NullInt64{Int64: user.ID, Valid: true},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	w := serveAs(t, app, user.ID, http.MethodGet, "/api/music/playlists", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	artwork := playlistArtworkByID(t, w.Body.Bytes())
	want := []string{
		"https://covers.example/one.jpg",
		"https://covers.example/two.jpg",
		"https://covers.example/three.jpg",
		"https://covers.example/four.jpg",
	}
	if !slices.Equal(artwork[filled.ID], want) {
		t.Errorf("artwork = %q, want %q", artwork[filled.ID], want)
	}
	if !strings.Contains(w.Body.String(), `"artwork":[]`) {
		t.Errorf("an empty playlist must send an empty artwork array: %s", w.Body.String())
	}
	if got := artwork[empty.ID]; len(got) != 0 {
		t.Errorf("empty playlist artwork = %q, want none", got)
	}
}

func TestGetMoviePlaylists_CarriesTheFirstFourPostersInAddedOrder(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Viewer", "playlist-posters@example.com", false)

	playlist, err := app.Queries.CreateMoviePlaylist(t.Context(), database.CreateMoviePlaylistParams{UserID: user.ID, Name: "Marathon"})
	if err != nil {
		t.Fatal(err)
	}
	empty, err := app.Queries.CreateMoviePlaylist(t.Context(), database.CreateMoviePlaylistParams{UserID: user.ID, Name: "Empty"})
	if err != nil {
		t.Fatal(err)
	}

	// Titles run against the added order, and the second movie has no poster.
	titles := []string{"Zulu", "Yankee", "X-Ray", "Whiskey", "Victor", "Uniform"}
	for index, title := range titles {
		movieID := createTestMovie(t, app, title, fmt.Sprintf("/movies/marathon-%d.mkv", index))
		if title != "Yankee" {
			_, err = app.DB.Exec(`UPDATE movies SET poster_path_source = ? WHERE id = ?`, "/"+title+".jpg", movieID)
			if err != nil {
				t.Fatal(err)
			}
		}
		_, err = app.Queries.AddMovieToPlaylist(t.Context(), database.AddMovieToPlaylistParams{
			PlaylistID: playlist.ID,
			MovieID:    movieID,
			AddedBy:    sql.NullInt64{Int64: user.ID, Valid: true},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	w := serveAs(t, app, user.ID, http.MethodGet, "/api/movies/playlists", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}

	artwork := playlistArtworkByID(t, w.Body.Bytes())
	want := []string{"/Zulu.jpg", "/X-Ray.jpg", "/Whiskey.jpg", "/Victor.jpg"}
	if !slices.Equal(artwork[playlist.ID], want) {
		t.Errorf("artwork = %q, want %q", artwork[playlist.ID], want)
	}
	if !strings.Contains(w.Body.String(), `"artwork":[]`) {
		t.Errorf("an empty playlist must send an empty artwork array: %s", w.Body.String())
	}
	if got := artwork[empty.ID]; len(got) != 0 {
		t.Errorf("empty playlist artwork = %q, want none", got)
	}
}
