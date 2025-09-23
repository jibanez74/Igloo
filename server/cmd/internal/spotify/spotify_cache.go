package spotify

import "github.com/zmb3/spotify/v2"

func (s *SpotifyClient) cleanArtistCache() {
	if len(s.artistCache) >= maxCacheSize {
		s.artistCache = make(map[string]*spotify.FullArtist)
	}
}

func (s *SpotifyClient) cleanAlbumCache() {
	if len(s.albumCache) >= maxCacheSize {
		s.albumCache = make(map[string]*spotify.FullAlbum)
	}
}

func (s *SpotifyClient) ClearCaches() {
	s.artistCache = make(map[string]*spotify.FullArtist)
	s.albumCache = make(map[string]*spotify.FullAlbum)
}
