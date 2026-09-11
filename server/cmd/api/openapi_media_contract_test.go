package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

func TestMediaResponsesConformToOpenAPI(t *testing.T) {
	app := setupSessionTestApp(t)
	defer app.DB.Close()
	ownerID, movieID := createTestUserAndMovie(t, app)
	payload := []byte("0123456789abcdef")
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "poster.png")
	err := os.WriteFile(mediaPath, payload, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	modTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	err = os.Chtimes(mediaPath, modTime, modTime)
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.DB.Exec(`UPDATE movies SET file_path = ?, file_name = 'movie.mp4', container = 'mp4', mime_type = 'video/mp4' WHERE id = ?`, mediaPath, movieID)
	if err != nil {
		t.Fatal(err)
	}
	track := seedStreamTestTrack(t, app, sql.NullInt64{}, payload)
	err = os.Chtimes(track.FilePath, modTime, modTime)
	if err != nil {
		t.Fatal(err)
	}
	app.SetSettings(&database.Setting{StaticDir: dir})
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	hlsRoom := createTestRoomWithMode(t, app, ownerID, movieID, helpers.HLS_PROFILE_720P_3MBPS)
	addMembersToRoom(t, app, hlsRoom.ID, ownerID)
	for _, name := range []string{"init.mp4", "segment_0.m4s"} {
		path := filepath.Join(dir, name)
		err = os.WriteFile(path, payload, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		err = os.Chtimes(path, modTime, modTime)
		if err != nil {
			t.Fatal(err)
		}
	}
	audioTrack := 0
	personalKey := HLSSessionKey(movieID, helpers.HLS_PROFILE_720P_3MBPS, &audioTrack, nil, testPlaybackSessionID, 0, ownerID)
	app.HLSSessionCache.SetDefault(personalKey, &HLSSession{MovieID: movieID, OwnerUserID: ownerID, PlaybackSession: testPlaybackSessionID, TempDir: dir, TempFileSegments: true, EffectiveProfile: helpers.HLS_PROFILE_720P_3MBPS})
	app.HLSSessionCache.SetDefault(RoomHLSSessionKey(hlsRoom.ID), &HLSSession{MovieID: movieID, TempDir: dir, TempFileSegments: true, EffectiveProfile: helpers.HLS_PROFILE_720P_3MBPS})
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, ownerID)
	server := httptest.NewServer(app.Router)
	defer server.Close()

	// Use the HTTP server so HEAD error bodies are suppressed at the real wire boundary.
	exchange := func(t *testing.T, method, path string, headers http.Header) (*http.Request, *httptest.ResponseRecorder) {
		t.Helper()
		request, requestErr := http.NewRequest(method, server.URL+path, nil)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		request.AddCookie(cookie)
		for name, values := range headers {
			request.Header[name] = values
		}
		response, responseErr := server.Client().Do(request)
		if responseErr != nil {
			t.Fatal(responseErr)
		}
		defer response.Body.Close()
		recorder := httptest.NewRecorder()
		for name, values := range response.Header {
			recorder.Header()[name] = values
		}
		recorder.WriteHeader(response.StatusCode)
		_, copyErr := io.Copy(recorder.Body, response.Body)
		if copyErr != nil {
			t.Fatal(copyErr)
		}
		// The contract uses a relative, same-origin server URL.
		request.URL.Scheme = ""
		request.URL.Host = ""
		return request, recorder
	}
	type mediaEndpoint struct {
		operation, path, contentType string
		etag, head                   bool
	}
	endpoints := []mediaEndpoint{
		{"streamMovie", fmt.Sprintf("/api/movies/%d/stream", movieID), "video/mp4", true, true},
		{"streamTrack", fmt.Sprintf("/api/music/tracks/%d/stream", track.ID), "audio/flac", true, true},
		{"streamWatchRoomMovie", fmt.Sprintf("/api/watch-rooms/%d/stream", room.ID), "video/mp4", true, true},
		{"serveStaticFiles", "/api/static/poster.png", "image/png", false, false},
	}
	for _, filename := range []string{"init.mp4", "segment_0.m4s"} {
		endpoints = append(endpoints, mediaEndpoint{
			"hlsSegment", fmt.Sprintf("/api/movies/%d/hls/%s/%s?audio_track=0&playback_session=%s&start=0", movieID, helpers.HLS_PROFILE_720P_3MBPS, filename, testPlaybackSessionID), "video/mp4", false, false,
		}, mediaEndpoint{
			"watchRoomHLSSegment", fmt.Sprintf("/api/watch-rooms/%d/hls/%s?audio_track=0", hlsRoom.ID, filename), "video/mp4", false, false,
		})
	}
	for _, endpoint := range endpoints {
		t.Run(endpoint.operation+"/"+filepath.Base(endpoint.path), func(t *testing.T) {
			request, full := exchange(t, http.MethodGet, endpoint.path, nil)
			if full.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", full.Code, full.Body.String())
			}
			if full.Header().Get("Content-Type") != endpoint.contentType || !bytes.Equal(full.Body.Bytes(), payload) {
				t.Fatalf("unexpected file response: %v %q", full.Header(), full.Body.String())
			}
			if full.Header().Get("Last-Modified") != modTime.Format(http.TimeFormat) {
				t.Fatalf("Last-Modified = %q", full.Header().Get("Last-Modified"))
			}
			etag := full.Header().Get("ETag")
			if (etag != "") != endpoint.etag {
				t.Fatalf("ETag = %q, expected present = %v", etag, endpoint.etag)
			}
			assertOpenAPIExchange(t, endpoint.operation, request, full)
			type mediaCase struct {
				name    string
				headers http.Header
				status  int
				body    string
			}
			cases := []mediaCase{
				{"single range", http.Header{"Range": {"bytes=0-3"}}, 206, "0123"},
				{"multipart", http.Header{"Range": {"bytes=0-1,4-5"}}, 206, ""},
				{"modified since", http.Header{"If-Modified-Since": {modTime.Format(http.TimeFormat)}}, 304, ""},
				{"none match wildcard", http.Header{"If-None-Match": {"*"}}, 304, ""},
				{"match fails", http.Header{"If-Match": {`"different"`}}, 412, ""},
				{"unmodified since fails", http.Header{"If-Unmodified-Since": {modTime.Add(-time.Hour).Format(http.TimeFormat)}}, 412, ""},
				{"if range date", http.Header{"Range": {"bytes=0-3"}, "If-Range": {modTime.Format(http.TimeFormat)}}, 206, "0123"},
				{"if range stale", http.Header{"Range": {"bytes=0-3"}, "If-Range": {`"different"`}}, 200, string(payload)},
				{"malformed range", http.Header{"Range": {"bytes=wat"}}, 416, "invalid range\n"},
				{"unsatisfiable range", http.Header{"Range": {"bytes=100-"}}, 416, "invalid range: failed to overlap\n"},
			}
			if endpoint.etag {
				cases = append(cases, mediaCase{"etag revalidation", http.Header{"If-None-Match": {etag}}, 304, ""}, mediaCase{"if range etag", http.Header{"Range": {"bytes=0-3"}, "If-Range": {etag}}, 206, "0123"})
			}
			methods := []string{http.MethodGet}
			if endpoint.head {
				methods = append(methods, http.MethodHead)
			}
			for _, method := range methods {
				for _, tc := range cases {
					t.Run(method+"/"+tc.name, func(t *testing.T) {
						request, response := exchange(t, method, endpoint.path, tc.headers)
						if response.Code != tc.status {
							t.Fatalf("status = %d, want %d: %s", response.Code, tc.status, response.Body.String())
						}
						operation := endpoint.operation
						if method == http.MethodHead {
							operation += "Head"
							if response.Body.Len() != 0 {
								t.Fatal("HEAD returned a body")
							}
						} else if tc.name == "multipart" {
							mediaType, params, parseErr := mime.ParseMediaType(response.Header().Get("Content-Type"))
							if parseErr != nil || mediaType != "multipart/byteranges" {
								t.Fatalf("multipart Content-Type = %q: %v", mediaType, parseErr)
							}
							if response.Header().Get("Content-Range") != "" {
								t.Fatal("multipart response has top-level Content-Range")
							}
							reader := multipart.NewReader(bytes.NewReader(response.Body.Bytes()), params["boundary"])
							for index, want := range []string{"01", "45"} {
								part, partErr := reader.NextPart()
								if partErr != nil {
									t.Fatal(partErr)
								}
								data, readErr := io.ReadAll(part)
								if readErr != nil {
									t.Fatal(readErr)
								}
								wantRange := fmt.Sprintf("bytes %d-%d/16", index*4, index*4+1)
								if string(data) != want || part.Header.Get("Content-Type") != endpoint.contentType || part.Header.Get("Content-Range") != wantRange {
									t.Fatalf("unexpected range part: %v %q", part.Header, data)
								}
							}
							_, partErr := reader.NextPart()
							if partErr != io.EOF {
								t.Fatalf("unexpected extra part: %v", partErr)
							}
						} else if response.Body.String() != tc.body {
							t.Fatalf("body = %q, want %q", response.Body.String(), tc.body)
						}
						if tc.status == 416 {
							if response.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
								t.Fatal("range error is not plain text")
							}
							wantRange := ""
							if tc.name == "unsatisfiable range" {
								wantRange = "bytes */16"
							}
							if response.Header().Get("Content-Range") != wantRange {
								t.Fatalf("Content-Range = %q", response.Header().Get("Content-Range"))
							}
						}
						assertOpenAPIExchange(t, operation, request, response)
					})
				}
				if method == http.MethodHead {
					request, response := exchange(t, method, endpoint.path, nil)
					if response.Code != 200 || response.Body.Len() != 0 || response.Header().Get("Content-Length") != strconv.Itoa(len(payload)) {
						t.Fatalf("HEAD response = %d %v %q", response.Code, response.Header(), response.Body.String())
					}
					assertOpenAPIExchange(t, endpoint.operation+"Head", request, response)
					request, response = exchange(t, method, endpoint.path, http.Header{"Authorization": {"Bearer igd_invalid"}})
					if response.Code != 401 || response.Body.Len() != 0 || response.Header().Get("Content-Type") != "application/json" {
						t.Fatalf("HEAD error = %d %v %q", response.Code, response.Header(), response.Body.String())
					}
					assertOpenAPIExchange(t, endpoint.operation+"Head", request, response)
				}
			}
		})
	}
}

func TestNonJSONRoutesMiddlewareErrorsConformToOpenAPI(t *testing.T) {
	app := setupSessionTestApp(t)
	defer app.DB.Close()
	app.InitRouter()
	endpoints := []struct{ path, operation string }{
		{"/api/static/poster.png", "serveStaticFiles"},
		{"/api/tmdb/images/w500/poster.jpg", "proxyTmdbImage"},
		{"/api/youtube/thumbnails/video", "proxyYouTubeThumbnail"},
		{"/api/movies/1/stream", "streamMovie"},
		{"/api/music/tracks/1/stream", "streamTrack"},
		{"/api/movies/1/subtitles/0/web.vtt", "subtitleWebVTT"},
		{"/api/movies/1/hls/720p_3mbps/playlist.m3u8?start=0&playback_session=" + testPlaybackSessionID, "hlsManifest"},
		{"/api/movies/1/hls/720p_3mbps/init.mp4?start=0&playback_session=" + testPlaybackSessionID, "hlsSegment"},
		{"/api/watch-rooms/1/stream", "streamWatchRoomMovie"},
		{"/api/watch-rooms/1/hls/playlist.m3u8", "watchRoomHLSManifest"},
		{"/api/watch-rooms/1/hls/init.mp4", "watchRoomHLSSegment"},
		{"/api/watch-rooms/1/ws", "watchRoomWebSocket"},
	}
	for _, status := range []int{http.StatusUnauthorized, http.StatusInternalServerError} {
		if status == http.StatusInternalServerError {
			err := app.DB.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
		for _, endpoint := range endpoints {
			t.Run(fmt.Sprintf("%s/%d", endpoint.operation, status), func(t *testing.T) {
				request := httptest.NewRequest(http.MethodGet, endpoint.path, nil)
				request.Header.Set("Authorization", "Bearer igd_invalid")
				response := httptest.NewRecorder()
				app.Router.ServeHTTP(response, request)
				if response.Code != status || response.Header().Get("Content-Type") != "application/json" {
					t.Fatalf("response = %d %v %s", response.Code, response.Header(), response.Body.String())
				}
				assertOpenAPIExchange(t, endpoint.operation, request, response)
			})
		}
	}
}
