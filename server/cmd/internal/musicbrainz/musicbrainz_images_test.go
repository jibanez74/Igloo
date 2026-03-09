package musicbrainz

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func setupAudioDBMockServer(t *testing.T, handler http.HandlerFunc) *musicBrainzClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := New().(*musicBrainzClient)
	client.audioDBBaseURL = server.URL
	client.audioDBLastReq = time.Time{}
	return client
}

func audioDBHandler(artists []audioDBArtist) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(audioDBResponse{Artists: artists})
	}
}

func TestGetArtistImageURL_Success(t *testing.T) {
	client := setupAudioDBMockServer(t, audioDBHandler([]audioDBArtist{
		{ArtistThumb: "https://www.theaudiodb.com/images/media/artist/thumb/radiohead.jpg"},
	}))

	url, err := client.GetArtistImageURL("a74b1b7f-71a5-4011-9441-d0b5e4122711")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "https://www.theaudiodb.com/images/media/artist/thumb/radiohead.jpg"
	if url != expected {
		t.Errorf("expected URL '%s', got '%s'", expected, url)
	}
}

func TestGetArtistImageURL_EmptyID(t *testing.T) {
	client := setupAudioDBMockServer(t, audioDBHandler(nil))

	_, err := client.GetArtistImageURL("")
	if err == nil {
		t.Fatal("expected error for empty musicbrainz ID")
	}
}

func TestGetArtistImageURL_NoResults(t *testing.T) {
	client := setupAudioDBMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(audioDBResponse{Artists: nil})
	})

	_, err := client.GetArtistImageURL("some-id")
	if err == nil {
		t.Fatal("expected error for no results")
	}
}

func TestGetArtistImageURL_NoThumb(t *testing.T) {
	client := setupAudioDBMockServer(t, audioDBHandler([]audioDBArtist{
		{ArtistThumb: ""},
	}))

	_, err := client.GetArtistImageURL("some-id")
	if err == nil {
		t.Fatal("expected error for empty thumb URL")
	}
}

func TestGetArtistImageURL_HTTPError(t *testing.T) {
	client := setupAudioDBMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	_, err := client.GetArtistImageURL("some-id")
	if err == nil {
		t.Fatal("expected error for HTTP 503")
	}
}

func TestGetArtistImageURL_Cache(t *testing.T) {
	var requestCount atomic.Int32
	client := setupAudioDBMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		json.NewEncoder(w).Encode(audioDBResponse{
			Artists: []audioDBArtist{{ArtistThumb: "https://example.com/thumb.jpg"}},
		})
	})

	url1, err := client.GetArtistImageURL("test-id")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	url2, err := client.GetArtistImageURL("test-id")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if url1 != url2 {
		t.Errorf("cache returned different URL: '%s' vs '%s'", url1, url2)
	}

	if requestCount.Load() != 1 {
		t.Errorf("expected 1 request (cache hit on second), got %d", requestCount.Load())
	}
}

func TestGetArtistImageURL_NegativeResultCached(t *testing.T) {
	var requestCount atomic.Int32
	client := setupAudioDBMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		json.NewEncoder(w).Encode(audioDBResponse{Artists: nil})
	})

	_, err := client.GetArtistImageURL("no-image-id")
	if err == nil {
		t.Fatal("expected error on first call for artist with no image")
	}

	_, err = client.GetArtistImageURL("no-image-id")
	if err == nil {
		t.Fatal("expected error on cached negative (empty image URL)")
	}

	if requestCount.Load() != 1 {
		t.Errorf("expected 1 API request (cached negative on second call), got %d", requestCount.Load())
	}
}

func TestGetArtistImageURL_MalformedJSON(t *testing.T) {
	client := setupAudioDBMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{invalid json"))
	})

	_, err := client.GetArtistImageURL("some-id")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestGetArtistImageURL_BuildsCorrectEndpoint(t *testing.T) {
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path + "?" + r.URL.RawQuery
		json.NewEncoder(w).Encode(audioDBResponse{
			Artists: []audioDBArtist{{ArtistThumb: "https://example.com/thumb.jpg"}},
		})
	}))
	t.Cleanup(server.Close)

	client := New().(*musicBrainzClient)
	client.audioDBBaseURL = server.URL
	client.audioDBLastReq = time.Time{}

	client.GetArtistImageURL("abc-123")

	expected := "/2/artist-mb.php?i=abc-123"
	if capturedPath != expected {
		t.Errorf("expected path '%s', got '%s'", expected, capturedPath)
	}
}

func TestAudioDBRateLimiterIndependent(t *testing.T) {
	mbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(artistSearchResponse{
			Artists: []artistJSON{{ID: "id-1", Name: "A"}},
		})
	}))
	t.Cleanup(mbServer.Close)

	audioDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(audioDBResponse{
			Artists: []audioDBArtist{{ArtistThumb: "https://example.com/thumb.jpg"}},
		})
	}))
	t.Cleanup(audioDBServer.Close)

	client := New().(*musicBrainzClient)
	client.baseURL = mbServer.URL
	client.audioDBBaseURL = audioDBServer.URL
	client.lastRequest = time.Time{}
	client.audioDBLastReq = time.Time{}

	client.SearchArtistByName("RateLimitTest")

	start := time.Now()
	_, err := client.GetArtistImageURL("rate-limit-test-id")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if elapsed > 500*time.Millisecond {
		t.Errorf("AudioDB call should not be delayed by MusicBrainz rate limiter, but took %v", elapsed)
	}
}

func TestImageCacheEviction(t *testing.T) {
	client := New().(*musicBrainzClient)

	for i := 0; i < 105; i++ {
		client.setImage(
			string(rune('A'+i%26))+string(rune('0'+i/26)),
			"https://example.com/thumb.jpg",
		)
	}

	if len(client.imageCache) > 100 {
		t.Errorf("expected cache size <= 100, got %d", len(client.imageCache))
	}
}

// --- Album Image (TheAudioDB) Tests ---

func TestGetAlbumImageURL_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(audioDBAlbumResponse{
			Albums: []audioDBAlbum{{AlbumThumb: "https://www.theaudiodb.com/images/media/album/thumb/test.jpg"}},
		})
	}))
	t.Cleanup(server.Close)

	client := New().(*musicBrainzClient)
	client.audioDBBaseURL = server.URL
	client.audioDBLastReq = time.Time{}

	url, err := client.GetAlbumImageURL("rg-abc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "https://www.theaudiodb.com/images/media/album/thumb/test.jpg"
	if url != expected {
		t.Errorf("expected URL '%s', got '%s'", expected, url)
	}
}

func TestGetAlbumImageURL_EmptyID(t *testing.T) {
	client := New().(*musicBrainzClient)
	_, err := client.GetAlbumImageURL("")
	if err == nil {
		t.Fatal("expected error for empty release group ID")
	}
}

func TestGetAlbumImageURL_NoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(audioDBAlbumResponse{Albums: nil})
	}))
	t.Cleanup(server.Close)

	client := New().(*musicBrainzClient)
	client.audioDBBaseURL = server.URL
	client.audioDBLastReq = time.Time{}

	_, err := client.GetAlbumImageURL("rg-no-image")
	if err == nil {
		t.Fatal("expected error for no results")
	}
}

func TestGetAlbumImageURL_Cache(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		json.NewEncoder(w).Encode(audioDBAlbumResponse{
			Albums: []audioDBAlbum{{AlbumThumb: "https://example.com/album.jpg"}},
		})
	}))
	t.Cleanup(server.Close)

	client := New().(*musicBrainzClient)
	client.audioDBBaseURL = server.URL
	client.audioDBLastReq = time.Time{}

	url1, err := client.GetAlbumImageURL("rg-cache-test")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	url2, err := client.GetAlbumImageURL("rg-cache-test")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if url1 != url2 {
		t.Errorf("cache returned different URL: '%s' vs '%s'", url1, url2)
	}

	if requestCount.Load() != 1 {
		t.Errorf("expected 1 request (cache hit on second), got %d", requestCount.Load())
	}
}

func TestGetAlbumImageURL_NegativeResultCached(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		json.NewEncoder(w).Encode(audioDBAlbumResponse{Albums: nil})
	}))
	t.Cleanup(server.Close)

	client := New().(*musicBrainzClient)
	client.audioDBBaseURL = server.URL
	client.audioDBLastReq = time.Time{}

	_, err := client.GetAlbumImageURL("rg-neg-test")
	if err == nil {
		t.Fatal("expected error on first call for album with no image")
	}

	_, err = client.GetAlbumImageURL("rg-neg-test")
	if err == nil {
		t.Fatal("expected error on cached negative (empty album image URL)")
	}

	if requestCount.Load() != 1 {
		t.Errorf("expected 1 API request (cached negative on second call), got %d", requestCount.Load())
	}
}

func TestGetAlbumImageURL_BuildsCorrectEndpoint(t *testing.T) {
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path + "?" + r.URL.RawQuery
		json.NewEncoder(w).Encode(audioDBAlbumResponse{
			Albums: []audioDBAlbum{{AlbumThumb: "https://example.com/album.jpg"}},
		})
	}))
	t.Cleanup(server.Close)

	client := New().(*musicBrainzClient)
	client.audioDBBaseURL = server.URL
	client.audioDBLastReq = time.Time{}

	client.GetAlbumImageURL("rg-endpoint-test")

	expected := "/2/album-mb.php?i=rg-endpoint-test"
	if capturedPath != expected {
		t.Errorf("expected path '%s', got '%s'", expected, capturedPath)
	}
}

func TestClearAllCaches_IncludesImageCache(t *testing.T) {
	client := New().(*musicBrainzClient)
	client.setImage("test-key", "https://example.com/thumb.jpg")

	if _, exists := client.getImage("test-key"); !exists {
		t.Fatal("expected image to be cached before clear")
	}

	client.ClearAllCaches()

	if _, exists := client.getImage("test-key"); exists {
		t.Fatal("expected image cache to be cleared")
	}
}
