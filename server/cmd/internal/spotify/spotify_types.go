package spotify

import (
	"github.com/zmb3/spotify/v2"
)

type SpotifyInterface interface {
	SearchAndGetAlbumDetails(query string) (*spotify.FullAlbum, error)
	SearchArtistByName(artistName string) (*spotify.FullArtist, error)
	ClearCaches()
}

type SpotifyClient struct {
	client      *spotify.Client
	artistCache map[string]*spotify.FullArtist
	albumCache  map[string]*spotify.FullAlbum
}
