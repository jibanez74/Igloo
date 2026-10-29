package artwork

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

// A copy is stored as <dir>/<key>/<stem><ext> under the static directory: dir
// names the kind of entity, key is its provider ID, stem joins the slug of its
// title or name to the image's role, and ext follows the type the downloaded
// bytes sniff as. The API serves it as STATIC_URL_PREFIX plus that path and
// ?v=, a hash of the source and size: static files are cached for a year, and
// a replaced image keeps its name.

// column is one image column the syncer manages.
type column struct {
	label string // names the column in logs
	dir   string
	role  string
	// size is the TMDB size a TMDB path source is fetched at; empty for a
	// column whose sources are full Spotify or Apple URLs, fetched as stored.
	size string
	set  func(ctx context.Context, q *database.Queries, id int64, value, source sql.NullString) (int64, error)
	// revert points every copy in the column back at its source.
	revert func(q *database.Queries, ctx context.Context) (int64, error)
}

var (
	moviePoster = &column{
		label: "movie poster", dir: "movies", role: "poster", size: helpers.TMDB_POSTER_SIZE,
		set: func(ctx context.Context, q *database.Queries, id int64, value, source sql.NullString) (int64, error) {
			return q.SetMoviePosterArtwork(ctx, database.SetMoviePosterArtworkParams{ID: id, Value: value, Source: source})
		},
		revert: (*database.Queries).RevertMoviePosterArtwork,
	}
	movieBackdrop = &column{
		label: "movie backdrop", dir: "movies", role: "backdrop", size: helpers.TMDB_BACKDROP_SIZE,
		set: func(ctx context.Context, q *database.Queries, id int64, value, source sql.NullString) (int64, error) {
			return q.SetMovieBackdropArtwork(ctx, database.SetMovieBackdropArtworkParams{ID: id, Value: value, Source: source})
		},
		revert: (*database.Queries).RevertMovieBackdropArtwork,
	}
	// Cast and crew of movies and shows are the same artist rows, so a person
	// has one folder whatever credits them.
	artistProfile = &column{
		label: "person profile", dir: "people", role: "profile", size: helpers.TMDB_PROFILE_SIZE,
		set: func(ctx context.Context, q *database.Queries, id int64, value, source sql.NullString) (int64, error) {
			return q.SetArtistProfileArtwork(ctx, database.SetArtistProfileArtworkParams{ID: id, Value: value, Source: source})
		},
		revert: (*database.Queries).RevertArtistProfileArtwork,
	}

	showPoster = &column{
		label: "show poster", dir: "shows", role: "poster", size: helpers.TMDB_POSTER_SIZE,
		set: func(ctx context.Context, q *database.Queries, id int64, value, source sql.NullString) (int64, error) {
			return q.SetShowPosterArtwork(ctx, database.SetShowPosterArtworkParams{ID: id, Value: value, Source: source})
		},
		revert: (*database.Queries).RevertShowPosterArtwork,
	}
	showBackdrop = &column{
		label: "show backdrop", dir: "shows", role: "backdrop", size: helpers.TMDB_BACKDROP_SIZE,
		set: func(ctx context.Context, q *database.Queries, id int64, value, source sql.NullString) (int64, error) {
			return q.SetShowBackdropArtwork(ctx, database.SetShowBackdropArtworkParams{ID: id, Value: value, Source: source})
		},
		revert: (*database.Queries).RevertShowBackdropArtwork,
	}
	// Seasons and episodes share their show's folder, so their stems name
	// them by number.
	seasonPoster = &column{
		label: "season poster", dir: "shows", role: "poster", size: helpers.TMDB_POSTER_SIZE,
		set: func(ctx context.Context, q *database.Queries, id int64, value, source sql.NullString) (int64, error) {
			return q.SetShowSeasonPosterArtwork(ctx, database.SetShowSeasonPosterArtworkParams{ID: id, Value: value, Source: source})
		},
		revert: (*database.Queries).RevertShowSeasonPosterArtwork,
	}
	episodeStill = &column{
		label: "episode still", dir: "shows", role: "still", size: helpers.TMDB_STILL_SIZE,
		set: func(ctx context.Context, q *database.Queries, id int64, value, source sql.NullString) (int64, error) {
			return q.SetShowEpisodeStillArtwork(ctx, database.SetShowEpisodeStillArtworkParams{ID: id, Value: value, Source: source})
		},
		revert: (*database.Queries).RevertShowEpisodeStillArtwork,
	}
	networkLogo = &column{
		label: "network logo", dir: "networks", role: "logo", size: helpers.TMDB_LOGO_SIZE,
		set: func(ctx context.Context, q *database.Queries, id int64, value, source sql.NullString) (int64, error) {
			return q.SetNetworkLogoArtwork(ctx, database.SetNetworkLogoArtworkParams{ID: id, Value: value, Source: source})
		},
		revert: (*database.Queries).RevertNetworkLogoArtwork,
	}
	albumCover = &column{
		label: "album cover", dir: "albums", role: "cover",
		set: func(ctx context.Context, q *database.Queries, id int64, value, source sql.NullString) (int64, error) {
			return q.SetAlbumCoverArtwork(ctx, database.SetAlbumCoverArtworkParams{ID: id, Value: value, Source: source})
		},
		revert: (*database.Queries).RevertAlbumCoverArtwork,
	}
	musicianThumb = &column{
		label: "musician photo", dir: "musicians", role: "thumb",
		set: func(ctx context.Context, q *database.Queries, id int64, value, source sql.NullString) (int64, error) {
			return q.SetMusicianThumbArtwork(ctx, database.SetMusicianThumbArtworkParams{ID: id, Value: value, Source: source})
		},
		revert: (*database.Queries).RevertMusicianThumbArtwork,
	}
)

var columns = []*column{moviePoster, movieBackdrop, artistProfile, showPoster, showBackdrop, seasonPoster, episodeStill, networkLogo, albumCover, musicianThumb}

// image is one managed image column of one catalog row.
type image struct {
	column *column
	rowID  int64
	key    string // the provider ID naming the folder; "" leaves the image remote
	stem   string
	source sql.NullString
	value  sql.NullString // what the API returns now
}

// table lists the managed images of one catalog table, each row giving one
// image per managed column. list reads a page of the rows after afterID, every
// row with an image or, when pending, only those with an image that still
// returns its source; it returns the last row id read and the number of rows,
// which is below pageSize on the last page. listInFolders reads every row
// whose key, one of the JSON array keys, names a folder of dir.
type table struct {
	name          string
	dir           string
	list          func(ctx context.Context, q *database.Queries, afterID int64, pending bool) ([]image, int64, int, error)
	listInFolders func(ctx context.Context, q *database.Queries, keys string) ([]image, error)
}

var tables = []table{
	{name: "movies", dir: moviePoster.dir, list: listMovieImages, listInFolders: listMovieImagesInFolders},
	{name: "artist", dir: artistProfile.dir, list: listArtistImages, listInFolders: listArtistImagesInFolders},
	{name: "shows", dir: showPoster.dir, list: listShowImages, listInFolders: listShowImagesInFolders},
	{name: "show_seasons", dir: seasonPoster.dir, list: listSeasonImages, listInFolders: listSeasonImagesInFolders},
	{name: "show_episodes", dir: episodeStill.dir, list: listEpisodeImages, listInFolders: listEpisodeImagesInFolders},
	{name: "networks", dir: networkLogo.dir, list: listNetworkImages, listInFolders: listNetworkImagesInFolders},
	{name: "albums", dir: albumCover.dir, list: listAlbumImages, listInFolders: listAlbumImagesInFolders},
	{name: "musicians", dir: musicianThumb.dir, list: listMusicianImages, listInFolders: listMusicianImagesInFolders},
}

// mapRows maps rows to their images, with the id of the last row and the
// number of rows. Every row gives at least one image, so the last image is the
// last row's.
func mapRows[R any](rows []R, rowImages func(R) []image) ([]image, int64, int) {
	var images []image
	for _, row := range rows {
		images = append(images, rowImages(row)...)
	}

	var lastID int64
	hasImages := len(images) > 0
	if hasImages {
		lastID = images[len(images)-1].rowID
	}

	return images, lastID, len(rows)
}

func movieImages(row database.ListMovieArtworkRow) []image {
	key := tmdbKey(row.TmdbID)
	title := slug(row.Title)
	return []image{
		{column: moviePoster, rowID: row.ID, key: key, stem: title + "_" + moviePoster.role, source: row.PosterPathSource, value: row.PosterPath},
		{column: movieBackdrop, rowID: row.ID, key: key, stem: title + "_" + movieBackdrop.role, source: row.BackdropPathSource, value: row.BackdropPath},
	}
}

func listMovieImages(ctx context.Context, q *database.Queries, afterID int64, pending bool) ([]image, int64, int, error) {
	page := database.ListMovieArtworkParams{AfterID: afterID, PageSize: pageSize}
	if pending {
		rows, err := q.ListPendingMovieArtwork(ctx, database.ListPendingMovieArtworkParams(page))
		if err != nil {
			return nil, 0, 0, err
		}
		images, lastID, n := mapRows(rows, func(row database.ListPendingMovieArtworkRow) []image {
			return movieImages(database.ListMovieArtworkRow(row))
		})
		return images, lastID, n, nil
	}

	rows, err := q.ListMovieArtwork(ctx, page)
	if err != nil {
		return nil, 0, 0, err
	}
	images, lastID, n := mapRows(rows, movieImages)
	return images, lastID, n, nil
}

func listMovieImagesInFolders(ctx context.Context, q *database.Queries, keys string) ([]image, error) {
	rows, err := q.ListMovieArtworkInFolders(ctx, keys)
	if err != nil {
		return nil, err
	}
	images, _, _ := mapRows(rows, func(row database.ListMovieArtworkInFoldersRow) []image {
		return movieImages(database.ListMovieArtworkRow(row))
	})
	return images, nil
}

func artistImages(row database.ListArtistArtworkRow) []image {
	return []image{{
		column: artistProfile, rowID: row.ID, key: strconv.FormatInt(row.TmdbID, 10),
		stem: slug(row.Name) + "_" + artistProfile.role, source: row.ProfileSource, value: row.Profile,
	}}
}

func listArtistImages(ctx context.Context, q *database.Queries, afterID int64, pending bool) ([]image, int64, int, error) {
	page := database.ListArtistArtworkParams{AfterID: afterID, PageSize: pageSize}
	if pending {
		rows, err := q.ListPendingArtistArtwork(ctx, database.ListPendingArtistArtworkParams(page))
		if err != nil {
			return nil, 0, 0, err
		}
		images, lastID, n := mapRows(rows, func(row database.ListPendingArtistArtworkRow) []image {
			return artistImages(database.ListArtistArtworkRow(row))
		})
		return images, lastID, n, nil
	}

	rows, err := q.ListArtistArtwork(ctx, page)
	if err != nil {
		return nil, 0, 0, err
	}
	images, lastID, n := mapRows(rows, artistImages)
	return images, lastID, n, nil
}

func listArtistImagesInFolders(ctx context.Context, q *database.Queries, keys string) ([]image, error) {
	rows, err := q.ListArtistArtworkInFolders(ctx, keys)
	if err != nil {
		return nil, err
	}
	images, _, _ := mapRows(rows, func(row database.ListArtistArtworkInFoldersRow) []image {
		return artistImages(database.ListArtistArtworkRow(row))
	})
	return images, nil
}

func showImages(row database.ListShowArtworkRow) []image {
	key := tmdbKey(row.TmdbID)
	name := slug(row.Name)
	return []image{
		{column: showPoster, rowID: row.ID, key: key, stem: name + "_" + showPoster.role, source: row.PosterPathSource, value: row.PosterPath},
		{column: showBackdrop, rowID: row.ID, key: key, stem: name + "_" + showBackdrop.role, source: row.BackdropPathSource, value: row.BackdropPath},
	}
}

func listShowImages(ctx context.Context, q *database.Queries, afterID int64, pending bool) ([]image, int64, int, error) {
	page := database.ListShowArtworkParams{AfterID: afterID, PageSize: pageSize}
	if pending {
		rows, err := q.ListPendingShowArtwork(ctx, database.ListPendingShowArtworkParams(page))
		if err != nil {
			return nil, 0, 0, err
		}
		images, lastID, n := mapRows(rows, func(row database.ListPendingShowArtworkRow) []image {
			return showImages(database.ListShowArtworkRow(row))
		})
		return images, lastID, n, nil
	}

	rows, err := q.ListShowArtwork(ctx, page)
	if err != nil {
		return nil, 0, 0, err
	}
	images, lastID, n := mapRows(rows, showImages)
	return images, lastID, n, nil
}

func listShowImagesInFolders(ctx context.Context, q *database.Queries, keys string) ([]image, error) {
	rows, err := q.ListShowArtworkInFolders(ctx, keys)
	if err != nil {
		return nil, err
	}
	images, _, _ := mapRows(rows, func(row database.ListShowArtworkInFoldersRow) []image {
		return showImages(database.ListShowArtworkRow(row))
	})
	return images, nil
}

func seasonImages(row database.ListShowSeasonArtworkRow) []image {
	return []image{{
		column: seasonPoster, rowID: row.ID, key: tmdbKey(row.ShowTmdbID),
		stem:   fmt.Sprintf("season_%02d_%s", row.SeasonNumber, seasonPoster.role),
		source: row.PosterPathSource, value: row.PosterPath,
	}}
}

func listSeasonImages(ctx context.Context, q *database.Queries, afterID int64, pending bool) ([]image, int64, int, error) {
	page := database.ListShowSeasonArtworkParams{AfterID: afterID, PageSize: pageSize}
	if pending {
		rows, err := q.ListPendingShowSeasonArtwork(ctx, database.ListPendingShowSeasonArtworkParams(page))
		if err != nil {
			return nil, 0, 0, err
		}
		images, lastID, n := mapRows(rows, func(row database.ListPendingShowSeasonArtworkRow) []image {
			return seasonImages(database.ListShowSeasonArtworkRow(row))
		})
		return images, lastID, n, nil
	}

	rows, err := q.ListShowSeasonArtwork(ctx, page)
	if err != nil {
		return nil, 0, 0, err
	}
	images, lastID, n := mapRows(rows, seasonImages)
	return images, lastID, n, nil
}

func listSeasonImagesInFolders(ctx context.Context, q *database.Queries, keys string) ([]image, error) {
	rows, err := q.ListShowSeasonArtworkInFolders(ctx, keys)
	if err != nil {
		return nil, err
	}
	images, _, _ := mapRows(rows, func(row database.ListShowSeasonArtworkInFoldersRow) []image {
		return seasonImages(database.ListShowSeasonArtworkRow(row))
	})
	return images, nil
}

func episodeImages(row database.ListShowEpisodeArtworkRow) []image {
	return []image{{
		column: episodeStill, rowID: row.ID, key: tmdbKey(row.ShowTmdbID),
		stem:   fmt.Sprintf("s%02de%02d_%s", row.SeasonNumber, row.EpisodeNumber, episodeStill.role),
		source: row.StillPathSource, value: row.StillPath,
	}}
}

func listEpisodeImages(ctx context.Context, q *database.Queries, afterID int64, pending bool) ([]image, int64, int, error) {
	page := database.ListShowEpisodeArtworkParams{AfterID: afterID, PageSize: pageSize}
	if pending {
		rows, err := q.ListPendingShowEpisodeArtwork(ctx, database.ListPendingShowEpisodeArtworkParams(page))
		if err != nil {
			return nil, 0, 0, err
		}
		images, lastID, n := mapRows(rows, func(row database.ListPendingShowEpisodeArtworkRow) []image {
			return episodeImages(database.ListShowEpisodeArtworkRow(row))
		})
		return images, lastID, n, nil
	}

	rows, err := q.ListShowEpisodeArtwork(ctx, page)
	if err != nil {
		return nil, 0, 0, err
	}
	images, lastID, n := mapRows(rows, episodeImages)
	return images, lastID, n, nil
}

func listEpisodeImagesInFolders(ctx context.Context, q *database.Queries, keys string) ([]image, error) {
	rows, err := q.ListShowEpisodeArtworkInFolders(ctx, keys)
	if err != nil {
		return nil, err
	}
	images, _, _ := mapRows(rows, func(row database.ListShowEpisodeArtworkInFoldersRow) []image {
		return episodeImages(database.ListShowEpisodeArtworkRow(row))
	})
	return images, nil
}

func networkImages(row database.ListNetworkArtworkRow) []image {
	return []image{{
		column: networkLogo, rowID: row.ID, key: strconv.FormatInt(row.TmdbID, 10),
		stem: slug(row.Name) + "_" + networkLogo.role, source: row.LogoSource, value: row.Logo,
	}}
}

func listNetworkImages(ctx context.Context, q *database.Queries, afterID int64, pending bool) ([]image, int64, int, error) {
	page := database.ListNetworkArtworkParams{AfterID: afterID, PageSize: pageSize}
	if pending {
		rows, err := q.ListPendingNetworkArtwork(ctx, database.ListPendingNetworkArtworkParams(page))
		if err != nil {
			return nil, 0, 0, err
		}
		images, lastID, n := mapRows(rows, func(row database.ListPendingNetworkArtworkRow) []image {
			return networkImages(database.ListNetworkArtworkRow(row))
		})
		return images, lastID, n, nil
	}

	rows, err := q.ListNetworkArtwork(ctx, page)
	if err != nil {
		return nil, 0, 0, err
	}
	images, lastID, n := mapRows(rows, networkImages)
	return images, lastID, n, nil
}

func listNetworkImagesInFolders(ctx context.Context, q *database.Queries, keys string) ([]image, error) {
	rows, err := q.ListNetworkArtworkInFolders(ctx, keys)
	if err != nil {
		return nil, err
	}
	images, _, _ := mapRows(rows, func(row database.ListNetworkArtworkInFoldersRow) []image {
		return networkImages(database.ListNetworkArtworkRow(row))
	})
	return images, nil
}

func albumImages(row database.ListAlbumArtworkRow) []image {
	key := providerKey(row.SpotifyID)
	if key == "" && row.AppleID.Valid {
		key = strconv.FormatInt(row.AppleID.Int64, 10)
	}
	return []image{{
		column: albumCover, rowID: row.ID, key: key,
		stem: slug(row.Title) + "_" + albumCover.role, source: row.CoverSource, value: row.Cover,
	}}
}

func listAlbumImages(ctx context.Context, q *database.Queries, afterID int64, pending bool) ([]image, int64, int, error) {
	page := database.ListAlbumArtworkParams{AfterID: afterID, PageSize: pageSize}
	if pending {
		rows, err := q.ListPendingAlbumArtwork(ctx, database.ListPendingAlbumArtworkParams(page))
		if err != nil {
			return nil, 0, 0, err
		}
		images, lastID, n := mapRows(rows, func(row database.ListPendingAlbumArtworkRow) []image {
			return albumImages(database.ListAlbumArtworkRow(row))
		})
		return images, lastID, n, nil
	}

	rows, err := q.ListAlbumArtwork(ctx, page)
	if err != nil {
		return nil, 0, 0, err
	}
	images, lastID, n := mapRows(rows, albumImages)
	return images, lastID, n, nil
}

func listAlbumImagesInFolders(ctx context.Context, q *database.Queries, keys string) ([]image, error) {
	rows, err := q.ListAlbumArtworkInFolders(ctx, keys)
	if err != nil {
		return nil, err
	}
	images, _, _ := mapRows(rows, func(row database.ListAlbumArtworkInFoldersRow) []image {
		return albumImages(database.ListAlbumArtworkRow(row))
	})
	return images, nil
}

func musicianImages(row database.ListMusicianArtworkRow) []image {
	return []image{{
		column: musicianThumb, rowID: row.ID, key: providerKey(row.SpotifyID),
		stem: slug(row.Name) + "_" + musicianThumb.role, source: row.ThumbSource, value: row.Thumb,
	}}
}

func listMusicianImages(ctx context.Context, q *database.Queries, afterID int64, pending bool) ([]image, int64, int, error) {
	page := database.ListMusicianArtworkParams{AfterID: afterID, PageSize: pageSize}
	if pending {
		rows, err := q.ListPendingMusicianArtwork(ctx, database.ListPendingMusicianArtworkParams(page))
		if err != nil {
			return nil, 0, 0, err
		}
		images, lastID, n := mapRows(rows, func(row database.ListPendingMusicianArtworkRow) []image {
			return musicianImages(database.ListMusicianArtworkRow(row))
		})
		return images, lastID, n, nil
	}

	rows, err := q.ListMusicianArtwork(ctx, page)
	if err != nil {
		return nil, 0, 0, err
	}
	images, lastID, n := mapRows(rows, musicianImages)
	return images, lastID, n, nil
}

func listMusicianImagesInFolders(ctx context.Context, q *database.Queries, keys string) ([]image, error) {
	rows, err := q.ListMusicianArtworkInFolders(ctx, keys)
	if err != nil {
		return nil, err
	}
	images, _, _ := mapRows(rows, func(row database.ListMusicianArtworkInFoldersRow) []image {
		return musicianImages(database.ListMusicianArtworkRow(row))
	})
	return images, nil
}

// providerKey names the folder of an entity matched on Spotify, whose IDs are
// base62; anything else leaves its images remote.
func providerKey(id sql.NullString) string {
	isPlain := id.Valid && helpers.IsPlainPathSegment(id.String, "")
	if !isPlain {
		return ""
	}
	return id.String
}

// tmdbKey names the folder of an entity matched on TMDB; an unmatched one has
// none, so its images stay remote.
func tmdbKey(id sql.NullInt64) string {
	if !id.Valid {
		return ""
	}
	return strconv.FormatInt(id.Int64, 10)
}

// managedDirs lists the folders under the static directory the syncer owns.
func managedDirs() []string {
	var dirs []string
	for _, c := range columns {
		known := false
		for _, dir := range dirs {
			if dir == c.dir {
				known = true
			}
		}
		if !known {
			dirs = append(dirs, c.dir)
		}
	}

	return dirs
}

const maxSlugLength = 64

// slug folds s to lowercase ASCII letters and digits joined by single
// underscores, so a file name needs no escaping in a URL: "Amélie" becomes
// "amelie" and "Spider-Man: No Way Home" "spider_man_no_way_home".
// Apostrophes join words rather than split them. A name with nothing left,
// such as one in another script, becomes "untitled"; the folder, the
// provider ID, still tells copies apart.
func slug(s string) string {
	var b strings.Builder
	separate := false
	for _, r := range norm.NFD.String(s) {
		isMark := unicode.Is(unicode.Mn, r)
		isApostrophe := r == '\'' || r == '’'
		if isMark || isApostrophe {
			continue
		}

		r = unicode.ToLower(r)
		isWordChar := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if !isWordChar {
			separate = b.Len() > 0
			continue
		}

		if b.Len()+2 > maxSlugLength {
			break
		}
		if separate {
			b.WriteByte('_')
			separate = false
		}
		b.WriteRune(r)
	}

	if b.Len() == 0 {
		return "untitled"
	}

	return b.String()
}

// version names the image a source at size gives, for the ?v= of its URL.
func version(size, source string) string {
	sum := sha256.Sum256([]byte(size + "|" + source))
	return hex.EncodeToString(sum[:4])
}

// sourceURL is where source is fetched from: a TMDB path at the column's size,
// or a Spotify or Apple URL as it is. It is "" when the image stays remote:
// for a malformed path or URL, and for an SVG, which is never served from the
// API's origin.
func (c *column) sourceURL(tmdbBaseURL, source string) string {
	isURLColumn := c.size == ""
	if isURLColumn {
		return remoteImageURL(source)
	}

	file, hasSlash := strings.CutPrefix(source, "/")
	isPlainFile := hasSlash && helpers.IsPlainPathSegment(file, ".") && !strings.Contains(file, "..")
	if !isPlainFile {
		return ""
	}

	isSVG := strings.EqualFold(path.Ext(file), ".svg")
	if isSVG {
		return ""
	}

	return tmdbBaseURL + "/" + c.size + "/" + file
}

// remoteImageURL is source when it is an http or https URL with a host and no
// SVG path, and "" otherwise.
func remoteImageURL(source string) string {
	parsed, err := url.Parse(source)
	if err != nil {
		return ""
	}

	isWeb := parsed.Scheme == "http" || parsed.Scheme == "https"
	isSVG := strings.EqualFold(path.Ext(parsed.Path), ".svg")
	if !isWeb || parsed.Host == "" || isSVG {
		return ""
	}

	return source
}

// localURL is the API URL of the copy at rel, a slash-separated path under
// the static directory.
func localURL(rel, version string) string {
	return helpers.STATIC_URL_PREFIX + rel + "?v=" + version
}

// parseLocalURL splits a value localURL made into the copy's path and
// version; ok is false for any other value.
func parseLocalURL(value string) (rel, version string, ok bool) {
	rest, isLocal := strings.CutPrefix(value, helpers.STATIC_URL_PREFIX)
	if !isLocal {
		return "", "", false
	}

	return strings.Cut(rest, "?v=")
}

// isManagedFile reports whether name, in a key folder of dir, is a file the
// syncer writes: a copy named <slug>_<role><ext>, or the temporary file of an
// interrupted write. Anything else in the folder is left alone.
func isManagedFile(dir, name string) bool {
	isTemporary := strings.HasPrefix(name, helpers.TEMP_FILE_PREFIX)
	if isTemporary {
		return true
	}

	ext := path.Ext(name)
	if !helpers.IsImageExtension(ext) {
		return false
	}

	base := strings.TrimSuffix(name, ext)
	for _, c := range columns {
		if c.dir != dir {
			continue
		}
		title, hasRole := strings.CutSuffix(base, "_"+c.role)
		if hasRole && isSlug(title) {
			return true
		}
	}

	return false
}

func isSlug(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		isSlugChar := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
		if !isSlugChar {
			return false
		}
	}

	return true
}
