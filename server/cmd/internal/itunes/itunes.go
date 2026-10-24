// Package itunes resolves Apple catalog ids through the keyless iTunes Lookup
// API, and reads those ids from the iTunes metadata of purchased MP4 files.
package itunes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"igloo/cmd/internal/helpers"
)

const (
	lookupURL = "https://itunes.apple.com/lookup"
	// storefront is the store whose catalog lookups and album links use.
	storefront = "us"
	// MaxLookupIDs is the most collection ids one lookup request carries.
	MaxLookupIDs = 50
	// albumTrackLimit is the most songs a lookup returns per collection; the
	// API's default is 50 and 200 is its cap.
	albumTrackLimit = 200
	// requestSpacing keeps lookups under the API's documented allowance of
	// roughly 20 calls a minute.
	requestSpacing = 3 * time.Second
	artworkSize    = "640x640bb"
)

// Client looks albums up by collection id. It makes one attempt per request
// and spaces requests out; it is safe for concurrent use.
type Client struct {
	baseURL    string
	httpClient *http.Client
	spacing    time.Duration
	mu         sync.Mutex
	lastSent   time.Time
}

// New returns a client for the US storefront.
func New() *Client {
	return &Client{
		baseURL:    lookupURL,
		httpClient: &http.Client{Timeout: helpers.PROVIDER_HTTP_TIMEOUT},
		spacing:    requestSpacing,
	}
}

// Album is a collection the catalog lists, with the songs it returned.
type Album struct {
	ID            int64
	Name          string
	ArtistName    string
	TrackCount    int
	ArtworkURL100 string
	Tracks        map[int64]Track
}

// Track is one song of a listed collection.
type Track struct {
	Explicit bool
}

// Type maps the collection onto the album types the library stores. The
// catalog reports every collection as an album, so singles and EPs are told
// apart by the title suffix the store gives them.
func (a Album) Type() string {
	single := strings.HasSuffix(a.Name, " - Single") || strings.HasSuffix(a.Name, " - EP")
	switch {
	case single:
		return "single"
	case a.ArtistName == "Various Artists":
		return "compilation"
	}
	return "album"
}

// CoverURL is the collection's artwork at 640x640, the size of the covers the
// library takes from Spotify. Artwork URLs encode their size, so the 100px
// URL the lookup returns is resized in place.
func (a Album) CoverURL() string {
	return strings.Replace(a.ArtworkURL100, "100x100bb", artworkSize, 1)
}

// AlbumURL is the collection's page on Apple Music.
func AlbumURL(id int64) string {
	return "https://music.apple.com/" + storefront + "/album/" + strconv.FormatInt(id, 10)
}

// ProviderFailure classifies a lookup error for scanner.ProviderBreaker. The
// API takes no credentials, so no answer rejects them for good; throttling and
// outages are transient alike.
func ProviderFailure(err error) (fatal, transient bool) {
	return false, err != nil
}

type lookupResponse struct {
	Results []lookupResult `json:"results"`
}

type lookupResult struct {
	WrapperType       string `json:"wrapperType"`
	CollectionID      int64  `json:"collectionId"`
	CollectionName    string `json:"collectionName"`
	ArtistName        string `json:"artistName"`
	TrackCount        int    `json:"trackCount"`
	ArtworkURL100     string `json:"artworkUrl100"`
	TrackID           int64  `json:"trackId"`
	TrackExplicitness string `json:"trackExplicitness"`
}

// LookupAlbums looks up to MaxLookupIDs collection ids up in one request and
// returns the listed ones by id. An id the catalog no longer lists is simply
// absent: the API answers it with no result rather than an error.
func (c *Client) LookupAlbums(ctx context.Context, ids []int64) (map[int64]Album, error) {
	if len(ids) == 0 {
		return map[int64]Album{}, nil
	}
	if len(ids) > MaxLookupIDs {
		return nil, fmt.Errorf("itunes lookup: %d ids exceed the limit of %d", len(ids), MaxLookupIDs)
	}
	idList := make([]string, len(ids))
	for i, id := range ids {
		idList[i] = strconv.FormatInt(id, 10)
	}
	query := url.Values{
		"id":      {strings.Join(idList, ",")},
		"entity":  {"song"},
		"limit":   {strconv.Itoa(albumTrackLimit)},
		"country": {storefront},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	err = c.wait(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("itunes lookup: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("itunes lookup: status %d", resp.StatusCode)
	}
	var body lookupResponse
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		return nil, fmt.Errorf("itunes lookup: decode response: %w", err)
	}

	albums := make(map[int64]Album, len(ids))
	for _, result := range body.Results {
		if result.WrapperType != "collection" {
			continue
		}
		albums[result.CollectionID] = Album{
			ID:            result.CollectionID,
			Name:          result.CollectionName,
			ArtistName:    result.ArtistName,
			TrackCount:    result.TrackCount,
			ArtworkURL100: result.ArtworkURL100,
			Tracks:        make(map[int64]Track),
		}
	}
	for _, result := range body.Results {
		album, listed := albums[result.CollectionID]
		if result.WrapperType != "track" || !listed {
			continue
		}
		album.Tracks[result.TrackID] = Track{Explicit: result.TrackExplicitness == "explicit"}
	}
	return albums, nil
}

// wait holds a request until the spacing since the previous one has passed.
func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delay := time.Until(c.lastSent.Add(c.spacing))
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	c.lastSent = time.Now()
	return nil
}
