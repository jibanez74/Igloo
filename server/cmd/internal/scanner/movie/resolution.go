package movie

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/tmdbmatch"
	"igloo/cmd/internal/tmdb"
)

type movieScanEntry struct {
	scanner.FileFingerprint
	ID             int64
	FilePath       string
	TmdbID         sql.NullInt64
	PendingRetry   bool
	RetryAttempts  int64
	LastAttemptAt  sql.NullInt64
	HasFingerprint bool
}

// pendingEnrichment reports whether the movie still lacks a confirmed TMDB
// match or was re-queued by a technical change.
func (e movieScanEntry) pendingEnrichment() bool {
	return e.PendingRetry || !e.TmdbID.Valid
}

// enrichmentEligible applies the shared miss backoff to a pending movie.
func (e movieScanEntry) enrichmentEligible(now time.Time) bool {
	return e.pendingEnrichment() && scanner.MissBackoffElapsed(e.RetryAttempts, e.LastAttemptAt, now)
}

type movieScanContext struct {
	// enriched and pending are derived from movieIndex and published by the
	// scan report; they change only through setEntry and deleteEntry.
	enriched int
	pending  int
	// movieIndex holds catalog identities, retry state, and successful fingerprints
	// by cleaned file path. It is only written after a
	// successful commit, never inside a transaction, so it is shared (not copied)
	// across per-movie transactions.
	movieIndex map[string]movieScanEntry
	// genreIDs memoizes genre tag -> id within a scan. It is written inside the
	// per-movie transaction (getOrCreateMovieGenreID), so the clone overlay
	// isolates it until commit to avoid caching ids from a rolled-back
	// transaction.
	genreIDs scanner.ScanCache[string, int64]
	// artistIDs memoizes TMDB person id -> artist.id within a scan, so a person
	// appearing in many movies (or in several crew roles of one movie) is
	// upserted once per scan instead of once per credit. Same overlay rollback
	// isolation as genreIDs. Artist rows are never deleted, so cached ids stay
	// valid for the whole scan.
	artistIDs scanner.ScanCache[int64, int64]
}

func newMovieScanContext(movieIndex map[string]movieScanEntry) *movieScanContext {
	if movieIndex == nil {
		movieIndex = make(map[string]movieScanEntry)
	}

	// Take ownership of movieIndex: loadMovieScanIndex already cleaned its keys
	// and the caller discards its reference, so no defensive copy is needed.
	scan := &movieScanContext{
		movieIndex: movieIndex,
		genreIDs:   scanner.NewScanCache[string, int64](),
		artistIDs:  scanner.NewScanCache[int64, int64](),
	}
	for _, entry := range movieIndex {
		if entry.pendingEnrichment() {
			scan.pending++
		}
	}
	return scan
}

func (scan *movieScanContext) setEntry(path string, entry movieScanEntry) {
	scan.deleteEntry(path)
	if entry.pendingEnrichment() {
		scan.pending++
	}
	scan.movieIndex[path] = entry
}

func (scan *movieScanContext) deleteEntry(path string) {
	previous, exists := scan.movieIndex[path]
	if exists && previous.pendingEnrichment() {
		scan.pending--
	}
	delete(scan.movieIndex, path)
}

func (scan *movieScanContext) clone() *movieScanContext {
	return &movieScanContext{
		movieIndex: scan.movieIndex, // shared; never written inside the transaction
		genreIDs:   scan.genreIDs.Overlay(),
		artistIDs:  scan.artistIDs.Overlay(),
	}
}

func (scan *movieScanContext) mergeFrom(other *movieScanContext) {
	scan.genreIDs.MergeFrom(other.genreIDs)
	scan.artistIDs.MergeFrom(other.artistIDs)
}

// ---------------------------------------------------------------------------
// Filename interpretation, local probing, and TMDB matching
// ---------------------------------------------------------------------------

// localMovie is a probed file waiting to be committed. baseline is the catalog
// identity the scan index held when the probe started; persistence rejects
// the result if the row moved on since.
type localMovie struct {
	baseline   movieScanEntry
	inspection *scanner.FileInspection
	params     database.UpsertMovieParams
	streams    []ffprobe.Stream
	chapters   []ffprobe.Chapter
}

// enrichedMovie is a completed TMDB lookup for a committed movie. A nil
// tmdbMovie is a definitive no-match.
type enrichedMovie struct {
	baseline   movieScanEntry
	inspection *scanner.FileInspection
	tmdbMovie  *tmdb.TmdbMovie
}

func (s *Scanner) resolveLocalMovie(ctx context.Context, file scanner.ScanFile, baseline movieScanEntry) (*localMovie, error) {
	titleYear := movieTitleYear(file.Path)
	info, err := s.ffprobe.GetMetadata(ctx, file.Path)
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed (required): %w", err)
	}

	mimeType := helpers.VideoMimeTypes[file.Ext]
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	params := database.UpsertMovieParams{
		Title:     titleYear.Title,
		FilePath:  file.Path,
		FileName:  filepath.Base(file.Path),
		Size:      file.Size,
		Container: file.Ext,
		MimeType:  mimeType,
		Adult:     false,
	}

	if titleYear.Year > 0 {
		params.Year = helpers.NullInt64(int64(titleYear.Year))
	}

	durationSec, err := strconv.ParseFloat(info.Format.Duration, 64)
	if err == nil && durationSec > 0 {
		params.Duration = helpers.NullFloat64(durationSec)
		runTimeMinutes := int64(math.Round(durationSec / 60))
		if runTimeMinutes > 0 {
			params.RunTime = helpers.NullInt64(runTimeMinutes)
		}
	}

	return &localMovie{
		baseline: baseline,
		params:   params,
		streams:  info.Streams,
		chapters: info.Chapters,
	}, nil
}

func movieTitleYear(path string) *helpers.TitleYearResponse {
	titleYear, err := helpers.GetTitleAndYearFromFileName(filepath.Base(path))
	if err != nil {
		baseName := filepath.Base(path)
		ext := filepath.Ext(baseName)
		titleYear = &helpers.TitleYearResponse{
			Title: strings.TrimSuffix(baseName, ext),
			Year:  0,
		}
	}

	return titleYear
}

// TMDB failures leave enrichment eligible for a later scan.
func (s *Scanner) lookupTmdbMovie(ctx context.Context, path, searchTitle string, year int, identity sql.NullInt64) (*tmdb.TmdbMovie, error) {
	if s.tmdb == nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, scanner.TmdbLookupTimeout)
	defer cancel()
	movie := &tmdb.TmdbMovie{TmdbID: int(identity.Int64)}
	if !identity.Valid {
		interpretations := []tmdbmatch.Interpretation{{Title: searchTitle, Year: year}}
		fullTitle := ambiguousFullTitle(path, year)
		if fullTitle != "" {
			interpretations = append(interpretations, tmdbmatch.Interpretation{Title: fullTitle})
		}
		var candidates tmdbmatch.Candidates
		var searchErr error
		for _, interpretation := range interpretations {
			contextErr := ctx.Err()
			if contextErr != nil {
				return nil, contextErr
			}
			results, err := s.tmdb.SearchMoviesByTitleAndYear(ctx, interpretation.Title, interpretation.Year)
			contextErr = ctx.Err()
			if contextErr != nil {
				return nil, contextErr
			}
			if err != nil {
				if !errors.Is(err, tmdb.ErrNoMoviesFound) {
					authentication, _ := tmdb.ProviderFailure(err)
					if authentication {
						return nil, err
					}
					s.logger.Warn("TMDB movie search failed", "path", path, "error", err)
					searchErr = err
				}
				continue
			}
			for _, target := range interpretations {
				candidates.Add(tmdbmatch.Rank(results, target.Title, target.Year))
			}
		}
		best := candidates.Best()
		if best == nil {
			return nil, searchErr
		}
		movie = best.Movie
	}
	contextErr := ctx.Err()
	if contextErr != nil {
		return nil, contextErr
	}
	err := s.tmdb.GetTmdbMovieByID(ctx, movie)
	contextErr = ctx.Err()
	if contextErr != nil {
		return nil, contextErr
	}
	if err != nil {
		return nil, err
	}
	return movie, nil
}

var parenthesizedYear = regexp.MustCompile(`\(\s*[0-9]{4}\s*\)`)

func ambiguousFullTitle(path string, year int) string {
	if year == 0 || parenthesizedYear.MatchString(filepath.Base(path)) {
		return ""
	}
	base := filepath.Base(path)
	full := tmdbmatch.NormalizeTitleForSearch(strings.TrimSuffix(base, filepath.Ext(base)))
	count := 0
	for _, token := range strings.Fields(full) {
		number, err := strconv.Atoi(token)
		if err == nil && len(token) == 4 && number >= 1900 && number <= 2100 {
			count++
		}
	}
	if count == 1 {
		return full
	}
	return ""
}
