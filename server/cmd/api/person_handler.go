package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
)

type personInfo struct {
	TmdbID             int64                  `json:"tmdb_id"`
	Name               string                 `json:"name"`
	Biography          string                 `json:"biography"`
	KnownForDepartment string                 `json:"known_for_department"`
	Birthday           string                 `json:"birthday"`
	Deathday           string                 `json:"deathday"`
	PlaceOfBirth       string                 `json:"place_of_birth"`
	Gender             int                    `json:"gender"`
	AlsoKnownAs        []string               `json:"also_known_as"`
	ProfilePath        string                 `json:"profile_path"`
	Homepage           string                 `json:"homepage"`
	ExternalIDs        tmdb.PersonExternalIDs `json:"external_ids"`
}

type personRole struct {
	Kind       string `json:"kind"`
	Role       string `json:"role"`
	Department string `json:"department"`
}

type personTitle struct {
	TmdbID     int64        `json:"tmdb_id"`
	MediaType  string       `json:"media_type"`
	Title      string       `json:"title"`
	Date       string       `json:"date"`
	PosterPath string       `json:"poster_path"`
	Popularity float64      `json:"popularity"`
	LibraryID  *int64       `json:"library_id"`
	Roles      []personRole `json:"roles"`
}

type personData struct {
	Person           personInfo    `json:"person"`
	Filmography      []personTitle `json:"filmography"`
	Photos           []string      `json:"photos"`
	EnrichmentStatus string        `json:"enrichment_status"`
}

func (app *Application) GetPerson(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r, "id", "invalid person id")
	if !ok {
		return
	}
	ctx := r.Context()
	artist, err := app.Queries.GetArtistByTmdbID(ctx, id)
	local := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		app.serverError(w, err, "failed to get person", "failed to load person")
		return
	}
	data := personData{
		Person:      personInfo{TmdbID: id, AlsoKnownAs: []string{}},
		Filmography: []personTitle{}, Photos: []string{}, EnrichmentStatus: "unconfigured",
	}
	if local {
		data.Person.Name = artist.Name
		data.Person.ProfilePath = artist.Profile.String
	}
	var provider *tmdb.Person
	if app.Tmdb != nil {
		provider, err = app.Tmdb.GetPersonDetails(ctx, int(id))
		data.EnrichmentStatus = "available"
		if err != nil {
			data.EnrichmentStatus = "unavailable"
			if tmdb.IsNotFound(err) {
				data.EnrichmentStatus = "not_found"
			}
			app.Logger.Warn("person enrichment failed", "error", err, "tmdb_id", id)
		}
	}
	if provider == nil && !local {
		switch data.EnrichmentStatus {
		case "not_found":
			helpers.ErrorJSON(w, errors.New("person not found"), http.StatusNotFound)
		default:
			helpers.ErrorJSON(w, errors.New("person information is unavailable"), http.StatusServiceUnavailable)
		}
		return
	}
	groups := make(map[string]*personTitle)
	if provider != nil {
		data.Person = personInfo{
			TmdbID: id, Name: provider.Name, Biography: provider.Biography,
			KnownForDepartment: provider.KnownForDepartment, Birthday: provider.Birthday,
			Deathday: provider.Deathday, PlaceOfBirth: provider.PlaceOfBirth, Gender: provider.Gender,
			AlsoKnownAs: provider.AlsoKnownAs, ProfilePath: provider.ProfilePath,
			Homepage: provider.Homepage, ExternalIDs: provider.ExternalIDs,
		}
		if data.Person.AlsoKnownAs == nil {
			data.Person.AlsoKnownAs = []string{}
		}
		if data.Person.ProfilePath == "" && local {
			data.Person.ProfilePath = artist.Profile.String
		}
		seenPhotos := map[string]bool{data.Person.ProfilePath: true, "": true}
		for _, photo := range provider.Images.Profiles {
			if !seenPhotos[photo.FilePath] {
				data.Photos = append(data.Photos, photo.FilePath)
				seenPhotos[photo.FilePath] = true
			}
		}
		for _, credit := range provider.CombinedCredits.Cast {
			mergePersonCredit(groups, credit, "acting")
		}
		for _, credit := range provider.CombinedCredits.Crew {
			mergePersonCredit(groups, credit, "crew")
		}
	}
	if local {
		credits, queryErr := app.Queries.GetPersonLocalCredits(ctx, artist.ID)
		if queryErr != nil {
			app.serverError(w, queryErr, "failed to get person credits", "failed to load person credits")
			return
		}
		for _, c := range credits {
			key := fmt.Sprintf("%s:%d", c.MediaType, c.TmdbID.Int64)
			// A stored title without a provider identity still has a usable library page.
			if !c.TmdbID.Valid {
				key = fmt.Sprintf("%s:local:%d", c.MediaType, c.LibraryID)
			}
			title := groups[key]
			if title == nil {
				title = &personTitle{TmdbID: c.TmdbID.Int64, MediaType: c.MediaType, Title: c.Title, Date: c.Date, PosterPath: c.PosterPath, Roles: []personRole{}}
				groups[key] = title
			}
			if title.Date == "" {
				title.Date = c.Date
			}
			if title.PosterPath == "" {
				title.PosterPath = c.PosterPath
			}
			if title.LibraryID == nil || c.LibraryID < *title.LibraryID {
				libraryID := c.LibraryID
				title.LibraryID = &libraryID
			}
			appendPersonRole(title, personRole{Kind: c.Kind, Role: c.Role, Department: c.Department})
		}
	}
	err = app.matchPersonLibrary(ctx, groups)
	if err != nil {
		app.serverError(w, err, "failed to match person library credits", "failed to load person credits")
		return
	}
	for _, title := range groups {
		sort.Slice(title.Roles, func(i, j int) bool {
			a, b := title.Roles[i], title.Roles[j]
			return a.Kind+"\x00"+a.Department+"\x00"+a.Role < b.Kind+"\x00"+b.Department+"\x00"+b.Role
		})
		data.Filmography = append(data.Filmography, *title)
	}
	sort.Slice(data.Filmography, func(i, j int) bool {
		a, b := data.Filmography[i], data.Filmography[j]
		if a.Date != b.Date {
			return a.Date > b.Date
		}
		if a.Title != b.Title {
			return a.Title < b.Title
		}
		if a.MediaType != b.MediaType {
			return a.MediaType < b.MediaType
		}
		return a.TmdbID < b.TmdbID
	})
	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: data})
}

func mergePersonCredit(groups map[string]*personTitle, c tmdb.PersonCredit, kind string) {
	if c.ID <= 0 || (c.MediaType != "movie" && c.MediaType != "tv") {
		return
	}
	name, date := c.Title, c.ReleaseDate
	if c.MediaType == "tv" {
		name, date = c.Name, c.FirstAirDate
	}
	if strings.TrimSpace(name) == "" {
		return
	}
	key := fmt.Sprintf("%s:%d", c.MediaType, c.ID)
	title := groups[key]
	if title == nil {
		title = &personTitle{TmdbID: c.ID, MediaType: c.MediaType, Title: name, Date: date, PosterPath: c.PosterPath, Popularity: c.Popularity, Roles: []personRole{}}
		groups[key] = title
	}
	role, department := c.Job, c.Department
	if kind == "acting" {
		role, department = c.Character, "Acting"
	}
	appendPersonRole(title, personRole{Kind: kind, Role: role, Department: department})
}

func appendPersonRole(title *personTitle, role personRole) {
	for _, existing := range title.Roles {
		if existing == role {
			return
		}
	}
	title.Roles = append(title.Roles, role)
}

func (app *Application) matchPersonLibrary(ctx context.Context, groups map[string]*personTitle) error {
	ids := map[string][]sql.NullInt64{"movie": {}, "tv": {}}
	for _, title := range groups {
		if title.TmdbID > 0 {
			ids[title.MediaType] = append(ids[title.MediaType], helpers.NullInt64(title.TmdbID))
		}
	}
	for mediaType, all := range ids {
		// Bound SQLite parameters even for unusually large filmographies.
		for start := 0; start < len(all); start += 500 {
			batch := all[start:min(start+500, len(all))]
			if mediaType == "movie" {
				rows, err := app.Queries.GetPersonLibraryMovies(ctx, batch)
				if err != nil {
					return err
				}
				for _, row := range rows {
					id := row.ID
					groups[fmt.Sprintf("movie:%d", row.TmdbID.Int64)].LibraryID = &id
				}
			} else {
				rows, err := app.Queries.GetPersonLibraryShows(ctx, batch)
				if err != nil {
					return err
				}
				for _, row := range rows {
					id := row.ID
					groups[fmt.Sprintf("tv:%d", row.TmdbID.Int64)].LibraryID = &id
				}
			}
		}
	}
	return nil
}
