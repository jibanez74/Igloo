package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/spotify"
	"net/http"
	"strings"
)

// spotifySearcher is the part of the Spotify client the request pickers use.
type spotifySearcher interface {
	SearchAlbums(ctx context.Context, title string) ([]spotify.SimpleAlbum, error)
	SearchTracks(ctx context.Context, title string) ([]spotify.FullTrack, error)
}

type spotifySearchRequest struct {
	Title string `json:"title"`
}

type spotifyAlbumSearchResult struct {
	SpotifyID        string   `json:"spotify_id"`
	Title            string   `json:"title"`
	ArtistNames      []string `json:"artist_names"`
	ReleaseDate      string   `json:"release_date"`
	AlbumType        string   `json:"album_type"`
	TotalTracks      int      `json:"total_tracks"`
	CoverURL         string   `json:"cover_url"`
	SpotifyURL       string   `json:"spotify_url"`
	AlreadyInLibrary bool     `json:"already_in_library"`
	LibraryAlbumID   *int64   `json:"library_album_id,omitempty"`
}

func (app *Application) GetSpotifyStatus(w http.ResponseWriter, r *http.Request) {
	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: map[string]any{"available": app.Spotify != nil}})
}

// readSpotifySearchTitle reads the title a Spotify search names, answering 503
// when Spotify is not configured and 400 for a bad body or a blank title.
func (app *Application) readSpotifySearchTitle(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !app.ensureSpotifyAvailable(w) {
		return "", false
	}

	var payload spotifySearchRequest
	err := helpers.ReadJSON(w, r, &payload)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return "", false
	}

	title := strings.TrimSpace(payload.Title)
	if title == "" {
		helpers.ErrorJSON(w, errors.New("title is required"), http.StatusBadRequest)
		return "", false
	}
	return title, true
}

func (app *Application) SearchSpotifyAlbums(w http.ResponseWriter, r *http.Request) {
	title, ok := app.readSpotifySearchTitle(w, r)
	if !ok {
		return
	}

	albums, err := app.Spotify.SearchAlbums(r.Context(), title)
	if err != nil {
		app.Logger.Error("spotify album search failed", "error", err, "title", title)
		helpers.ErrorJSON(w, errors.New("Spotify album search failed"), http.StatusBadGateway)
		return
	}

	results, err := app.mapSpotifyAlbumSearchResults(r, albums)
	if err != nil {
		app.serverError(w, err, "failed to map spotify album search results", "failed to map Spotify album search results")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: map[string]any{"results": results}})
}

type spotifyTrackSearchResult struct {
	SpotifyID   string   `json:"spotify_id"`
	Title       string   `json:"title"`
	ArtistNames []string `json:"artist_names"`
	AlbumName   string   `json:"album_name"`
	ReleaseDate string   `json:"release_date"`
	DurationMs  int      `json:"duration_ms"`
	CoverURL    string   `json:"cover_url"`
	SpotifyURL  string   `json:"spotify_url"`
}

// SearchSpotifyTracks backs the "Request Track" picker. Unlike albums, tracks
// have no spotify_id in the library, so results omit an "already in library"
// flag.
func (app *Application) SearchSpotifyTracks(w http.ResponseWriter, r *http.Request) {
	title, ok := app.readSpotifySearchTitle(w, r)
	if !ok {
		return
	}

	tracks, err := app.Spotify.SearchTracks(r.Context(), title)
	if err != nil {
		app.Logger.Error("spotify track search failed", "error", err, "title", title)
		helpers.ErrorJSON(w, errors.New("Spotify track search failed"), http.StatusBadGateway)
		return
	}

	results := mapSpotifyTrackSearchResults(tracks)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: map[string]any{"results": results}})
}

func mapSpotifyTrackSearchResults(tracks []spotify.FullTrack) []spotifyTrackSearchResult {
	mapped := make([]spotifyTrackSearchResult, 0, len(tracks))

	for _, track := range tracks {
		spotifyID := track.ID.String()
		if spotifyID == "" {
			continue
		}

		mapped = append(mapped, spotifyTrackSearchResult{
			SpotifyID:   spotifyID,
			Title:       track.Name,
			ArtistNames: spotifyAlbumArtistNames(track.Artists),
			AlbumName:   track.Album.Name,
			ReleaseDate: track.Album.ReleaseDate,
			DurationMs:  int(track.Duration),
			CoverURL:    firstSpotifyAlbumImageURL(track.Album.Images),
			SpotifyURL:  track.ExternalURLs["spotify"],
		})
	}

	return mapped
}

func (app *Application) ensureSpotifyAvailable(w http.ResponseWriter) bool {
	if app.Spotify != nil {
		return true
	}

	helpers.ErrorJSON(w, errors.New("Spotify search is unavailable"), http.StatusServiceUnavailable)
	return false
}

// libraryAlbumIDsBySpotifyID resolves a whole page of Spotify ids in one query,
// for the same reason as libraryMovieIDsByTmdbID: a lookup per result row is
// ~20 serialized point queries on a single-connection pool.
func (app *Application) libraryAlbumIDsBySpotifyID(ctx context.Context, albums []spotify.SimpleAlbum) (map[string]int64, error) {
	spotifyIDs := make([]sql.NullString, 0, len(albums))
	seen := make(map[string]struct{}, len(albums))

	for _, album := range albums {
		id := album.ID.String()
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		spotifyIDs = append(spotifyIDs, helpers.NullString(id))
	}

	if len(spotifyIDs) == 0 {
		return nil, nil
	}

	rows, err := app.Queries.GetAlbumsBySpotifyIDs(ctx, spotifyIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to look up existing albums by spotify id: %w", err)
	}

	existing := make(map[string]int64, len(rows))
	for _, row := range rows {
		if row.SpotifyID.Valid {
			existing[row.SpotifyID.String] = row.ID
		}
	}

	return existing, nil
}

// libraryAlbumIDsByName resolves the albums that have no Spotify id match, in
// one query: a library album matched on Apple Music, or not at all, has no
// Spotify id to find it by, so it is found by the alias keys its files' tags
// saved it under. A Spotify-matched library album is only ever found by id.
func (app *Application) libraryAlbumIDsByName(ctx context.Context, albums []spotify.SimpleAlbum, byID map[string]int64) (map[string]int64, error) {
	candidates := make(map[string][][2]string, len(albums))
	var keys [][2]string
	seen := make(map[[2]string]bool)
	for _, album := range albums {
		id := album.ID.String()
		_, found := byID[id]
		if id == "" || found {
			continue
		}
		candidates[id] = albumIdentityCandidates(album)
		for _, key := range candidates[id] {
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	if len(keys) == 0 {
		return nil, nil
	}

	encoded, err := json.Marshal(keys)
	if err != nil {
		return nil, err
	}
	rows, err := app.Queries.GetAlbumsWithoutSpotifyIDByIdentityKeys(ctx, string(encoded))
	if err != nil {
		return nil, fmt.Errorf("failed to look up existing albums by name: %w", err)
	}
	libraryIDs := make(map[[2]string]int64, len(rows))
	for _, row := range rows {
		libraryIDs[[2]string{row.TitleKey, row.ArtistKey}] = row.AlbumID
	}

	existing := make(map[string]int64)
	for id, keys := range candidates {
		for _, key := range keys {
			albumID, found := libraryIDs[key]
			if found {
				existing[id] = albumID
				break
			}
		}
	}
	return existing, nil
}

// albumIdentityCandidates are the alias keys a library copy of the album
// could be saved under, most specific first. A file's album artist tag holds
// the whole credit, spelled the ways tags join names, or one of its artists.
func albumIdentityCandidates(album spotify.SimpleAlbum) [][2]string {
	names := spotifyAlbumArtistNames(album.Artists)
	var artists []string
	if len(names) > 1 {
		artists = append(artists, strings.Join(names, ", "), strings.Join(names, " & "))
	}
	if len(names) > 2 {
		last := len(names) - 1
		artists = append(artists, strings.Join(names[:last], ", ")+" & "+names[last])
	}
	artists = append(artists, names...)

	title := scanner.NormalizedScanCacheKey(album.Name)
	keys := make([][2]string, 0, len(artists))
	for _, artist := range artists {
		keys = append(keys, [2]string{title, scanner.NormalizedScanCacheKey(artist)})
	}
	return keys
}

func (app *Application) mapSpotifyAlbumSearchResults(r *http.Request, albums []spotify.SimpleAlbum) ([]spotifyAlbumSearchResult, error) {
	mapped := make([]spotifyAlbumSearchResult, 0, len(albums))

	existing, err := app.libraryAlbumIDsBySpotifyID(r.Context(), albums)
	if err != nil {
		return nil, err
	}
	existingByName, err := app.libraryAlbumIDsByName(r.Context(), albums, existing)
	if err != nil {
		return nil, err
	}

	for _, album := range albums {
		spotifyID := album.ID.String()
		if spotifyID == "" {
			continue
		}

		result := spotifyAlbumSearchResult{
			SpotifyID:   spotifyID,
			Title:       album.Name,
			ArtistNames: spotifyAlbumArtistNames(album.Artists),
			ReleaseDate: album.ReleaseDate,
			AlbumType:   album.AlbumType,
			TotalTracks: int(album.TotalTracks),
			CoverURL:    firstSpotifyAlbumImageURL(album.Images),
			SpotifyURL:  album.ExternalURLs["spotify"],
		}

		albumID, found := existing[spotifyID]
		if !found {
			albumID, found = existingByName[spotifyID]
		}
		if found {
			result.AlreadyInLibrary = true
			result.LibraryAlbumID = &albumID
		}

		mapped = append(mapped, result)
	}

	return mapped, nil
}

func spotifyAlbumArtistNames(artists []spotify.SimpleArtist) []string {
	names := make([]string, 0, len(artists))

	for _, artist := range artists {
		name := strings.TrimSpace(artist.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}

	return names
}

func firstSpotifyAlbumImageURL(images []spotify.Image) string {
	if len(images) == 0 {
		return ""
	}

	return images[0].URL
}
