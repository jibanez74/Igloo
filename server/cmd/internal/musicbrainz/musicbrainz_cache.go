package musicbrainz

import (
	"time"

	"igloo/cmd/internal/helpers"
)

func (c *musicBrainzClient) getArtist(key string) (*ArtistResult, bool) {
	c.artistMu.Lock()
	defer c.artistMu.Unlock()
	ent, exists := c.artistCache[key]
	if exists && time.Now().Before(ent.expiresAt) {
		return ent.result, true
	}
	if exists {
		delete(c.artistCache, key)
		c.artistKeys = removeKey(c.artistKeys, key)
	}
	return nil, false
}

func (c *musicBrainzClient) setArtistMemory(key string, artist *ArtistResult) {
	expiresAt := time.Now().Add(helpers.MUSICBRAINZ_CACHE_TTL)
	if _, exists := c.artistCache[key]; exists {
		c.artistCache[key] = &artistCacheEntry{result: artist, expiresAt: expiresAt}
		return
	}
	if len(c.artistCache) >= helpers.MUSICBRAINZ_ARTIST_MAX_CACHE {
		oldestKey := c.artistKeys[0]
		c.artistKeys = c.artistKeys[1:]
		delete(c.artistCache, oldestKey)
	}
	c.artistCache[key] = &artistCacheEntry{result: artist, expiresAt: expiresAt}
	c.artistKeys = append(c.artistKeys, key)
}

func (c *musicBrainzClient) setArtist(key string, artist *ArtistResult) {
	c.artistMu.Lock()
	defer c.artistMu.Unlock()
	c.setArtistMemory(key, artist)
}

func (c *musicBrainzClient) getAlbum(key string) (*AlbumResult, bool) {
	c.albumMu.Lock()
	defer c.albumMu.Unlock()
	ent, exists := c.albumCache[key]
	if exists && time.Now().Before(ent.expiresAt) {
		return ent.result, true
	}
	if exists {
		delete(c.albumCache, key)
		c.albumKeys = removeKey(c.albumKeys, key)
	}
	return nil, false
}

func (c *musicBrainzClient) setAlbumMemory(key string, album *AlbumResult) {
	expiresAt := time.Now().Add(helpers.MUSICBRAINZ_CACHE_TTL)
	if _, exists := c.albumCache[key]; exists {
		c.albumCache[key] = &albumCacheEntry{result: album, expiresAt: expiresAt}
		return
	}
	if len(c.albumCache) >= helpers.MUSICBRAINZ_ALBUM_MAX_CACHE {
		oldestKey := c.albumKeys[0]
		c.albumKeys = c.albumKeys[1:]
		delete(c.albumCache, oldestKey)
	}
	c.albumCache[key] = &albumCacheEntry{result: album, expiresAt: expiresAt}
	c.albumKeys = append(c.albumKeys, key)
}

func (c *musicBrainzClient) setAlbum(key string, album *AlbumResult) {
	c.albumMu.Lock()
	defer c.albumMu.Unlock()
	c.setAlbumMemory(key, album)
}

func (c *musicBrainzClient) getImage(key string) (string, bool) {
	c.imageMu.Lock()
	defer c.imageMu.Unlock()
	ent, exists := c.imageCache[key]
	if exists && time.Now().Before(ent.expiresAt) {
		return ent.url, true
	}
	if exists {
		delete(c.imageCache, key)
		c.imageKeys = removeKey(c.imageKeys, key)
	}
	return "", false
}

func (c *musicBrainzClient) setImageMemory(key, url string) {
	expiresAt := time.Now().Add(helpers.MUSICBRAINZ_CACHE_TTL)
	if _, exists := c.imageCache[key]; exists {
		c.imageCache[key] = &imageCacheEntry{url: url, expiresAt: expiresAt}
		return
	}
	if len(c.imageCache) >= helpers.MUSICBRAINZ_ARTIST_MAX_CACHE {
		oldestKey := c.imageKeys[0]
		c.imageKeys = c.imageKeys[1:]
		delete(c.imageCache, oldestKey)
	}
	c.imageCache[key] = &imageCacheEntry{url: url, expiresAt: expiresAt}
	c.imageKeys = append(c.imageKeys, key)
}

func (c *musicBrainzClient) setImage(key, url string) {
	c.imageMu.Lock()
	defer c.imageMu.Unlock()
	c.setImageMemory(key, url)
}

func removeKey(keys []string, k string) []string {
	for i, key := range keys {
		if key == k {
			return append(keys[:i], keys[i+1:]...)
		}
	}
	return keys
}

func (c *musicBrainzClient) Close() error {
	return nil
}

func (c *musicBrainzClient) ClearAllCaches() {
	c.artistMu.Lock()
	c.artistCache = make(map[string]*artistCacheEntry)
	c.artistKeys = make([]string, 0, helpers.MUSICBRAINZ_ARTIST_MAX_CACHE)
	c.artistMu.Unlock()

	c.albumMu.Lock()
	c.albumCache = make(map[string]*albumCacheEntry)
	c.albumKeys = make([]string, 0, helpers.MUSICBRAINZ_ALBUM_MAX_CACHE)
	c.albumMu.Unlock()

	c.imageMu.Lock()
	c.imageCache = make(map[string]*imageCacheEntry)
	c.imageKeys = make([]string, 0, helpers.MUSICBRAINZ_ARTIST_MAX_CACHE)
	c.imageMu.Unlock()
}
