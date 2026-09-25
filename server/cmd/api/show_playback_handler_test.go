package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/keyframeindex"
)

// playbackEpisodeFixture is one show with one combined file backing two
// episodes, the shape every episode playback test needs: the file has a
// video, an audio and a subtitle stream plus a chapter, and both episodes
// resolve to it.
type playbackEpisodeFixture struct {
	ShowID    int64
	SeasonID  int64
	FileID    int64
	Episode1  int64
	Episode2  int64
	FilePath  string
	Container string
}

func seedPlaybackEpisode(t *testing.T, app *Application) playbackEpisodeFixture {
	t.Helper()
	path := fmt.Sprintf("/tmp/%s-S01E01E02.mkv", sanitizeTestPathComponent(t.Name()))
	return seedPlaybackEpisodeAt(t, app, path, "mkv")
}

// seedPlaybackEpisodeAt is seedPlaybackEpisode with an explicit file path and
// container, for tests that put a real media file on disk.
func seedPlaybackEpisodeAt(t *testing.T, app *Application, path string, container string) playbackEpisodeFixture {
	t.Helper()
	ctx := context.Background()
	q := app.Queries

	show, err := q.UpsertLocalShow(ctx, database.UpsertLocalShowParams{
		DirectoryPath: "/shows/Playback Show (2024)",
		LocalName:     "Playback Show",
		PremiereYear:  sql.NullInt64{Int64: 2024, Valid: true},
		Name:          "Playback Show",
	})
	if err != nil {
		t.Fatalf("upsert show: %v", err)
	}
	err = q.UpdateShowMetadata(ctx, database.UpdateShowMetadataParams{
		Name:         "Playback Show",
		TmdbID:       sql.NullInt64{Int64: 4242, Valid: true},
		PosterPath:   sql.NullString{String: "/playback-poster.jpg", Valid: true},
		BackdropPath: sql.NullString{String: "/playback-backdrop.jpg", Valid: true},
		ID:           show.ID,
	})
	if err != nil {
		t.Fatalf("update show metadata: %v", err)
	}

	season, err := q.UpsertLocalShowSeason(ctx, database.UpsertLocalShowSeasonParams{
		ShowID:       show.ID,
		SeasonNumber: 1,
		Name:         "Season 1",
	})
	if err != nil {
		t.Fatalf("upsert season: %v", err)
	}

	file, err := q.UpsertShowFile(ctx, database.UpsertShowFileParams{
		SeasonID:  season.ID,
		FilePath:  path,
		FileName:  filepath.Base(path),
		Size:      1_000_000,
		Container: container,
		MimeType:  helpers.VideoMimeTypes[container],
		Duration:  sql.NullFloat64{Float64: 7200, Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert show file: %v", err)
	}

	var episodeIDs []int64
	for order, number := range []int64{1, 2} {
		episode, upsertErr := q.UpsertLocalShowEpisode(ctx, database.UpsertLocalShowEpisodeParams{
			SeasonID:      season.ID,
			EpisodeNumber: number,
			Name:          fmt.Sprintf("Episode %d", number),
		})
		if upsertErr != nil {
			t.Fatalf("upsert episode %d: %v", number, upsertErr)
		}
		linkErr := q.LinkShowEpisodeFile(ctx, database.LinkShowEpisodeFileParams{
			EpisodeID:    episode.ID,
			FileID:       file.ID,
			SeasonID:     season.ID,
			EpisodeOrder: int64(order),
		})
		if linkErr != nil {
			t.Fatalf("link episode %d: %v", number, linkErr)
		}
		episodeIDs = append(episodeIDs, episode.ID)
	}
	// The first episode is enriched so the nullable columns are exercised
	// both ways across the two rows.
	err = q.UpdateShowEpisodeMetadata(ctx, database.UpdateShowEpisodeMetadataParams{
		Name:        "Pilot",
		TmdbID:      sql.NullInt64{Int64: 70101, Valid: true},
		Overview:    sql.NullString{String: "It begins.", Valid: true},
		AirDate:     sql.NullString{String: "2024-03-08", Valid: true},
		StillPath:   sql.NullString{String: "/still.jpg", Valid: true},
		TmdbRuntime: sql.NullInt64{Int64: 47, Valid: true},
		VoteAverage: sql.NullFloat64{Float64: 8.1, Valid: true},
		VoteCount:   sql.NullInt64{Int64: 220, Valid: true},
		ID:          episodeIDs[0],
	})
	if err != nil {
		t.Fatalf("update episode metadata: %v", err)
	}

	// codec_profile LC keeps the fixture on the audio-copy path, as the movie
	// HLS fixture does.
	_, err = app.DB.Exec(`
		INSERT INTO show_video_streams (file_id, stream_index, codec, codec_profile, bit_rate, width, height, frame_rate, bit_depth, pixel_format)
		VALUES (?, 0, 'h264', 'High', 5000000, 1920, 1080, 23.976, 8, 'yuv420p');
		INSERT INTO show_audio_streams (file_id, stream_index, codec, codec_profile, bit_rate, channels, language, is_default)
		VALUES (?, 1, 'aac', 'LC', 192000, 2, 'eng', true);
		INSERT INTO show_subtitles (file_id, stream_index, codec, language)
		VALUES (?, ?, 'subrip', 'eng');
		INSERT INTO show_chapters (file_id, title, start_time)
		VALUES (?, 'Cold Open', 0), (?, 'Past The End', 9999);
	`, file.ID, file.ID, file.ID, testSubtitleStreamIndex, file.ID, file.ID)
	if err != nil {
		t.Fatalf("insert show streams: %v", err)
	}

	return playbackEpisodeFixture{
		ShowID:    show.ID,
		SeasonID:  season.ID,
		FileID:    file.ID,
		Episode1:  episodeIDs[0],
		Episode2:  episodeIDs[1],
		FilePath:  path,
		Container: container,
	}
}

func episodePlaybackSource(t *testing.T, app *Application, episodeID int64) playbackSource {
	t.Helper()
	source, err := app.loadPlaybackSource(context.Background(), episodeRef(episodeID))
	if err != nil {
		t.Fatalf("load episode source: %v", err)
	}
	return source
}

func TestGetShowEpisode_ConformsToOpenAPI(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Episode User", "episode@example.com", false)
	fixture := seedPlaybackEpisode(t, app)
	chain := seedNextEpisodeChain(t, app, fixture)

	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/episodes/%d", fixture.Episode1), nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}

	var body struct {
		Data struct {
			Show struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"show"`
			Season struct {
				SeasonNumber int64 `json:"season_number"`
			} `json:"season"`
			Episode struct {
				ID            int64  `json:"id"`
				EpisodeNumber int64  `json:"episode_number"`
				Name          string `json:"name"`
			} `json:"episode"`
			NextEpisode *struct {
				ID int64 `json:"id"`
			} `json:"next_episode"`
		} `json:"data"`
	}
	err := json.Unmarshal(response.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Data.Show.ID != fixture.ShowID || body.Data.Show.Name != "Playback Show" {
		t.Fatalf("show = %+v, want the fixture show", body.Data.Show)
	}
	if body.Data.Season.SeasonNumber != 1 || body.Data.Episode.EpisodeNumber != 1 || body.Data.Episode.Name != "Pilot" {
		t.Fatalf("season/episode = %+v / %+v", body.Data.Season, body.Data.Episode)
	}

	// S1E2 shares S1E1's file and has already played with it, so S1E3 is next.
	if body.Data.NextEpisode == nil || body.Data.NextEpisode.ID != chain.S1E3 {
		t.Fatalf("next_episode = %+v, want episode %d", body.Data.NextEpisode, chain.S1E3)
	}

	assertOpenAPIExchange(t, "getShowEpisode", request, response)
}

// nextEpisodeChain extends the playback fixture (S1E1+S1E2 in one file) with
// S1E3, S2E1 and the special S0E1, each in its own file, so every "next"
// rule has an episode to answer with.
type nextEpisodeChain struct {
	S1E3 int64
	S2E1 int64
	S0E1 int64
}

func seedNextEpisodeChain(t *testing.T, app *Application, fixture playbackEpisodeFixture) nextEpisodeChain {
	t.Helper()
	ctx := context.Background()
	q := app.Queries

	addEpisode := func(seasonNumber, episodeNumber int64) int64 {
		season, err := q.UpsertLocalShowSeason(ctx, database.UpsertLocalShowSeasonParams{
			ShowID:       fixture.ShowID,
			SeasonNumber: seasonNumber,
			Name:         fmt.Sprintf("Season %d", seasonNumber),
		})
		if err != nil {
			t.Fatalf("upsert season %d: %v", seasonNumber, err)
		}
		path := fmt.Sprintf("/tmp/%s-S%02dE%02d.mkv", sanitizeTestPathComponent(t.Name()), seasonNumber, episodeNumber)
		file, err := q.UpsertShowFile(ctx, database.UpsertShowFileParams{
			SeasonID:  season.ID,
			FilePath:  path,
			FileName:  filepath.Base(path),
			Size:      1_000_000,
			Container: "mkv",
			MimeType:  helpers.VideoMimeTypes["mkv"],
			Duration:  sql.NullFloat64{Float64: 2700, Valid: true},
		})
		if err != nil {
			t.Fatalf("upsert file S%dE%d: %v", seasonNumber, episodeNumber, err)
		}
		episode, err := q.UpsertLocalShowEpisode(ctx, database.UpsertLocalShowEpisodeParams{
			SeasonID:      season.ID,
			EpisodeNumber: episodeNumber,
			Name:          fmt.Sprintf("S%dE%d", seasonNumber, episodeNumber),
		})
		if err != nil {
			t.Fatalf("upsert episode S%dE%d: %v", seasonNumber, episodeNumber, err)
		}
		err = q.LinkShowEpisodeFile(ctx, database.LinkShowEpisodeFileParams{
			EpisodeID:    episode.ID,
			FileID:       file.ID,
			SeasonID:     season.ID,
			EpisodeOrder: 0,
		})
		if err != nil {
			t.Fatalf("link episode S%dE%d: %v", seasonNumber, episodeNumber, err)
		}
		return episode.ID
	}

	// Seeded out of listing order so the query, not insertion order, sorts.
	s0e1 := addEpisode(0, 1)
	s2e1 := addEpisode(2, 1)
	s1e3 := addEpisode(1, 3)
	return nextEpisodeChain{S1E3: s1e3, S2E1: s2e1, S0E1: s0e1}
}

func TestGetShowNextEpisode_FollowsListingOrderAndSkipsTheCombinedFile(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Next User", "next@example.com", false)
	other := createTestUser(t, app, "Other User", "other@example.com", false)
	fixture := seedPlaybackEpisode(t, app)
	chain := seedNextEpisodeChain(t, app, fixture)

	ctx := context.Background()
	err := app.Queries.UpsertShowEpisodeWatchProgress(ctx, database.UpsertShowEpisodeWatchProgressParams{
		UserID:        user.ID,
		EpisodeID:     chain.S1E3,
		ProgressSec:   600,
		DurationSec:   2700,
		SaveSessionID: "11111111-1111-4111-8111-111111111111",
		SaveSequence:  1,
	})
	if err != nil {
		t.Fatalf("seed progress: %v", err)
	}

	for _, tc := range []struct {
		name    string
		userID  int64
		current int64
		want    int64
		wantSec float64
	}{
		// S1E2 shares S1E1's file, so the whole file has already played.
		{"combined file skips its sibling", user.ID, fixture.Episode1, chain.S1E3, 600},
		{"other users see no progress", other.ID, fixture.Episode1, chain.S1E3, 0},
		{"last of a season rolls into the next season", user.ID, chain.S1E3, chain.S2E1, 0},
		{"specials follow the last regular season", user.ID, chain.S2E1, chain.S0E1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next, err := app.Queries.GetShowNextEpisode(ctx, database.GetShowNextEpisodeParams{UserID: tc.userID, EpisodeID: tc.current})
			if err != nil {
				t.Fatalf("next episode: %v", err)
			}
			if next.ID != tc.want {
				t.Fatalf("next of %d = %d, want %d", tc.current, next.ID, tc.want)
			}
			if next.ProgressSec.Float64 != tc.wantSec || next.Watched {
				t.Fatalf("progress = %+v / watched %v, want %v / false", next.ProgressSec, next.Watched, tc.wantSec)
			}
		})
	}

	_, err = app.Queries.GetShowNextEpisode(ctx, database.GetShowNextEpisodeParams{UserID: user.ID, EpisodeID: chain.S0E1})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("next of the last special = %v, want no rows", err)
	}

	// The header route serves null after the last episode.
	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)
	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/episodes/%d", chain.S0E1), nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"next_episode":null`) {
		t.Fatalf("last episode header = %d %s, want next_episode null", response.Code, response.Body.String())
	}
	assertOpenAPIExchange(t, "getShowEpisode", request, response)
}

func TestGetShowNextEpisode_SkipsEpisodesWithoutAPlayableFile(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Gap User", "gap@example.com", false)
	fixture := seedPlaybackEpisode(t, app)
	chain := seedNextEpisodeChain(t, app, fixture)

	ctx := context.Background()
	season, err := app.Queries.UpsertLocalShowSeason(ctx, database.UpsertLocalShowSeasonParams{
		ShowID:       fixture.ShowID,
		SeasonNumber: 1,
		Name:         "Season 1",
	})
	if err != nil {
		t.Fatalf("upsert season 1: %v", err)
	}
	// TMDB knows S1E4, but the library holds no file for it: the episode row
	// exists with nothing to play, so the hand-off has to step over it.
	gap, err := app.Queries.UpsertLocalShowEpisode(ctx, database.UpsertLocalShowEpisodeParams{
		SeasonID:      season.ID,
		EpisodeNumber: 4,
		Name:          "S1E4",
	})
	if err != nil {
		t.Fatalf("upsert metadata-only episode: %v", err)
	}

	next, err := app.Queries.GetShowNextEpisode(ctx, database.GetShowNextEpisodeParams{
		UserID:    user.ID,
		EpisodeID: chain.S1E3,
	})
	if err != nil {
		t.Fatalf("next episode: %v", err)
	}
	if next.ID == gap.ID {
		t.Fatalf("next of S1E3 = the metadata-only episode %d, want a playable one", gap.ID)
	}
	if next.ID != chain.S2E1 {
		t.Fatalf("next of S1E3 = %d, want S2E1 %d", next.ID, chain.S2E1)
	}

	// The gap is not a dead end either: the episode after it still answers.
	next, err = app.Queries.GetShowNextEpisode(ctx, database.GetShowNextEpisodeParams{
		UserID:    user.ID,
		EpisodeID: gap.ID,
	})
	if err != nil {
		t.Fatalf("next of the metadata-only episode: %v", err)
	}
	if next.ID != chain.S2E1 {
		t.Fatalf("next of the metadata-only episode = %d, want S2E1 %d", next.ID, chain.S2E1)
	}
}

func TestGetShowNextEpisode_SkipsASiblingThatWouldReplayTheCombinedFile(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Dupe User", "dupe@example.com", false)
	fixture := seedPlaybackEpisode(t, app)
	chain := seedNextEpisodeChain(t, app, fixture)

	ctx := context.Background()
	// The library also holds a standalone copy of S1E2. Scanned after the
	// combined file, it carries the higher file id and so loses the
	// lowest-id race GetShowFileForEpisode runs: owning a file of its own
	// does not make S1E2 playable as anything but the file that just ended.
	path := fmt.Sprintf("/tmp/%s-S01E02.mkv", sanitizeTestPathComponent(t.Name()))
	duplicate, err := app.Queries.UpsertShowFile(ctx, database.UpsertShowFileParams{
		SeasonID:  fixture.SeasonID,
		FilePath:  path,
		FileName:  filepath.Base(path),
		Size:      1_000_000,
		Container: "mkv",
		MimeType:  helpers.VideoMimeTypes["mkv"],
		Duration:  sql.NullFloat64{Float64: 2700, Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert duplicate file: %v", err)
	}
	if duplicate.ID <= fixture.FileID {
		t.Fatalf("duplicate file id = %d, want one above the combined file %d", duplicate.ID, fixture.FileID)
	}
	err = app.Queries.LinkShowEpisodeFile(ctx, database.LinkShowEpisodeFileParams{
		EpisodeID:    fixture.Episode2,
		FileID:       duplicate.ID,
		SeasonID:     fixture.SeasonID,
		EpisodeOrder: 0,
	})
	if err != nil {
		t.Fatalf("link duplicate file: %v", err)
	}

	selected, err := app.Queries.GetShowFileForEpisode(ctx, fixture.Episode2)
	if err != nil {
		t.Fatalf("selected file for S1E2: %v", err)
	}
	if selected.ID != fixture.FileID {
		t.Fatalf("S1E2 plays file %d, want the combined file %d", selected.ID, fixture.FileID)
	}

	next, err := app.Queries.GetShowNextEpisode(ctx, database.GetShowNextEpisodeParams{
		UserID:    user.ID,
		EpisodeID: fixture.Episode1,
	})
	if err != nil {
		t.Fatalf("next episode: %v", err)
	}
	if next.ID == fixture.Episode2 {
		t.Fatalf("next of S1E1 = S1E2 %d, which would replay the combined file", fixture.Episode2)
	}
	if next.ID != chain.S1E3 {
		t.Fatalf("next of S1E1 = %d, want S1E3 %d", next.ID, chain.S1E3)
	}
}

func TestGetShowEpisode_UnknownAndMalformed(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Episode User", "episode@example.com", false)
	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	for _, tc := range []struct {
		path   string
		status int
		body   string
	}{
		{"/api/shows/episodes/999999", http.StatusNotFound, "episode not found"},
		{"/api/shows/episodes/abc", http.StatusBadRequest, "invalid episode id"},
		{"/api/shows/episodes/999999/technical-details", http.StatusNotFound, "episode not found"},
		{"/api/shows/episodes/999999/watch-progress", http.StatusNotFound, "episode not found"},
		{"/api/shows/episodes/999999/stream", http.StatusNotFound, "episode not found"},
		{"/api/shows/episodes/999999/subtitles/0/web.vtt", http.StatusNotFound, "episode not found"},
	} {
		request := httptest.NewRequest(http.MethodGet, tc.path, nil)
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		app.Router.ServeHTTP(response, request)
		if response.Code != tc.status || !strings.Contains(response.Body.String(), tc.body) {
			t.Fatalf("%s: response = %d %s, want %d %q", tc.path, response.Code, response.Body.String(), tc.status, tc.body)
		}
	}
}

func TestGetShowEpisodeTechnicalDetails_ConformsToOpenAPI(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Episode Tech User", "episode-tech@example.com", false)
	fixture := seedPlaybackEpisode(t, app)

	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	type techBody struct {
		Data struct {
			File struct {
				FileName  string          `json:"file_name"`
				Container string          `json:"container"`
				MimeType  string          `json:"mime_type"`
				Duration  sql.NullFloat64 `json:"duration"`
			} `json:"file"`
			VideoStreams []struct {
				FileID int64  `json:"file_id"`
				Codec  string `json:"codec"`
			} `json:"video_streams"`
			AudioStreams []json.RawMessage `json:"audio_streams"`
			Subtitles    []json.RawMessage `json:"subtitles"`
			Chapters     []struct {
				StartTime int64 `json:"start_time"`
			} `json:"chapters"`
		} `json:"data"`
	}

	// Both episodes of the combined file describe the same file.
	var bodies []techBody
	for _, episodeID := range []int64{fixture.Episode1, fixture.Episode2} {
		request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/episodes/%d/technical-details", episodeID), nil)
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		app.Router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}

		var body techBody
		err := json.Unmarshal(response.Body.Bytes(), &body)
		if err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(body.Data.VideoStreams) != 1 || len(body.Data.AudioStreams) != 1 || len(body.Data.Subtitles) != 1 || len(body.Data.Chapters) != 2 {
			t.Fatalf("stream counts = %d/%d/%d/%d, want 1/1/1/2: %s", len(body.Data.VideoStreams), len(body.Data.AudioStreams), len(body.Data.Subtitles), len(body.Data.Chapters), response.Body.String())
		}
		if body.Data.VideoStreams[0].FileID != fixture.FileID || body.Data.VideoStreams[0].Codec != "h264" {
			t.Fatalf("video stream = %+v", body.Data.VideoStreams[0])
		}
		if body.Data.File.MimeType != "video/x-matroska" || body.Data.File.Container != "mkv" || !body.Data.File.Duration.Valid {
			t.Fatalf("file = %+v", body.Data.File)
		}
		// A chapter past the end is clamped into the duration, like movies.
		if body.Data.Chapters[1].StartTime != 7200 {
			t.Fatalf("chapter start = %d, want it clamped to 7200", body.Data.Chapters[1].StartTime)
		}
		assertOpenAPIExchange(t, "getShowEpisodeTechnicalDetails", request, response)
		bodies = append(bodies, body)
	}
	if bodies[0].Data.File.FileName != bodies[1].Data.File.FileName {
		t.Fatal("episodes of one file must report the same file")
	}
}

func TestEpisodeWatchProgressHandlers_ConformToOpenAPI(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Episode Progress User", "episode-progress@example.com", false)
	fixture := seedPlaybackEpisode(t, app)
	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	exchange := func(operationID string, req *http.Request) *httptest.ResponseRecorder {
		t.Helper()
		req.AddCookie(cookie)
		response := httptest.NewRecorder()
		app.Router.ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", operationID, response.Code, response.Body.String())
		}
		assertOpenAPIExchange(t, operationID, req, response)
		return response
	}

	progressPath := fmt.Sprintf("/api/shows/episodes/%d/watch-progress", fixture.Episode1)
	empty := exchange("getEpisodeWatchProgress", httptest.NewRequest(http.MethodGet, progressPath, nil))
	if !strings.Contains(empty.Body.String(), `"progress_sec":null`) {
		t.Fatalf("fresh progress = %s, want nulls", empty.Body.String())
	}

	updateBody := `{"progress_sec":120,"duration_sec":7200,"save_session_id":"11111111-1111-4111-8111-111111111111","save_sequence":1}`
	exchange("updateEpisodeWatchProgress", newOpenAPIJSONRequest(http.MethodPut, progressPath, updateBody))

	// The season listing carries the position for the same user, and only for
	// the episode that was played: the other episode of the combined file is
	// its own logical episode.
	listRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/%d/seasons/1/episodes", fixture.ShowID), nil)
	listRequest.AddCookie(cookie)
	listResponse := httptest.NewRecorder()
	app.Router.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("season list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	var list struct {
		Data struct {
			Episodes []struct {
				ID          int64           `json:"id"`
				ProgressSec sql.NullFloat64 `json:"progress_sec"`
				DurationSec sql.NullFloat64 `json:"duration_sec"`
				Watched     bool            `json:"watched"`
			} `json:"episodes"`
		} `json:"data"`
	}
	err := json.Unmarshal(listResponse.Body.Bytes(), &list)
	if err != nil {
		t.Fatalf("decode season list: %v", err)
	}
	if len(list.Data.Episodes) != 2 {
		t.Fatalf("episodes = %d, want 2", len(list.Data.Episodes))
	}
	first, second := list.Data.Episodes[0], list.Data.Episodes[1]
	if first.ID != fixture.Episode1 || !first.ProgressSec.Valid || first.ProgressSec.Float64 != 120 || first.DurationSec.Float64 != 7200 || first.Watched {
		t.Fatalf("played episode = %+v, want progress 120/7200 unwatched", first)
	}
	if second.ProgressSec.Valid || second.Watched {
		t.Fatalf("untouched episode = %+v, want no progress", second)
	}
	assertOpenAPIExchange(t, "getShowSeasonEpisodes", listRequest, listResponse)

	watchedPath := fmt.Sprintf("/api/shows/episodes/%d/watch-progress/watched", fixture.Episode2)
	watched := exchange("setEpisodeWatched", newOpenAPIJSONRequest(http.MethodPut, watchedPath, `{"watched":true}`))
	if !strings.Contains(watched.Body.String(), fmt.Sprintf(`"episode_id":%d`, fixture.Episode2)) {
		t.Fatalf("watched response = %s, want the episode id", watched.Body.String())
	}
	row, err := app.Queries.GetShowEpisodeWatchProgress(context.Background(), database.GetShowEpisodeWatchProgressParams{UserID: user.ID, EpisodeID: fixture.Episode2})
	if err != nil || !row.Watched {
		t.Fatalf("watched row = %+v, %v", row, err)
	}

	// Another user sees none of it.
	other := createTestUser(t, app, "Other Viewer", "other-viewer@example.com", false)
	otherRequest := httptest.NewRequest(http.MethodGet, progressPath, nil)
	otherRequest.AddCookie(newAuthSessionCookie(t, app, other.ID))
	otherResponse := httptest.NewRecorder()
	app.Router.ServeHTTP(otherResponse, otherRequest)
	if !strings.Contains(otherResponse.Body.String(), `"progress_sec":null`) {
		t.Fatalf("other user's progress = %s, want nulls", otherResponse.Body.String())
	}

	exchange("deleteEpisodeWatchProgress", httptest.NewRequest(http.MethodDelete, progressPath, nil))
	_, err = app.Queries.GetShowEpisodeWatchProgress(context.Background(), database.GetShowEpisodeWatchProgressParams{UserID: user.ID, EpisodeID: fixture.Episode1})
	if err != sql.ErrNoRows {
		t.Fatalf("progress after delete: err = %v, want no rows", err)
	}
}

func TestEpisodeWatchProgress_CompletionMarksWatchedAndCascades(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Completion User", "completion@example.com", false)
	fixture := seedPlaybackEpisode(t, app)
	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	progressPath := fmt.Sprintf("/api/shows/episodes/%d/watch-progress", fixture.Episode1)
	request := newOpenAPIJSONRequest(http.MethodPut, progressPath, `{"progress_sec":7150,"duration_sec":7200,"save_session_id":"11111111-1111-4111-8111-111111111111","save_sequence":1}`)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"watched":true`) {
		t.Fatalf("completion response = %d %s", response.Code, response.Body.String())
	}

	// An unknown episode is a 404 on the write path via the foreign key.
	missing := newOpenAPIJSONRequest(http.MethodPut, "/api/shows/episodes/999999/watch-progress", `{"progress_sec":10,"duration_sec":7200,"save_session_id":"11111111-1111-4111-8111-111111111111","save_sequence":1}`)
	missing.AddCookie(cookie)
	missingResponse := httptest.NewRecorder()
	app.Router.ServeHTTP(missingResponse, missing)
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("unknown episode write = %d %s, want 404", missingResponse.Code, missingResponse.Body.String())
	}

	// Deleting the episode's file prunes the episode and its progress row.
	_, err := app.Queries.DeleteMissingShowFile(context.Background(), database.DeleteMissingShowFileParams{ID: fixture.FileID, FilePath: fixture.FilePath})
	if err != nil {
		t.Fatalf("delete show file: %v", err)
	}
	err = app.Queries.PruneShowSeasonEpisodes(context.Background(), fixture.SeasonID)
	if err != nil {
		t.Fatalf("prune episodes: %v", err)
	}
	var remaining int
	err = app.DB.QueryRow(`SELECT COUNT(*) FROM show_episode_watch_progress`).Scan(&remaining)
	if err != nil || remaining != 0 {
		t.Fatalf("progress rows after prune = %d (%v), want 0", remaining, err)
	}
}

func TestStopEpisodeHLSSession_ConformsToOpenAPI(t *testing.T) {
	app := setupTestApp(t)

	user := createTestUser(t, app, "Stop User", "stop@example.com", false)
	fixture := seedPlaybackEpisode(t, app)
	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	media := episodeRef(fixture.Episode1)
	key := HLSSessionKey(media, helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, testPlaybackSessionID, 0, user.ID)
	app.HLSSessionCache.SetDefault(key, &HLSSession{Media: media, FileID: fixture.FileID, OwnerUserID: user.ID, PlaybackSession: testPlaybackSessionID, TempDir: t.TempDir()})
	// A movie session with the same numeric id and playback session is a
	// different media and must survive the episode stop.
	movieKey := HLSSessionKey(movieRef(fixture.Episode1), helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, testPlaybackSessionID, 0, user.ID)
	app.HLSSessionCache.SetDefault(movieKey, &HLSSession{Media: movieRef(fixture.Episode1), FileID: fixture.Episode1, OwnerUserID: user.ID, PlaybackSession: testPlaybackSessionID, TempDir: t.TempDir()})

	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/shows/episodes/%d/hls/session/stop?playback_session=%s", fixture.Episode1, testPlaybackSessionID), nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	assertOpenAPIExchange(t, "stopEpisodeHlsSession", request, response)

	if _, ok := app.HLSSessionCache.Get(key); ok {
		t.Fatal("episode session survived its stop")
	}
	if _, ok := app.HLSSessionCache.Get(movieKey); !ok {
		t.Fatal("movie session with the same id was stopped by the episode route")
	}
}

func TestHLSSessionKey_SeparatesMediaKinds(t *testing.T) {
	movieKey := HLSSessionKey(movieRef(9), helpers.HLS_PROFILE_REMUX, nil, nil, testPlaybackSessionID, 0, 1)
	episodeKey := HLSSessionKey(episodeRef(9), helpers.HLS_PROFILE_REMUX, nil, nil, testPlaybackSessionID, 0, 1)
	if movieKey == episodeKey {
		t.Fatalf("movie and episode keys collide: %q", movieKey)
	}
	if !strings.Contains(movieKey, ":movie:9:") || !strings.Contains(episodeKey, ":episode:9:") {
		t.Fatalf("keys do not name their kind: %q %q", movieKey, episodeKey)
	}
	session := &HLSSession{Media: episodeRef(9), OwnerUserID: 1}
	if canAccessPersonalHLSSession(session, movieRef(9), 1) {
		t.Fatal("a movie ref must not access an episode session")
	}
	if !canAccessPersonalHLSSession(session, episodeRef(9), 1) {
		t.Fatal("the episode ref must access its own session")
	}
}

func TestLoadHLSSourceForSession_Episode(t *testing.T) {
	app := setupTestApp(t)
	fixture := seedPlaybackEpisode(t, app)
	ctx := context.Background()

	for _, episodeID := range []int64{fixture.Episode1, fixture.Episode2} {
		source, start, err := app.loadHLSSourceForSession(ctx, episodeRef(episodeID), 30)
		if err != nil {
			t.Fatalf("episode %d: %v", episodeID, err)
		}
		if source.Ref != episodeRef(episodeID) || source.FileID != fixture.FileID || source.FilePath != fixture.FilePath || start != 30 {
			t.Fatalf("episode %d source = %+v start %d", episodeID, source, start)
		}
	}

	_, _, err := app.loadHLSSourceForSession(ctx, episodeRef(999999), 0)
	var notFound *hlsMediaNotFoundError
	if !errors.As(err, &notFound) || notFound.Media != episodeRef(999999) {
		t.Fatalf("missing episode error = %v, want a typed not-found", err)
	}

	_, err = app.DB.Exec(`UPDATE show_files SET duration = NULL WHERE id = ?`, fixture.FileID)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = app.loadHLSSourceForSession(ctx, episodeRef(fixture.Episode1), 0)
	var metadataErr *hlsMediaMetadataError
	if !errors.As(err, &metadataErr) {
		t.Fatalf("missing duration error = %v, want hlsMediaMetadataError", err)
	}
}

func TestEpisodeKeyframeAndRemuxPersistenceUseTheShowTables(t *testing.T) {
	app := setupTestApp(t)
	fixture := seedPlaybackEpisode(t, app)
	ctx := context.Background()
	source := episodePlaybackSource(t, app, fixture.Episode1)
	sibling := episodePlaybackSource(t, app, fixture.Episode2)
	video := &database.VideoStream{StreamIndex: 0, Codec: "h264"}

	fingerprint := keyframeIndexFingerprint(source, video)
	if fingerprint != keyframeIndexFingerprint(sibling, video) {
		t.Fatal("episodes of one file must share the file fingerprint")
	}
	app.setKeyframeIndex(source, 0, fingerprint, keyframeindex.Index{KeyframeSec: []float64{0, 4, 8}, DurationSec: 12})
	idx, ok := app.getKeyframeIndex(ctx, sibling, 0, fingerprint)
	if !ok || len(idx.KeyframeSec) != 3 {
		t.Fatalf("sibling episode did not read the file's keyframe index: %v %v", idx, ok)
	}
	var rows int
	err := app.DB.QueryRow(`SELECT COUNT(*) FROM show_keyframe_indexes WHERE file_id = ?`, fixture.FileID).Scan(&rows)
	if err != nil || rows != 1 {
		t.Fatalf("show_keyframe_indexes rows = %d (%v), want 1", rows, err)
	}
	if _, ok := app.getKeyframeIndex(ctx, playbackSourceFromMovie(database.Movie{ID: fixture.FileID}), 0, fingerprint); ok {
		t.Fatal("a movie with the file's id must not read the show table")
	}

	verdictFingerprint := remuxSafetyFingerprint(source, video, "7.0.2")
	app.setRemuxSafetyVerdict(source, 0, verdictFingerprint, false, "10-bit")
	verdict, ok := app.getRemuxSafetyVerdict(ctx, sibling, 0, verdictFingerprint)
	if !ok || verdict.Safe || verdict.Reason != "10-bit" {
		t.Fatalf("sibling verdict = %+v %v", verdict, ok)
	}
	err = app.DB.QueryRow(`SELECT COUNT(*) FROM show_remux_safety_verdicts WHERE file_id = ?`, fixture.FileID).Scan(&rows)
	if err != nil || rows != 1 {
		t.Fatalf("show_remux_safety_verdicts rows = %d (%v), want 1", rows, err)
	}
}

func TestInvalidateCommittedShowFile_EvictsOnlyThatFile(t *testing.T) {
	app := setupTestApp(t)
	fixture := seedPlaybackEpisode(t, app)

	episodeKey := HLSSessionKey(episodeRef(fixture.Episode1), helpers.HLS_PROFILE_REMUX, nil, nil, testPlaybackSessionID, 0, 1)
	otherEpisodeKey := HLSSessionKey(episodeRef(77), helpers.HLS_PROFILE_REMUX, nil, nil, testPlaybackSessionID, 0, 1)
	movieKey := HLSSessionKey(movieRef(fixture.FileID), helpers.HLS_PROFILE_REMUX, nil, nil, testPlaybackSessionID, 0, 1)
	app.HLSSessionCache.SetDefault(episodeKey, &HLSSession{Media: episodeRef(fixture.Episode1), FileID: fixture.FileID, TempDir: t.TempDir()})
	app.HLSSessionCache.SetDefault(otherEpisodeKey, &HLSSession{Media: episodeRef(77), FileID: fixture.FileID + 1000, TempDir: t.TempDir()})
	app.HLSSessionCache.SetDefault(movieKey, &HLSSession{Media: movieRef(fixture.FileID), FileID: fixture.FileID, TempDir: t.TempDir()})
	app.SubtitleVTTCache.Set(helpers.SubtitleCacheKey("episode", fixture.FileID, 2), []byte("WEBVTT\n"), subtitleCacheTTL)
	app.SubtitleVTTCache.Set(helpers.SubtitleCacheKey("movie", fixture.FileID, 2), []byte("WEBVTT\n"), subtitleCacheTTL)
	_, err := app.episodeStreamFile(context.Background(), fixture.Episode1)
	if err != nil {
		t.Fatalf("warm stream file cache: %v", err)
	}

	app.invalidateCommittedShowFile(fixture.FileID, []int64{fixture.Episode1, fixture.Episode2})
	app.Wait.Wait()

	if _, ok := app.HLSSessionCache.Get(episodeKey); ok {
		t.Fatal("episode session of the rescanned file survived")
	}
	if _, ok := app.HLSSessionCache.Get(otherEpisodeKey); !ok {
		t.Fatal("episode session of another file was evicted")
	}
	if _, ok := app.HLSSessionCache.Get(movieKey); !ok {
		t.Fatal("movie session with the same file id was evicted")
	}
	if _, ok := app.SubtitleVTTCache.Get(helpers.SubtitleCacheKey("episode", fixture.FileID, 2)); ok {
		t.Fatal("episode subtitle cache survived")
	}
	if _, ok := app.SubtitleVTTCache.Get(helpers.SubtitleCacheKey("movie", fixture.FileID, 2)); !ok {
		t.Fatal("movie subtitle cache with the same file id was evicted")
	}
	if _, ok := app.StreamFileCache.get(episodeStreamFileKey(fixture.Episode1)); ok {
		t.Fatal("episode stream file cache survived")
	}
}

func serveEpisodeSubtitleWebVTT(t *testing.T, app *Application, userID int64, episodeID int64, trackIndex string, query string) *httptest.ResponseRecorder {
	t.Helper()
	router := authenticatedRouter(t, app, userID)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/shows/episodes/%d/subtitles/%s/web.vtt%s", episodeID, trackIndex, query), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestEpisodeSubtitleWebVTT_ExtractsSharesTheFileCacheAndRejectsBitmaps(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake
	fixture := seedPlaybackEpisode(t, app)

	first := serveEpisodeSubtitleWebVTT(t, app, viewer.ID, fixture.Episode1, "0", "")
	if first.Code != http.StatusOK || first.Body.String() != "WEBVTT\n" {
		t.Fatalf("first = %d %q", first.Code, first.Body.String())
	}
	// The sibling episode is the same file, so its track is already cached.
	second := serveEpisodeSubtitleWebVTT(t, app, viewer.ID, fixture.Episode2, "0", "")
	if second.Code != http.StatusOK || fake.SubtitleCallCount() != 1 {
		t.Fatalf("sibling = %d, extractor calls = %d, want one extraction", second.Code, fake.SubtitleCallCount())
	}
	if _, ok := app.SubtitleVTTCache.Get(helpers.SubtitleCacheKey("episode", fixture.FileID, testSubtitleStreamIndex)); !ok {
		t.Fatal("cache is not keyed on the episode's file")
	}

	shifted := serveEpisodeSubtitleWebVTT(t, app, viewer.ID, fixture.Episode1, "0", "?start=-1")
	if shifted.Code != http.StatusBadRequest {
		t.Fatalf("negative start = %d, want 400", shifted.Code)
	}
	outOfRange := serveEpisodeSubtitleWebVTT(t, app, viewer.ID, fixture.Episode1, "3", "")
	if outOfRange.Code != http.StatusBadRequest {
		t.Fatalf("track out of range = %d, want 400", outOfRange.Code)
	}

	_, err := app.DB.Exec(`UPDATE show_subtitles SET codec = 'hdmv_pgs_subtitle' WHERE file_id = ?`, fixture.FileID)
	if err != nil {
		t.Fatal(err)
	}
	bitmap := serveEpisodeSubtitleWebVTT(t, app, viewer.ID, fixture.Episode1, "0", "")
	if bitmap.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("bitmap = %d, want 415", bitmap.Code)
	}
}

func TestEpisodeHLSManifest_ValidatesParamsAgainstTheEpisodeRoute(t *testing.T) {
	app := setupTestApp(t)
	fixture := seedPlaybackEpisode(t, app)
	handler := authenticatedRouter(t, app, 42)

	for _, tc := range []struct {
		name   string
		path   string
		status int
		body   string
	}{
		{"malformed id", "/api/shows/episodes/abc/hls/720p_3mbps/playlist.m3u8?start=0&playback_session=" + testPlaybackSessionID, http.StatusBadRequest, "invalid episode id"},
		{"missing episode", "/api/shows/episodes/999999/hls/720p_3mbps/playlist.m3u8?audio_track=0&start=0&playback_session=" + testPlaybackSessionID, http.StatusNotFound, "episode not found"},
		{"segment before manifest", fmt.Sprintf("/api/shows/episodes/%d/hls/720p_3mbps/init.mp4?audio_track=0&start=0&playback_session=%s", fixture.Episode1, testPlaybackSessionID), http.StatusNotFound, "request the manifest first"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if recorder.Code != tc.status || !strings.Contains(recorder.Body.String(), tc.body) {
				t.Fatalf("response = %d %s, want %d %q", recorder.Code, recorder.Body.String(), tc.status, tc.body)
			}
		})
	}
}

// The show stream rows reach FFmpeg through the movie row types
// (showVideoStreamAsPlayback and friends), so a successful session is the
// only thing that proves the column mapping: a mis-mapped stream index or
// audio row would start FFmpeg on the wrong track and nothing earlier would
// notice.
func TestEpisodeHLSManifest_StartsFFmpegFromTheShowStreams(t *testing.T) {
	app := setupTestApp(t)
	ffmpegRunner := &fakeFFmpeg{plans: []fakeFFmpegRunPlan{hlsRunPlan(transcodeFixture), hlsRunPlan(transcodeFixture)}}
	app.FFmpeg = ffmpegRunner
	fixture := seedPlaybackEpisode(t, app)
	userID := int64(42)
	handler := authenticatedRouter(t, app, userID)

	// Both episodes of the combined file start a session on that one file.
	for _, episodeID := range []int64{fixture.Episode1, fixture.Episode2} {
		manifestURL := fmt.Sprintf(
			"/api/shows/episodes/%d/hls/%s/playlist.m3u8?audio_track=0&playback_session=%s&start=0",
			episodeID, helpers.HLS_PROFILE_720P_3MBPS, testPlaybackSessionID,
		)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, manifestURL, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("episode %d manifest = %d %s", episodeID, recorder.Code, recorder.Body.String())
		}

		key := HLSSessionKey(episodeRef(episodeID), helpers.HLS_PROFILE_720P_3MBPS, testIntPtr(0), nil, testPlaybackSessionID, 0, userID)
		raw, ok := app.HLSSessionCache.Get(key)
		session, _ := raw.(*HLSSession)
		if !ok || session == nil {
			t.Fatalf("episode %d: no session cached under %q", episodeID, key)
		}
		defer cleanupHLSSession(session)
		if session.Media != episodeRef(episodeID) || session.FileID != fixture.FileID {
			t.Fatalf("episode %d session identity = %s file %d, want %s file %d", episodeID, session.Media, session.FileID, episodeRef(episodeID), fixture.FileID)
		}
	}

	calls := ffmpegRunner.Calls()
	if len(calls) != 2 {
		t.Fatalf("FFmpeg calls = %d, want one per episode", len(calls))
	}
	for i, call := range calls {
		if call.SourcePath != fixture.FilePath || call.Profile != helpers.HLS_PROFILE_720P_3MBPS {
			t.Fatalf("call %d source/profile = %q/%q, want the show file and %s", i, call.SourcePath, call.Profile, helpers.HLS_PROFILE_720P_3MBPS)
		}
		// The fixture's video is stream 0 and its audio stream 1; the ordinal
		// audio_track=0 must map to the stored audio row's ffprobe index.
		if call.VideoStreamIndex != 0 || call.AudioStreamIndex != 1 {
			t.Fatalf("call %d stream indexes = video %d audio %d, want 0 and 1", i, call.VideoStreamIndex, call.AudioStreamIndex)
		}
		if !call.CopyAudio {
			t.Fatalf("call %d did not copy the AAC-LC audio row", i)
		}
	}
}

// An episode with two copies on disk resolves to the lowest file id every
// time, on both the HLS/subtitle path (loadPlaybackSource) and the direct
// stream path (episodeStreamFile).
func TestGetShowFileForEpisode_PicksTheLowestFileIDForDuplicateCopies(t *testing.T) {
	app := setupTestApp(t)
	fixture := seedPlaybackEpisode(t, app)
	ctx := context.Background()

	copyPath := strings.TrimSuffix(fixture.FilePath, ".mkv") + " (copy).mkv"
	duplicate, err := app.Queries.UpsertShowFile(ctx, database.UpsertShowFileParams{
		SeasonID:  fixture.SeasonID,
		FilePath:  copyPath,
		FileName:  filepath.Base(copyPath),
		Size:      2_000_000,
		Container: "mkv",
		MimeType:  helpers.VideoMimeTypes["mkv"],
		Duration:  sql.NullFloat64{Float64: 7200, Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert duplicate file: %v", err)
	}
	if duplicate.ID <= fixture.FileID {
		t.Fatalf("duplicate id %d is not above the original %d", duplicate.ID, fixture.FileID)
	}
	err = app.Queries.LinkShowEpisodeFile(ctx, database.LinkShowEpisodeFileParams{
		EpisodeID:    fixture.Episode1,
		FileID:       duplicate.ID,
		SeasonID:     fixture.SeasonID,
		EpisodeOrder: 0,
	})
	if err != nil {
		t.Fatalf("link duplicate file: %v", err)
	}

	source := episodePlaybackSource(t, app, fixture.Episode1)
	if source.FileID != fixture.FileID || source.FilePath != fixture.FilePath {
		t.Fatalf("playback source = file %d %q, want the lower id %d %q", source.FileID, source.FilePath, fixture.FileID, fixture.FilePath)
	}
	stream, err := app.episodeStreamFile(ctx, fixture.Episode1)
	if err != nil {
		t.Fatalf("episode stream file: %v", err)
	}
	if stream.Path != fixture.FilePath {
		t.Fatalf("direct stream path = %q, want %q", stream.Path, fixture.FilePath)
	}
}
