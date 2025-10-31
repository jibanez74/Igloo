package main

import (
	"context"
	"encoding/json"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var TEMPLATES_BASE_PATH = filepath.Join("cmd", "templates")

// Helper types for parsing JSON aggregated data from database
type AlbumGenre struct {
	ID  int32  `json:"id"`
	Tag string `json:"tag"`
}

type AlbumMusician struct {
	ID        int32       `json:"id"`
	Name      string      `json:"name"`
	SortName  string      `json:"sort_name"`
	Thumb     interface{} `json:"thumb"`
	SpotifyID interface{} `json:"spotify_id"`
}

type AlbumTrack struct {
	ID            int32       `json:"id"`
	Title         string      `json:"title"`
	SortTitle     string      `json:"sort_title"`
	Disc          int32       `json:"disc"`
	TrackIndex    int32       `json:"track_index"`
	Duration      interface{} `json:"duration"`
	FilePath      string      `json:"file_path"`
	FileName      string      `json:"file_name"`
	Container     string      `json:"container"`
	Codec         string      `json:"codec"`
	Channels      int32       `json:"channels"`
	ChannelLayout string      `json:"channel_layout"`
	Size          int64       `json:"size"`
	BitRate       int32       `json:"bit_rate"`
}

type AlbumPageData struct {
	ID                int32
	Title             string
	SortTitle         string
	SpotifyID         pgtype.Text
	ReleaseDate       pgtype.Date
	Year              pgtype.Int4
	SpotifyPopularity pgtype.Int4
	TotalTracks       int32
	Musician          pgtype.Text
	Cover             pgtype.Text
	Genres            []AlbumGenre
	Musicians         []AlbumMusician
	Tracks            []AlbumTrack
}

func (app *Application) RouteGetIndexPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles(
		filepath.Join(TEMPLATES_BASE_PATH, "base.html"),
		filepath.Join(TEMPLATES_BASE_PATH, "home.html"),
	)

	if err != nil {
		helpers.ErrorJSON(w, errors.New("fail to parse templates"), http.StatusInternalServerError)
		return
	}

	albums, err := app.Queries.GetLatestAlbums(context.Background())
	if err != nil {
		helpers.ErrorJSON(w, errors.New("fail to get latest albums"), http.StatusInternalServerError)
		return
	}

	data := struct {
		Albums []database.GetLatestAlbumsRow
	}{
		Albums: albums,
	}

	tmpl.Execute(w, data)
}

func (app *Application) RouteGetLoginPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles(
		filepath.Join(TEMPLATES_BASE_PATH, "login.html"),
	)

	if err != nil {
		helpers.ErrorJSON(w, errors.New("fail to parse templates"), http.StatusInternalServerError)
		return
	}

	data := struct{}{}

	tmpl.Execute(w, data)
}

func (app *Application) RouteGetAlbumDetailsPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles(
		filepath.Join(TEMPLATES_BASE_PATH, "base.html"),
		filepath.Join(TEMPLATES_BASE_PATH, "album-details.html"),
	)

	if err != nil {
		helpers.ErrorJSON(w, errors.New("fail to parse templates"), http.StatusInternalServerError)
		return
	}

	albumID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid album ID format"), http.StatusBadRequest)
		return
	}

	albumDetails, err := app.Queries.GetAlbumDetails(context.Background(), int32(albumID))
	if err != nil {
		if err == pgx.ErrNoRows {
			helpers.ErrorJSON(w, errors.New("album not found"), http.StatusNotFound)
		} else {
			helpers.ErrorJSON(w, errors.New("fail to get album details"), http.StatusInternalServerError)
		}

    return
	}

	// Parse JSON aggregated fields
	var genres []AlbumGenre
	if albumDetails.Genres != nil {
		genresJSON, _ := json.Marshal(albumDetails.Genres)
		if err := json.Unmarshal(genresJSON, &genres); err != nil {
			log.Printf("Error unmarshaling genres: %v", err)
		}
	}

	var musicians []AlbumMusician
	if albumDetails.Musicians != nil {
		musiciansJSON, _ := json.Marshal(albumDetails.Musicians)
		if err := json.Unmarshal(musiciansJSON, &musicians); err != nil {
			log.Printf("Error unmarshaling musicians: %v", err)
		}
	}

	var tracks []AlbumTrack
	if albumDetails.Tracks != nil {
		tracksJSON, _ := json.Marshal(albumDetails.Tracks)
		if err := json.Unmarshal(tracksJSON, &tracks); err != nil {
			log.Printf("Error unmarshaling tracks: %v", err)
		}
	}

	// Prepare data for template
	data := struct {
		Album AlbumPageData
	}{
		Album: AlbumPageData{
			ID:                albumDetails.ID,
			Title:             albumDetails.Title,
			SortTitle:         albumDetails.SortTitle,
			SpotifyID:         albumDetails.SpotifyID,
			ReleaseDate:       albumDetails.ReleaseDate,
			Year:              albumDetails.Year,
			SpotifyPopularity: albumDetails.SpotifyPopularity,
			TotalTracks:       albumDetails.TotalTracks,
			Musician:          albumDetails.Musician,
			Cover:             albumDetails.Cover,
			Genres:            genres,
			Musicians:         musicians,
			Tracks:            tracks,
		},
	}

	// Execute template
	tmpl.Execute(w, data)
}
