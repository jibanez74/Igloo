package spotify

import (
	"time"

	cache "github.com/patrickmn/go-cache"
	"github.com/zmb3/spotify/v2"
)

const (
	spotifyArtistCacheTTL = 15 * time.Minute
	spotifyAlbumCacheTTL  = 15 * time.Minute
	spotifyCacheCleanup   = 5 * time.Minute
)

func (c *spotifyClient) getArtist(key string) (*spotify.FullArtist, bool) {
	v, found := c.artistCache.Get(key)
	if !found {
		return nil, false
	}
	artist, ok := v.(*spotify.FullArtist)
	if !ok {
		return nil, false
	}
	return artist, true
}

func (c *spotifyClient) setArtist(key string, artist *spotify.FullArtist) {
	c.artistCache.Set(key, artist, cache.DefaultExpiration)
}

func (c *spotifyClient) getAlbum(key string) (*spotify.FullAlbum, bool) {
	v, found := c.albumCache.Get(key)
	if !found {
		return nil, false
	}
	album, ok := v.(*spotify.FullAlbum)
	if !ok {
		return nil, false
	}
	return album, true
}

func (c *spotifyClient) setAlbum(key string, album *spotify.FullAlbum) {
	c.albumCache.Set(key, album, cache.DefaultExpiration)
}

func (c *spotifyClient) ClearAllCaches() {
	c.artistCache.Flush()
	c.albumCache.Flush()
}
