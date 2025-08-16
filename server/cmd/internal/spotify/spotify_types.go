package spotify

import "github.com/zmb3/spotify/v2"

type SpotifyInterface interface {
	GetAlbumBySpotifyID(albumID string) (*spotify.FullAlbum, error)
	SearchAlbums(query string, limit int) ([]spotify.SimpleAlbum, error)
	GetArtistBySpotifyID(artistID string) (*spotify.FullArtist, error)
	GetArtistAlbums(artistID string, limit int) ([]spotify.SimpleAlbum, error)
	SearchArtists(query string, limit int) ([]spotify.FullArtist, error)
	SearchArtistByName(artistName string) (*spotify.FullArtist, error)
	GetTrackBySpotifyID(trackID string) (*spotify.FullTrack, error)
	SearchTracks(query string, limit int) ([]spotify.FullTrack, error)
	SearchTracksByArtist(artistName string, limit int) ([]spotify.FullTrack, error)
}

type SpotifyClient struct {
	client *spotify.Client
}
