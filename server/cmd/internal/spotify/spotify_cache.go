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
	return v.(*spotify.FullArtist), true
}

func (c *spotifyClient) setArtist(key string, artist *spotify.FullArtist) {
	c.artistCache.Set(key, artist, cache.DefaultExpiration)
}

func (c *spotifyClient) getAlbum(key string) (*spotify.FullAlbum, bool) {
	v, found := c.albumCache.Get(key)
	if !found {
		return nil, false
	}
	return v.(*spotify.FullAlbum), true
}

func (c *spotifyClient) setAlbum(key string, album *spotify.FullAlbum) {
	c.albumCache.Set(key, album, cache.DefaultExpiration)
}

func (c *spotifyClient) clearArtistCache() {
	c.artistCache.Flush()
}

func (c *spotifyClient) clearAlbumCache() {
	c.albumCache.Flush()
}

func (c *spotifyClient) ClearAllCaches() {
	c.clearArtistCache()
	c.clearAlbumCache()
}
