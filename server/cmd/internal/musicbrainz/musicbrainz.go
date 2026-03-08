package musicbrainz

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"igloo/cmd/internal/helpers"
	"golang.org/x/sync/singleflight"
)

type MusicBrainzInterface interface {
	SearchArtistByName(name string) (*ArtistResult, error)
	SearchAlbumByName(title string, artistHint string) (*AlbumResult, error)
	GetArtistImageURL(musicBrainzID string) (string, error)
	GetAlbumImageURL(releaseGroupID string) (string, error)
	ClearAllCaches()
	Close() error
}

var _ MusicBrainzInterface = (*musicBrainzClient)(nil)

type musicBrainzClient struct {
	httpClient       *http.Client
	userAgent        string
	baseURL          string
	audioDBBaseURL   string
	audioDBAPIKey    string
	lastRequest      time.Time
	rateMu           sync.Mutex
	audioDBLastReq   time.Time
	audioDBRateMu    sync.Mutex
	artistCache      map[string]*artistCacheEntry
	artistKeys       []string
	artistMu         sync.RWMutex
	albumCache       map[string]*albumCacheEntry
	albumKeys        []string
	albumMu          sync.RWMutex
	imageCache       map[string]*imageCacheEntry
	imageKeys        []string
	imageMu          sync.RWMutex
	sfGroup          singleflight.Group
	disk             *diskCache
}

const musicBrainzCacheTTL = 2 * time.Hour

type artistCacheEntry struct {
	result    *ArtistResult
	expiresAt time.Time
}

type albumCacheEntry struct {
	result    *AlbumResult
	expiresAt time.Time
}

type imageCacheEntry struct {
	url       string
	expiresAt time.Time
}

func New(cacheDir string) MusicBrainzInterface {
	c := &musicBrainzClient{
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		userAgent:      "Igloo/1.0 (music media server)",
		baseURL:        "https://musicbrainz.org/ws/2",
		audioDBBaseURL: audioDBBaseURL,
		audioDBAPIKey:  "2",
		artistCache:    make(map[string]*artistCacheEntry),
		artistKeys:     make([]string, 0, helpers.MUSICBRAINZ_ARTIST_MAX_CACHE),
		albumCache:     make(map[string]*albumCacheEntry),
		albumKeys:      make([]string, 0, helpers.MUSICBRAINZ_ALBUM_MAX_CACHE),
		imageCache:     make(map[string]*imageCacheEntry),
		imageKeys:      make([]string, 0, helpers.MUSICBRAINZ_ARTIST_MAX_CACHE),
	}
	if cacheDir != "" {
		if disk, err := openDiskCache(cacheDir); err == nil {
			c.disk = disk
		}
	}
	return c
}

func (c *musicBrainzClient) rateLimitedGet(lastReq *time.Time, mu *sync.Mutex, endpoint string, v any) error {
	mu.Lock()
	if elapsed := time.Since(*lastReq); elapsed < time.Second {
		time.Sleep(time.Second - elapsed)
	}
	*lastReq = time.Now()
	mu.Unlock()

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	return json.Unmarshal(body, v)
}

func (c *musicBrainzClient) doRequest(endpoint string, v any) error {
	return c.rateLimitedGet(&c.lastRequest, &c.rateMu, endpoint, v)
}

func (c *musicBrainzClient) doAudioDBRequest(endpoint string, v any) error {
	return c.rateLimitedGet(&c.audioDBLastReq, &c.audioDBRateMu, endpoint, v)
}
