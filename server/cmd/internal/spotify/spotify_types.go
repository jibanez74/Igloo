package spotify

import (
	"github.com/zmb3/spotify/v2"
)

type SpotifyInterface interface {
	GetAlbumBySpotifyID(albumID string) (*spotify.FullAlbum, error)
	SearchAlbums(query string, limit int) ([]spotify.SimpleAlbum, error)
	SearchAndGetAlbumDetails(query string) (*spotify.FullAlbum, error)
	GetArtistBySpotifyID(artistID string) (*spotify.FullArtist, error)
	SearchArtistByName(artistName string) (*spotify.FullArtist, error)
}

type SpotifyClient struct {
	client      *spotify.Client
	artistCache map[string]*spotify.FullArtist
	albumCache  map[string]*spotify.FullAlbum
}
