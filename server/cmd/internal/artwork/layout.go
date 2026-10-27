package artwork

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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
	size  string // the TMDB size the source is fetched at
	set   func(ctx context.Context, q *database.Queries, id int64, value, source sql.NullString) (int64, error)
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
)

var columns = []*column{moviePoster, movieBackdrop, artistProfile}

// image is one managed image column of one catalog row.
type image struct {
	column *column
	rowID  int64
	key    string // the provider ID naming the folder; "" leaves the image remote
	stem   string
	source sql.NullString
	value  sql.NullString // what the API returns now
}

// table lists the managed images of one catalog table a page of rows at a
// time, each row giving one image per managed column. It returns the last row
// id read and the number of rows, which is below pageSize on the last page.
type table struct {
	name string
	list func(ctx context.Context, q *database.Queries, afterID int64) ([]image, int64, int, error)
}

var tables = []table{
	{name: "movies", list: listMovieImages},
	{name: "artist", list: listArtistImages},
}

func listMovieImages(ctx context.Context, q *database.Queries, afterID int64) ([]image, int64, int, error) {
	rows, err := q.ListMovieArtwork(ctx, database.ListMovieArtworkParams{AfterID: afterID, PageSize: pageSize})
	if err != nil {
		return nil, 0, 0, err
	}

	images := make([]image, 0, 2*len(rows))
	var lastID int64
	for _, row := range rows {
		key := ""
		if row.TmdbID.Valid {
			key = strconv.FormatInt(row.TmdbID.Int64, 10)
		}
		title := slug(row.Title)
		images = append(images,
			image{column: moviePoster, rowID: row.ID, key: key, stem: title + "_" + moviePoster.role, source: row.PosterPathSource, value: row.PosterPath},
			image{column: movieBackdrop, rowID: row.ID, key: key, stem: title + "_" + movieBackdrop.role, source: row.BackdropPathSource, value: row.BackdropPath},
		)
		lastID = row.ID
	}

	return images, lastID, len(rows), nil
}

func listArtistImages(ctx context.Context, q *database.Queries, afterID int64) ([]image, int64, int, error) {
	rows, err := q.ListArtistArtwork(ctx, database.ListArtistArtworkParams{AfterID: afterID, PageSize: pageSize})
	if err != nil {
		return nil, 0, 0, err
	}

	images := make([]image, 0, len(rows))
	var lastID int64
	for _, row := range rows {
		images = append(images, image{
			column: artistProfile, rowID: row.ID, key: strconv.FormatInt(row.TmdbID, 10),
			stem: slug(row.Name) + "_" + artistProfile.role, source: row.ProfileSource, value: row.Profile,
		})
		lastID = row.ID
	}

	return images, lastID, len(rows), nil
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

// sourceURL is where a TMDB path source is fetched from, or "" when the
// image stays remote: for a malformed path, and for an SVG, which is never
// served from the API's origin.
func (c *column) sourceURL(tmdbBaseURL, source string) string {
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
