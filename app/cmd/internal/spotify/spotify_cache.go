package spotify

import (
	"igloo/cmd/internal/helpers"

	"github.com/zmb3/spotify/v2"
)

func (c *spotifyClient) getArtist(key string) (*spotify.FullArtist, bool) {
	c.artistMu.RLock()
	defer c.artistMu.RUnlock()

	artist, exists := c.artistCache[key]
	return artist, exists
}

func (c *spotifyClient) setArtist(key string, artist *spotify.FullArtist) {
	c.artistMu.Lock()
	defer c.artistMu.Unlock()

	if _, exists := c.artistCache[key]; exists {
		c.artistCache[key] = artist
		return
	}

	if len(c.artistCache) >= helpers.SPOTIFY_ARTIST_MAX_CACHE {
		oldestKey := c.artistKeys[0]
		c.artistKeys = c.artistKeys[1:]
		delete(c.artistCache, oldestKey)
	}

	c.artistCache[key] = artist
	c.artistKeys = append(c.artistKeys, key)
}

func (c *spotifyClient) getAlbum(key string) (*spotify.FullAlbum, bool) {
	c.albumMu.RLock()
	defer c.albumMu.RUnlock()

	album, exists := c.albumCache[key]
	return album, exists
}

func (c *spotifyClient) setAlbum(key string, album *spotify.FullAlbum) {
	c.albumMu.Lock()
	defer c.albumMu.Unlock()

	if _, exists := c.albumCache[key]; exists {
		c.albumCache[key] = album
		return
	}

	if len(c.albumCache) >= helpers.SPOTIFY_ALBUM_MAX_CACHE {
		oldestKey := c.albumKeys[0]
		c.albumKeys = c.albumKeys[1:]
		delete(c.albumCache, oldestKey)
	}

	c.albumCache[key] = album
	c.albumKeys = append(c.albumKeys, key)
}

func (c *spotifyClient) clearArtistCache() {
	c.artistMu.Lock()
	defer c.artistMu.Unlock()

	c.artistCache = make(map[string]*spotify.FullArtist)
	c.artistKeys = make([]string, 0, helpers.SPOTIFY_ARTIST_MAX_CACHE)
}

func (c *spotifyClient) clearAlbumCache() {
	c.albumMu.Lock()
	defer c.albumMu.Unlock()

	c.albumCache = make(map[string]*spotify.FullAlbum)
	c.albumKeys = make([]string, 0, helpers.SPOTIFY_ALBUM_MAX_CACHE)
}

func (c *spotifyClient) ClearAllCaches() {
	c.clearArtistCache()
	c.clearAlbumCache()
}
