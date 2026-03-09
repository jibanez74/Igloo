package main

import (
	"context"
	"database/sql"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/musicbrainz"
	"strings"
)

func generateMusicianSummary(name, country, artistType, disambiguation string) string {
  var parts []string
  parts = append(parts, name)

  if artistType != "" {
    parts = append(parts, fmt.Sprintf("is a %s", strings.ToLower(artistType)))
  }

  if country != "" {
    parts = append(parts, fmt.Sprintf("from %s", country))
  }

  if disambiguation != "" {
    parts = append(parts, fmt.Sprintf("(%s)", disambiguation))
  }

  return strings.Join(parts, " ") + "."
}

func (app *Application) getOrCreateMusician(ctx context.Context, qtx *database.Queries, name, sortName string) (*database.Musician, error) {
  existing, err := qtx.GetMusicianByName(ctx, name)
  if err == nil {
    return &existing, nil
  }

  artist, err := app.MusicBrainz.SearchArtistByName(name)
  if err == nil && artist != nil {
    existing, err := qtx.GetMusicianByMusicBrainzID(ctx, sql.NullString{String: artist.MusicBrainzID, Valid: true})
    if err == nil {
      return &existing, nil
    }

    summary := generateMusicianSummary(artist.Name, artist.Country, artist.Type, artist.Disambiguation)
    var thumb sql.NullString
    app.throttleCoverArtDownload()
    if thumbURL, thumbErr := app.MusicBrainz.GetArtistImageURL(artist.MusicBrainzID); thumbErr == nil && thumbURL != "" {
      thumb = sql.NullString{String: thumbURL, Valid: true}
    }

    musician, err := qtx.UpsertMusician(ctx, database.UpsertMusicianParams{
      Name:          name,
      SortName:      sortName,
      Summary:       sql.NullString{String: summary, Valid: true},
      MusicbrainzID: sql.NullString{String: artist.MusicBrainzID, Valid: true},
      Thumb:         thumb,
    })

    if err != nil {
      return nil, err
    }

    return &musician, nil
  }

  musician, err := qtx.UpsertMusician(ctx, database.UpsertMusicianParams{
    Name:     name,
    SortName: sortName,
  })

  if err != nil {
    return nil, err
  }

  return &musician, nil
}

func (app *Application) getOrCreateMusicianWithResult(ctx context.Context, qtx *database.Queries, name, sortName string, mbArtist *musicbrainz.ArtistResult) (*database.Musician, error) {
	existing, err := qtx.GetMusicianByName(ctx, name)
	if err == nil {
		return &existing, nil
	}
	if mbArtist != nil {
		existing, err := qtx.GetMusicianByMusicBrainzID(ctx, sql.NullString{String: mbArtist.MusicBrainzID, Valid: true})
		if err == nil {
			return &existing, nil
		}
		summary := generateMusicianSummary(mbArtist.Name, mbArtist.Country, mbArtist.Type, mbArtist.Disambiguation)
		var thumb sql.NullString
		app.throttleCoverArtDownload()
		if thumbURL, thumbErr := app.MusicBrainz.GetArtistImageURL(mbArtist.MusicBrainzID); thumbErr == nil && thumbURL != "" {
			thumb = sql.NullString{String: thumbURL, Valid: true}
		}
		musician, err := qtx.UpsertMusician(ctx, database.UpsertMusicianParams{
			Name:          name,
			SortName:      sortName,
			Summary:       sql.NullString{String: summary, Valid: true},
			MusicbrainzID: sql.NullString{String: mbArtist.MusicBrainzID, Valid: true},
			Thumb:         thumb,
		})
		if err != nil {
			return nil, err
		}
		return &musician, nil
	}
	return app.getOrCreateMusician(ctx, qtx, name, sortName)
}

func (app *Application) getOrCreateAlbum(ctx context.Context, qtx *database.Queries, title, sortTitle, albumArtist string) (*database.Album, string, error) {
  if albumArtist != "" {
    existing, err := qtx.GetAlbumByTitleAndMusician(ctx, database.GetAlbumByTitleAndMusicianParams{
      Title:    title,
      Musician: sql.NullString{String: albumArtist, Valid: true},
    })

    if err == nil {
      return &existing, "", nil
    }
  }

  albumDetails, err := app.MusicBrainz.SearchAlbumByName(title, albumArtist)
  if err == nil && albumDetails != nil {
    existing, err := qtx.GetAlbumByMusicBrainzID(ctx, sql.NullString{String: albumDetails.MusicBrainzID, Valid: true})
    if err == nil {
      return &existing, albumDetails.CoverURL, nil
    }

    params := database.UpsertAlbumParams{
      Title:         title,
      SortTitle:     sortTitle,
      MusicbrainzID: sql.NullString{String: albumDetails.MusicBrainzID, Valid: true},
    }

    if albumDetails.ReleaseDate != "" {
      params.ReleaseDate = sql.NullString{String: albumDetails.ReleaseDate, Valid: true}
    }

    if albumDetails.Year > 0 {
      params.Year = sql.NullInt64{Int64: int64(albumDetails.Year), Valid: true}
    }

    if albumArtist != "" {
      params.Musician = sql.NullString{String: albumArtist, Valid: true}
    }

    if albumDetails.CoverURL != "" {
      params.Cover = sql.NullString{String: albumDetails.CoverURL, Valid: true}
    } else if albumDetails.MusicBrainzID != "" {
      params.Cover = sql.NullString{String: fmt.Sprintf("%s/%s/front-500", helpers.COVER_ART_ARCHIVE_BASE_URL, albumDetails.MusicBrainzID), Valid: true}
    }

    album, err := qtx.UpsertAlbum(ctx, params)
    if err != nil {
      return nil, "", err
    }

    return &album, albumDetails.CoverURL, nil
  }

  params := database.UpsertAlbumParams{
    Title:     title,
    SortTitle: sortTitle,
  }

  if albumArtist != "" {
    params.Musician = sql.NullString{String: albumArtist, Valid: true}
  }

  album, err := qtx.UpsertAlbum(ctx, params)
  if err != nil {
    return nil, "", err
  }

  return &album, "", nil
}

func (app *Application) getOrCreateAlbumWithResult(ctx context.Context, qtx *database.Queries, title, sortTitle, albumArtist string, mbAlbum *musicbrainz.AlbumResult) (*database.Album, string, error) {
	if albumArtist != "" {
		existing, err := qtx.GetAlbumByTitleAndMusician(ctx, database.GetAlbumByTitleAndMusicianParams{
			Title:    title,
			Musician: sql.NullString{String: albumArtist, Valid: true},
		})
		if err == nil {
			return &existing, "", nil
		}
	}
	if mbAlbum != nil {
		existing, err := qtx.GetAlbumByMusicBrainzID(ctx, sql.NullString{String: mbAlbum.MusicBrainzID, Valid: true})
		if err == nil {
			return &existing, mbAlbum.CoverURL, nil
		}
		params := database.UpsertAlbumParams{
			Title:         title,
			SortTitle:     sortTitle,
			MusicbrainzID: sql.NullString{String: mbAlbum.MusicBrainzID, Valid: true},
		}
		if mbAlbum.ReleaseDate != "" {
			params.ReleaseDate = sql.NullString{String: mbAlbum.ReleaseDate, Valid: true}
		}
		if mbAlbum.Year > 0 {
			params.Year = sql.NullInt64{Int64: int64(mbAlbum.Year), Valid: true}
		}
		if albumArtist != "" {
			params.Musician = sql.NullString{String: albumArtist, Valid: true}
		}
		if mbAlbum.CoverURL != "" {
			params.Cover = sql.NullString{String: mbAlbum.CoverURL, Valid: true}
		} else if mbAlbum.MusicBrainzID != "" {
			params.Cover = sql.NullString{String: fmt.Sprintf("%s/%s/front-500", helpers.COVER_ART_ARCHIVE_BASE_URL, mbAlbum.MusicBrainzID), Valid: true}
		}
		album, err := qtx.UpsertAlbum(ctx, params)
		if err != nil {
			return nil, "", err
		}
		return &album, mbAlbum.CoverURL, nil
	}
	return app.getOrCreateAlbum(ctx, qtx, title, sortTitle, albumArtist)
}
