package musicbrainz

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"igloo/cmd/internal/helpers"
)

func setupMockServer(t *testing.T, handler http.HandlerFunc) *musicBrainzClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := New().(*musicBrainzClient)
	client.baseURL = server.URL
	client.lastRequest = time.Time{}
	return client
}

func artistHandler(artists []artistJSON) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(artistSearchResponse{Artists: artists})
	}
}

func releaseGroupHandler(rgs []releaseGroupJSON) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releaseGroupSearchResponse{ReleaseGroups: rgs})
	}
}

// --- Artist Search Tests ---

func TestSearchArtistByName_Success(t *testing.T) {
	client := setupMockServer(t, artistHandler([]artistJSON{
		{
			ID:             "a74b1b7f-71a5-4011-9441-d0b5e4122711",
			Name:           "Radiohead",
			SortName:       "Radiohead",
			Type:           "Group",
			Country:        "GB",
			Disambiguation: "UK rock band",
		},
	}))

	result, err := client.SearchArtistByName("Radiohead")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.MusicBrainzID != "a74b1b7f-71a5-4011-9441-d0b5e4122711" {
		t.Errorf("expected MBID 'a74b1b7f-71a5-4011-9441-d0b5e4122711', got '%s'", result.MusicBrainzID)
	}
	if result.Name != "Radiohead" {
		t.Errorf("expected Name 'Radiohead', got '%s'", result.Name)
	}
	if result.SortName != "Radiohead" {
		t.Errorf("expected SortName 'Radiohead', got '%s'", result.SortName)
	}
	if result.Country != "GB" {
		t.Errorf("expected Country 'GB', got '%s'", result.Country)
	}
	if result.Type != "Group" {
		t.Errorf("expected Type 'Group', got '%s'", result.Type)
	}
	if result.Disambiguation != "UK rock band" {
		t.Errorf("expected Disambiguation 'UK rock band', got '%s'", result.Disambiguation)
	}
}

func TestSearchArtistByName_EmptyName(t *testing.T) {
	client := setupMockServer(t, artistHandler(nil))

	_, err := client.SearchArtistByName("")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestSearchArtistByName_NoResults(t *testing.T) {
	client := setupMockServer(t, artistHandler([]artistJSON{}))

	_, err := client.SearchArtistByName("nonexistentartist12345")
	if err == nil {
		t.Fatal("expected error for no results")
	}
}

func TestSearchArtistByName_HTTPError(t *testing.T) {
	client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	_, err := client.SearchArtistByName("Radiohead")
	if err == nil {
		t.Fatal("expected error for HTTP 503")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("expected error to contain '503', got '%s'", err.Error())
	}
}

func TestSearchArtistByName_MalformedJSON(t *testing.T) {
	client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{invalid json"))
	})

	_, err := client.SearchArtistByName("Radiohead")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestSearchArtistByName_Cache(t *testing.T) {
	var requestCount atomic.Int32
	client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		json.NewEncoder(w).Encode(artistSearchResponse{
			Artists: []artistJSON{{ID: "test-id", Name: "Test"}},
		})
	})

	_, err := client.SearchArtistByName("Test")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	_, err = client.SearchArtistByName("Test")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if requestCount.Load() != 1 {
		t.Errorf("expected 1 request (cache hit on second), got %d", requestCount.Load())
	}
}

func TestSearchArtistByName_NegativeResultCached(t *testing.T) {
	var requestCount atomic.Int32
	client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		json.NewEncoder(w).Encode(artistSearchResponse{Artists: []artistJSON{}})
	})

	_, err := client.SearchArtistByName("nonexistent")
	if err == nil {
		t.Fatal("expected error on first call for nonexistent artist")
	}

	result, err := client.SearchArtistByName("nonexistent")
	if err != nil {
		t.Fatalf("expected no error on cached negative, got: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result for cached negative")
	}

	if requestCount.Load() != 1 {
		t.Errorf("expected 1 API request (cached negative on second call), got %d", requestCount.Load())
	}
}

func TestSearchAlbumByName_NegativeResultCached(t *testing.T) {
	var requestCount atomic.Int32
	client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		json.NewEncoder(w).Encode(releaseGroupSearchResponse{ReleaseGroups: []releaseGroupJSON{}})
	})

	_, err := client.SearchAlbumByName("nonexistent album", "")
	if err == nil {
		t.Fatal("expected error on first call for nonexistent album")
	}

	result, err := client.SearchAlbumByName("nonexistent album", "")
	if err != nil {
		t.Fatalf("expected no error on cached negative, got: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result for cached negative")
	}

	if requestCount.Load() != 1 {
		t.Errorf("expected 1 API request (cached negative on second call), got %d", requestCount.Load())
	}
}

// --- Primary Artist Extraction ---

func TestPrimaryArtist(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Coldplay & Ayra Starr", "Coldplay"},
		{"Bon Jovi & Jennifer Nettles", "Bon Jovi"},
		{"Coldplay, Little Simz, Burna Boy, Elyanna & TINI", "Coldplay"},
		{"Frank Sinatra & Nancy Sinatra", "Frank Sinatra"},
		{"Ariana Grande feat. John Legend", "Ariana Grande"},
		{"Slash feat Myles Kennedy", "Slash"},
		{"Jack White featuring Alicia Keys", "Jack White"},
		{"Cheap Thrills ft. Sean Paul", "Cheap Thrills"},
		{"Dolly Parton and Kenny Rogers", "Dolly Parton"},
		{"Gente de Zona & Orishas", "Gente de Zona"},
		{"Coldplay", ""},
		{"Radiohead", ""},
		{"", ""},
	}

	for _, tc := range tests {
		got := primaryArtist(tc.input)
		if got != tc.expected {
			t.Errorf("primaryArtist(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestSearchArtistByName_FallbackPrimaryArtist(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		query := r.URL.Query().Get("query")

		if strings.Contains(query, "Ayra Starr") {
			json.NewEncoder(w).Encode(artistSearchResponse{})
			return
		}

		json.NewEncoder(w).Encode(artistSearchResponse{
			Artists: []artistJSON{
				{ID: "coldplay-id", Name: "Coldplay", Type: "Group", Country: "GB"},
			},
		})
	}))
	defer server.Close()

	client := New().(*musicBrainzClient)
	client.baseURL = server.URL
	client.lastRequest = time.Time{}

	result, err := client.SearchArtistByName("Coldplay & Ayra Starr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.MusicBrainzID != "coldplay-id" {
		t.Errorf("expected coldplay-id, got %s", result.MusicBrainzID)
	}

	if result.Name != "Coldplay" {
		t.Errorf("expected Coldplay, got %s", result.Name)
	}

	if atomic.LoadInt32(&requestCount) != 2 {
		t.Errorf("expected 2 requests (original + fallback), got %d", requestCount)
	}
}

// --- Album Search Tests ---

func TestSearchAlbumByName_Success(t *testing.T) {
	rgs := []releaseGroupJSON{{
		ID:               "b1392450-e666-3926-a536-22c65f834433",
		Title:            "OK Computer",
		FirstReleaseDate: "1997-05-21",
		ArtistCredit: []artistCreditJSON{{
			Name:   "Radiohead",
			Artist: artistBriefJSON{ID: "artist-id", Name: "Radiohead"},
		}},
		Releases: []releaseJSON{
			{ID: "release-001", Title: "OK Computer", Status: "Official"},
		},
	}}

	client := setupMockServer(t, releaseGroupHandler(rgs))

	result, err := client.SearchAlbumByName("OK Computer", "Radiohead")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.MusicBrainzID != "b1392450-e666-3926-a536-22c65f834433" {
		t.Errorf("expected MBID 'b1392450-e666-3926-a536-22c65f834433', got '%s'", result.MusicBrainzID)
	}
	if result.ReleaseID != "release-001" {
		t.Errorf("expected ReleaseID 'release-001', got '%s'", result.ReleaseID)
	}
	if result.Title != "OK Computer" {
		t.Errorf("expected Title 'OK Computer', got '%s'", result.Title)
	}
	if result.ReleaseDate != "1997-05-21" {
		t.Errorf("expected ReleaseDate '1997-05-21', got '%s'", result.ReleaseDate)
	}
	if result.Year != 1997 {
		t.Errorf("expected Year 1997, got %d", result.Year)
	}
	if result.ArtistName != "Radiohead" {
		t.Errorf("expected ArtistName 'Radiohead', got '%s'", result.ArtistName)
	}
}

func TestSearchAlbumByName_CoverURL(t *testing.T) {
	rgs := []releaseGroupJSON{{
		ID:    "rg-id",
		Title: "Test Album",
		Releases: []releaseJSON{
			{ID: "rel-abc-123", Title: "Test Album", Status: "Official"},
		},
	}}

	client := setupMockServer(t, releaseGroupHandler(rgs))

	result, err := client.SearchAlbumByName("Test Album", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "https://coverartarchive.org/release-group/rg-id/front-500"
	if result.CoverURL != expected {
		t.Errorf("expected CoverURL '%s', got '%s'", expected, result.CoverURL)
	}
}

func TestSearchAlbumByName_WithArtistHint(t *testing.T) {
	var capturedQuery string
	client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("query")
		json.NewEncoder(w).Encode(releaseGroupSearchResponse{
			ReleaseGroups: []releaseGroupJSON{{
				ID:       "rg-id",
				Title:    "Test",
				Releases: []releaseJSON{{ID: "rel-id", Status: "Official"}},
			}},
		})
	})

	_, err := client.SearchAlbumByName("Test", "Artist Name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(capturedQuery, `artist:"Artist Name"`) {
		t.Errorf("expected query to contain artist hint, got '%s'", capturedQuery)
	}
}

func TestSearchAlbumByName_WithoutArtistHint(t *testing.T) {
	var capturedQuery string
	client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("query")
		json.NewEncoder(w).Encode(releaseGroupSearchResponse{
			ReleaseGroups: []releaseGroupJSON{{
				ID:       "rg-id",
				Title:    "Test",
				Releases: []releaseJSON{{ID: "rel-id", Status: "Official"}},
			}},
		})
	})

	_, err := client.SearchAlbumByName("Test", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(capturedQuery, "artist:") {
		t.Errorf("expected query without artist clause, got '%s'", capturedQuery)
	}
}

func TestSearchAlbumByName_YearParsing(t *testing.T) {
	tests := []struct {
		date     string
		expected int
	}{
		{"1997-05-21", 1997},
		{"2001", 2001},
		{"", 0},
		{"abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.date, func(t *testing.T) {
			result := parseYear(tt.date)
			if result != tt.expected {
				t.Errorf("parseYear(%q) = %d, want %d", tt.date, result, tt.expected)
			}
		})
	}
}

func TestSearchAlbumByName_OfficialRelease(t *testing.T) {
	rgs := []releaseGroupJSON{{
		ID:    "rg-id",
		Title: "Test",
		Releases: []releaseJSON{
			{ID: "promo-id", Title: "Test", Status: "Promotion"},
			{ID: "official-id", Title: "Test", Status: "Official"},
			{ID: "bootleg-id", Title: "Test", Status: "Bootleg"},
		},
	}}

	client := setupMockServer(t, releaseGroupHandler(rgs))

	result, err := client.SearchAlbumByName("Test", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ReleaseID != "official-id" {
		t.Errorf("expected ReleaseID 'official-id', got '%s'", result.ReleaseID)
	}
}

func TestSearchAlbumByName_NoResults(t *testing.T) {
	client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releaseGroupSearchResponse{ReleaseGroups: []releaseGroupJSON{}})
	})

	_, err := client.SearchAlbumByName("nonexistent album 12345", "")
	if err == nil {
		t.Fatal("expected error for no results")
	}
}

func TestSearchAlbumByName_EmptyTitle(t *testing.T) {
	client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {})

	_, err := client.SearchAlbumByName("", "")
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestSearchAlbumByName_Cache(t *testing.T) {
	var requestCount atomic.Int32
	client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		json.NewEncoder(w).Encode(releaseGroupSearchResponse{
			ReleaseGroups: []releaseGroupJSON{{
				ID:       "rg-id",
				Title:    "Cached Album",
				Releases: []releaseJSON{{ID: "rel-id", Status: "Official"}},
			}},
		})
	})

	_, err := client.SearchAlbumByName("Cached Album", "Artist")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	firstCount := requestCount.Load()

	_, err = client.SearchAlbumByName("Cached Album", "Artist")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if requestCount.Load() != firstCount {
		t.Errorf("expected no additional requests on cache hit, got %d total", requestCount.Load())
	}
}

// --- Cache Tests ---

func TestClearAllCaches(t *testing.T) {
	var requestCount atomic.Int32
	client := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		json.NewEncoder(w).Encode(artistSearchResponse{
			Artists: []artistJSON{{ID: "id", Name: "Test"}},
		})
	})

	client.SearchArtistByName("Test")
	client.ClearAllCaches()

	client.SearchArtistByName("Test")
	if requestCount.Load() != 2 {
		t.Errorf("expected 2 requests after cache clear, got %d", requestCount.Load())
	}
}

func TestArtistCacheEviction(t *testing.T) {
	client := New().(*musicBrainzClient)
	max := helpers.MUSICBRAINZ_ARTIST_MAX_CACHE

	for i := 0; i < max+5; i++ {
		client.setArtist(
			string(rune('A'+i%26))+string(rune('0'+i/26)),
			&ArtistResult{MusicBrainzID: "id"},
		)
	}

	if len(client.artistCache) > max {
		t.Errorf("expected cache size <= %d, got %d", max, len(client.artistCache))
	}
}

func TestCleanAlbumTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Speak Now (Deluxe Edition)", "Speak Now"},
		{"1989 (Deluxe)", "1989"},
		{"The Foundation (Deluxe Version)", "The Foundation"},
		{"Bark at the Moon (Bonus Track Version)", "Bark at the Moon"},
		{"Zenyatta Mondatta (Remastered)", "Zenyatta Mondatta"},
		{"(What's The Story) Morning Glory? [Deluxe Remastered Edition]", "(What's The Story) Morning Glory?"},
		{"Stay in the Dark - Single", "Stay in the Dark"},
		{"Cheap Thrills (feat. Sean Paul) - Single", "Cheap Thrills (feat. Sean Paul)"},
		// (from ...) suffix
		{"Mount Pleasant (from The Park)", "Mount Pleasant"},
		// Catch-all version suffix
		{"Up! (Red \"Pop\" Version)", "Up!"},
		// Original with year in middle
		{"The Lion King (Original 1997 Broadway Cast Recording)", "The Lion King"},
		{"Brand New Eyes (Special Edition)", "Brand New Eyes"},
		{"OK Computer", "OK Computer"},
		{"folklore", "folklore"},
		{"", ""},
		// Year-prefixed remaster
		{"Kind of Blue (2015 Remaster)", "Kind of Blue"},
		{"Rumours (2004 Remastered)", "Rumours"},
		// Label-prefixed remaster
		{"Something Cool (OJC Remaster)", "Something Cool"},
		// Anniversary edition with number
		{"Buena Vista Social Club (25th Anniversary Edition)", "Buena Vista Social Club"},
		// Live recordings
		{"MTV Unplugged (Live)", "MTV Unplugged"},
		{"One Night Only (Live at the Royal Albert Hall)", "One Night Only"},
		// Portuguese/Spanish live
		{"Acústico (Ao Vivo)", "Acústico"},
		{"En Concierto (En Directo)", "En Concierto"},
		// Soundtracks
		{"The Lion King (Original Motion Picture Soundtrack)", "The Lion King"},
		{"Hamilton (Original Broadway Cast Recording)", "Hamilton"},
		// Instrumental
		{"Colors (Instrumental)", "Colors"},
		// EP
		{"Dream - EP", "Dream"},
		// Any brackets at end
		{"Red (Taylor's Version) [+ A Message from Taylor]", "Red (Taylor's Version)"},
		{"Abbey Road [2019 Remaster]", "Abbey Road"},
		// Plus suffix
		{"The Life of a Showgirl + Acoustic Collection", "The Life of a Showgirl"},
		// Multi-suffix (loops)
		{"Buena Vista Social Club (25th Anniversary Edition) [2021 Remaster]", "Buena Vista Social Club"},
		// Spanish edition
		{"Más (Edición Especial)", "Más"},
	}

	for _, tc := range tests {
		got := cleanAlbumTitle(tc.input)
		if got != tc.expected {
			t.Errorf("cleanAlbumTitle(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestStripFeaturing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Cheap Thrills (feat. Sean Paul)", "Cheap Thrills"},
		{"Despacito (feat. Daddy Yankee)", "Despacito"},
		{"World On Fire (feat. Myles Kennedy & The Conspirators)", "World On Fire"},
		{"Eachother (feat. Jackson Browne, Marcus King & Lucius)", "Eachother"},
		{"Hell Right (feat. Trace Adkins)", "Hell Right"},
		{"Cheap Thrills (ft. Sean Paul)", "Cheap Thrills"},
		{"Song (featuring Artist)", "Song"},
		{"OK Computer", "OK Computer"},
		{"", ""},
	}

	for _, tc := range tests {
		got := stripFeaturing(tc.input)
		if got != tc.expected {
			t.Errorf("stripFeaturing(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestStripBeforeColon(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"André Rieu: At the Movies", "At the Movies"},
		{"MTV Unplugged: Katy Perry", "Katy Perry"},
		{"Stars: The Best of the Cranberries 1992-2002", "The Best of the Cranberries 1992-2002"},
		{"OK Computer", ""},
		{"No colon here", ""},
		{":", ""},
	}

	for _, tc := range tests {
		got := stripBeforeColon(tc.input)
		if got != tc.expected {
			t.Errorf("stripBeforeColon(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestStripAfterDash(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Daytime Friends - The Very Best of Kenny Rogers", "Daytime Friends"},
		{"Oro - Lo Nuevo y Lo Mejor", "Oro"},
		{"Primera Fila - La Oreja de Van Gogh", "Primera Fila"},
		{"OK Computer", ""},
		{"No dash here", ""},
	}

	for _, tc := range tests {
		got := stripAfterDash(tc.input)
		if got != tc.expected {
			t.Errorf("stripAfterDash(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestAlbumTitleVariants(t *testing.T) {
	variants := albumTitleVariants("Cheap Thrills (feat. Sean Paul) - Single")
	required := []string{
		"Cheap Thrills (feat. Sean Paul)",
		"Cheap Thrills",
	}
	variantSet := make(map[string]bool)
	for _, v := range variants {
		variantSet[v] = true
	}
	for _, r := range required {
		if !variantSet[r] {
			t.Errorf("missing required variant %q, got %v", r, variants)
		}
	}
}

func TestAlbumTitleVariants_ColonTitle(t *testing.T) {
	variants := albumTitleVariants("André Rieu: At the Movies")
	found := false
	for _, v := range variants {
		if v == "At the Movies" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected variant 'At the Movies' from colon stripping, got %v", variants)
	}
}

func TestAlbumTitleVariants_DashSubtitle(t *testing.T) {
	variants := albumTitleVariants("Daytime Friends - The Very Best of Kenny Rogers")
	found := false
	for _, v := range variants {
		if v == "Daytime Friends" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected variant 'Daytime Friends' from dash stripping, got %v", variants)
	}
}

func TestSearchAlbumByName_FallbackFeaturing(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		query := r.URL.Query().Get("query")

		if strings.Contains(query, "feat.") || strings.Contains(query, "Single") {
			json.NewEncoder(w).Encode(releaseGroupSearchResponse{})
			return
		}

		json.NewEncoder(w).Encode(releaseGroupSearchResponse{
			ReleaseGroups: []releaseGroupJSON{{
				ID:       "rg-cheap-thrills",
				Title:    "Cheap Thrills",
				Releases: []releaseJSON{{ID: "rel-1", Status: "Official"}},
			}},
		})
	}))
	defer server.Close()

	client := New().(*musicBrainzClient)
	client.baseURL = server.URL
	client.lastRequest = time.Time{}

	result, err := client.SearchAlbumByName("Cheap Thrills (feat. Sean Paul) - Single", "Sia")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.MusicBrainzID != "rg-cheap-thrills" {
		t.Errorf("expected rg-cheap-thrills, got %s", result.MusicBrainzID)
	}
}

func TestSearchAlbumByName_FallbackNoArtist(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		query := r.URL.Query().Get("query")

		if strings.Contains(query, "artist:") {
			json.NewEncoder(w).Encode(releaseGroupSearchResponse{})
			return
		}

		json.NewEncoder(w).Encode(releaseGroupSearchResponse{
			ReleaseGroups: []releaseGroupJSON{{
				ID:       "rg-found-no-artist",
				Title:    "Own the Night",
				Releases: []releaseJSON{{ID: "rel-1", Status: "Official"}},
			}},
		})
	}))
	defer server.Close()

	client := New().(*musicBrainzClient)
	client.baseURL = server.URL
	client.lastRequest = time.Time{}

	result, err := client.SearchAlbumByName("Own the Night", "Lady A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.MusicBrainzID != "rg-found-no-artist" {
		t.Errorf("expected rg-found-no-artist, got %s", result.MusicBrainzID)
	}
}

func TestSearchAlbumByName_FallbackColonStrip(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		query := r.URL.Query().Get("query")

		if strings.Contains(query, "Andr") {
			json.NewEncoder(w).Encode(releaseGroupSearchResponse{})
			return
		}

		json.NewEncoder(w).Encode(releaseGroupSearchResponse{
			ReleaseGroups: []releaseGroupJSON{{
				ID:       "rg-at-movies",
				Title:    "At the Movies",
				Releases: []releaseJSON{{ID: "rel-1", Status: "Official"}},
			}},
		})
	}))
	defer server.Close()

	client := New().(*musicBrainzClient)
	client.baseURL = server.URL
	client.lastRequest = time.Time{}

	result, err := client.SearchAlbumByName("André Rieu: At the Movies", "André Rieu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.MusicBrainzID != "rg-at-movies" {
		t.Errorf("expected rg-at-movies, got %s", result.MusicBrainzID)
	}
}

func TestSearchAlbumByName_FallbackCleanTitle(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		query := r.URL.Query().Get("query")

		if strings.Contains(query, "Deluxe Edition") {
			json.NewEncoder(w).Encode(releaseGroupSearchResponse{})
			return
		}

		json.NewEncoder(w).Encode(releaseGroupSearchResponse{
			ReleaseGroups: []releaseGroupJSON{{
				ID:    "rg-fallback",
				Title: "Speak Now",
				Releases: []releaseJSON{
					{ID: "rel-1", Status: "Official"},
				},
			}},
		})
	}))
	defer server.Close()

	client := New().(*musicBrainzClient)
	client.baseURL = server.URL
	client.lastRequest = time.Time{}

	result, err := client.SearchAlbumByName("Speak Now (Deluxe Edition)", "Taylor Swift")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.MusicBrainzID != "rg-fallback" {
		t.Errorf("expected rg-fallback, got %s", result.MusicBrainzID)
	}

	if atomic.LoadInt32(&requestCount) != 2 {
		t.Errorf("expected 2 requests (original + fallback), got %d", requestCount)
	}
}

func TestSearchAlbumByName_FallbackPrimaryArtist(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		query := r.URL.Query().Get("query")

		if strings.Contains(query, `artist:"Elton John & Tim Rice"`) ||
			strings.Contains(query, `artist:"Elton John %26 Tim Rice"`) {
			json.NewEncoder(w).Encode(releaseGroupSearchResponse{})
			return
		}

		json.NewEncoder(w).Encode(releaseGroupSearchResponse{
			ReleaseGroups: []releaseGroupJSON{{
				ID:       "rg-lion-king",
				Title:    "The Lion King",
				Releases: []releaseJSON{{ID: "rel-1", Status: "Official"}},
			}},
		})
	}))
	defer server.Close()

	client := New().(*musicBrainzClient)
	client.baseURL = server.URL
	client.lastRequest = time.Time{}

	result, err := client.SearchAlbumByName("The Lion King", "Elton John & Tim Rice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.MusicBrainzID != "rg-lion-king" {
		t.Errorf("expected rg-lion-king, got %s", result.MusicBrainzID)
	}

	count := atomic.LoadInt32(&requestCount)
	if count < 2 {
		t.Errorf("expected at least 2 requests (original + primary artist fallback), got %d", count)
	}
}

func TestAlbumCacheEviction(t *testing.T) {
	client := New().(*musicBrainzClient)

	for i := 0; i < 205; i++ {
		client.setAlbum(
			string(rune('A'+i%26))+string(rune('0'+i/26)),
			&AlbumResult{MusicBrainzID: "id"},
		)
	}

	if len(client.albumCache) > 200 {
		t.Errorf("expected cache size <= 200, got %d", len(client.albumCache))
	}
}

// --- Rate Limiting ---

func TestRateLimiting(t *testing.T) {
	client := setupMockServer(t, artistHandler([]artistJSON{
		{ID: "id-1", Name: "A"},
	}))

	client.SearchArtistByName("A")
	client.ClearAllCaches()

	start := time.Now()
	client.SearchArtistByName("A")
	elapsed := time.Since(start)

	if elapsed < 900*time.Millisecond {
		t.Errorf("expected rate limiting delay of ~1s, got %v", elapsed)
	}
}
