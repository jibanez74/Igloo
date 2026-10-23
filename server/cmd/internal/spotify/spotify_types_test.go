package spotify

import (
	"encoding/json"
	"testing"
	"time"
)

func TestReleaseDateTime(t *testing.T) {
	tests := []struct {
		date      string
		precision string
		want      string
	}{
		{"1981-12-15", "day", "1981-12-15"},
		{"1981-12", "month", "1981-12-01"},
		{"1981", "year", "1981-01-01"},
		{"0000", "year", ""},
		{"", "", ""},
		{"1981", "month", ""},
		{"1981-12-15", "", ""},
		{"not-a-date", "day", ""},
	}

	for _, tc := range tests {
		album := SimpleAlbum{ReleaseDate: tc.date, ReleaseDatePrecision: tc.precision}
		got := album.ReleaseDateTime()
		if tc.want == "" {
			if !got.IsZero() {
				t.Errorf("ReleaseDateTime(%q, %q) = %s, want the zero time", tc.date, tc.precision, got)
			}
			continue
		}
		if got.Format(time.DateOnly) != tc.want || got.Location() != time.UTC {
			t.Errorf("ReleaseDateTime(%q, %q) = %s, want %s UTC", tc.date, tc.precision, got, tc.want)
		}
	}
}

func TestNumericDecodesIntegersFloatsAndNull(t *testing.T) {
	var got struct {
		Integer Numeric `json:"integer"`
		Float   Numeric `json:"float"`
		Null    Numeric `json:"null"`
	}
	err := json.Unmarshal([]byte(`{"integer":42,"float":1.5e6,"null":null}`), &got)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Integer != 42 || got.Float != 1_500_000 || got.Null != 0 {
		t.Fatalf("decoded = %+v, want 42, 1500000 and 0", got)
	}

	err = json.Unmarshal([]byte(`{"integer":"42"}`), &got)
	if err == nil {
		t.Fatal("expected a string to fail Numeric decoding")
	}
}

// These payloads follow the Spotify Web API reference shapes, so a wrong JSON
// tag on any field Igloo reads fails here rather than as a silently empty field.
func TestCatalogTypesDecodeSpotifyPayloads(t *testing.T) {
	t.Run("search result pages", func(t *testing.T) {
		var result searchResult
		err := json.Unmarshal([]byte(`{
			"artists": {"href": "", "limit": 1, "offset": 0, "total": 1, "next": null, "previous": null, "items": [{
				"id": "artist1", "name": "Radiohead", "type": "artist", "uri": "spotify:artist:artist1",
				"popularity": 79, "genres": ["alternative rock", "art rock"],
				"followers": {"href": null, "total": 9876543},
				"images": [{"url": "https://i.scdn.co/artist-640.jpg", "height": 640, "width": 640}],
				"external_urls": {"spotify": "https://open.spotify.com/artist/artist1"}
			}]},
			"albums": {"items": [{
				"id": "album1", "name": "OK Computer", "album_type": "album", "total_tracks": 12,
				"release_date": "1997-05-21", "release_date_precision": "day",
				"artists": [{"id": "artist1", "name": "Radiohead"}],
				"images": [{"url": "https://i.scdn.co/album-640.jpg", "height": null, "width": null}],
				"external_urls": {"spotify": "https://open.spotify.com/album/album1"}
			}]},
			"tracks": {"items": [{
				"id": "track1", "name": "Paranoid Android", "duration_ms": 383066, "explicit": false,
				"artists": [{"id": "artist1", "name": "Radiohead"}],
				"album": {"id": "album1", "name": "OK Computer", "release_date": "1997-05-21", "release_date_precision": "day",
					"images": [{"url": "https://i.scdn.co/album-640.jpg"}]},
				"external_urls": {"spotify": "https://open.spotify.com/track/track1"}
			}]}
		}`), &result)
		if err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}

		artist := result.Artists.Items[0]
		if artist.ID.String() != "artist1" || artist.Name != "Radiohead" || artist.Images[0].URL != "https://i.scdn.co/artist-640.jpg" {
			t.Fatalf("artist = %+v", artist)
		}

		album := result.Albums.Items[0]
		if album.ID.String() != "album1" || album.Name != "OK Computer" || album.AlbumType != "album" || album.TotalTracks != 12 ||
			album.ReleaseDate != "1997-05-21" || album.ReleaseDatePrecision != "day" || album.Artists[0].Name != "Radiohead" ||
			album.Images[0].URL != "https://i.scdn.co/album-640.jpg" || album.ExternalURLs["spotify"] != "https://open.spotify.com/album/album1" {
			t.Fatalf("album = %+v", album)
		}

		track := result.Tracks.Items[0]
		if track.ID.String() != "track1" || track.Name != "Paranoid Android" || track.Duration != 383066 ||
			track.Artists[0].Name != "Radiohead" || track.Album.Name != "OK Computer" || track.Album.ReleaseDate != "1997-05-21" ||
			track.Album.Images[0].URL != "https://i.scdn.co/album-640.jpg" || track.ExternalURLs["spotify"] != "https://open.spotify.com/track/track1" {
			t.Fatalf("track = %+v", track)
		}
	})

	t.Run("album details", func(t *testing.T) {
		var album albumDetails
		err := json.Unmarshal([]byte(`{
			"id": "album1", "name": "OK Computer", "album_type": "album", "total_tracks": 12,
			"release_date": "1997", "release_date_precision": "year",
			"popularity": 81, "genres": [], "label": "XL",
			"tracks": {"href": "", "limit": 50, "offset": 0, "next": null, "previous": null, "total": 12, "items": [
				{"id": "track1", "name": "Airbag", "disc_number": 1, "track_number": 1, "duration_ms": 287880, "explicit": false, "type": "track"},
				{"id": "track2", "name": "Paranoid Android", "disc_number": 1, "track_number": 2, "duration_ms": 387213, "explicit": true, "type": "track"}
			]},
			"copyrights": [{"text": "1997 XL", "type": "C"}],
			"external_ids": {"upc": "634904078164"},
			"images": [{"url": "https://i.scdn.co/album-640.jpg"}]
		}`), &album)
		if err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}

		if album.ID.String() != "album1" || album.AlbumType != "album" || album.TotalTracks != 12 ||
			album.Images[0].URL != "https://i.scdn.co/album-640.jpg" ||
			album.ReleaseDateTime().Format(time.DateOnly) != "1997-01-01" {
			t.Fatalf("album = %+v", album)
		}
		if album.Tracks.Total != 12 || len(album.Tracks.Items) != 2 {
			t.Fatalf("tracks = %+v, want a page of 2 out of 12", album.Tracks)
		}
		track := album.Tracks.Items[1]
		if track.ID != "track2" || track.Name != "Paranoid Android" || track.DiscNumber != 1 || track.TrackNumber != 2 ||
			track.Duration != 387213 || !track.Explicit {
			t.Fatalf("track = %+v", track)
		}
	})

	t.Run("catalog urls", func(t *testing.T) {
		if AlbumURL("album1") != "https://open.spotify.com/album/album1" || ArtistURL("artist1") != "https://open.spotify.com/artist/artist1" {
			t.Fatalf("urls = %q, %q", AlbumURL("album1"), ArtistURL("artist1"))
		}
	})
}
