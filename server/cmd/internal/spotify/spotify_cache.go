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
	v, _ := c.artistCache.Get(key)
	artist, ok := v.(*spotify.FullArtist)
	return artist, ok
}

func (c *spotifyClient) setArtist(key string, artist *spotify.FullArtist) {
	c.artistCache.Set(key, artist, cache.DefaultExpiration)
}

func (c *spotifyClient) getAlbum(key string) (*spotify.FullAlbum, bool) {
	v, _ := c.albumCache.Get(key)
	album, ok := v.(*spotify.FullAlbum)
	return album, ok
}

func (c *spotifyClient) setAlbum(key string, album *spotify.FullAlbum) {
	c.albumCache.Set(key, album, cache.DefaultExpiration)
}

func (c *spotifyClient) ClearAllCaches() {
	c.artistCache.Flush()
	c.albumCache.Flush()
}
