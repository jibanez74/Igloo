package movie

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/tmdb"
)

type movieScanContext struct {
	// movieIndex maps cleaned file path -> file size for every movie already in
	// the DB. It is read to skip unchanged files and is only written after a
	// successful commit, never inside a transaction, so it is shared (not copied)
	// across per-movie transactions.
	movieIndex map[string]int64
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

func newMovieScanContext(movieIndex map[string]int64) *movieScanContext {
	if movieIndex == nil {
		movieIndex = make(map[string]int64)
	}

	// Take ownership of movieIndex: loadMovieScanIndex already cleaned its keys
	// and the caller discards its reference, so no defensive copy is needed.
	return &movieScanContext{
		movieIndex: movieIndex,
		genreIDs:   scanner.NewScanCache[string, int64](),
		artistIDs:  scanner.NewScanCache[int64, int64](),
	}
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

func (scan *movieScanContext) movieUnchanged(path string, size int64) bool {
	return scanner.ScanIndexUnchanged(scan.movieIndex, path, size)
}

// ---------------------------------------------------------------------------
// Movie resolution (file name -> ffprobe + TMDB metadata)
// ---------------------------------------------------------------------------

type resolvedMovie struct {
	params    database.UpsertMovieParams
	tmdbMovie *tmdb.TmdbMovie
	streams   []ffprobe.Stream
	chapters  []ffprobe.Chapter
}

func (s *Scanner) resolveMovieFile(ctx context.Context, file scanner.ScanFile) (*resolvedMovie, error) {
	titleYear, err := helpers.GetTitleAndYearFromFileName(filepath.Base(file.Path))
	if err != nil {
		baseName := filepath.Base(file.Path)
		ext := filepath.Ext(baseName)
		titleYear = &helpers.TitleYearResponse{
			Title: strings.TrimSuffix(baseName, ext),
			Year:  0,
		}
	}

	searchTitle := NormalizeTitleForSearch(titleYear.Title)
	if searchTitle == "" {
		searchTitle = titleYear.Title
	}

	// ffprobe is required and TMDB is not, so probe first: a file that cannot be
	// probed is discarded either way, and TMDB search results are never cached.
	info, err := s.ffprobe.GetMetadata(ctx, file.Path)
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed (required): %w", err)
	}

	tmdbMovie := s.lookupTmdbMovie(ctx, file.Path, searchTitle, titleYear.Year)

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

	if tmdbMovie != nil {
		params.TmdbID = helpers.NullInt64(int64(tmdbMovie.TmdbID))
		params.ImdbID = helpers.NullString(tmdbMovie.ImdbID)
		params.PosterPath = helpers.NullString(tmdbMovie.PosterPath)
		params.BackdropPath = helpers.NullString(tmdbMovie.BackdropPath)
		params.Title = tmdbMovie.Title
		params.Adult = tmdbMovie.Adult
		params.Language = helpers.NullString(tmdbMovie.OriginalLang)
		params.Overview = helpers.NullString(tmdbMovie.Overview)
		params.TagLine = helpers.NullString(tmdbMovie.Tagline)
		params.Certification = helpers.NullString(tmdbMovie.Certification())
		params.CriticRating = helpers.NullFloat64(tmdbMovie.VoteAverage)
		params.Revenue = helpers.NullFloat64(float64(tmdbMovie.Revenue))
		params.Budget = helpers.NullFloat64(float64(tmdbMovie.Budget))

		if tmdbMovie.ReleaseDate != "" {
			params.ReleaseDate = helpers.NullString(tmdbMovie.ReleaseDate)

			year := extractYearFromReleaseDate(tmdbMovie.ReleaseDate)
			if year > 0 {
				params.Year = helpers.NullInt64(int64(year))
			}
		}
	} else if titleYear.Year > 0 {
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

	return &resolvedMovie{
		params:    params,
		tmdbMovie: tmdbMovie,
		streams:   info.Streams,
		chapters:  info.Chapters,
	}, nil
}

// lowTmdbConfidence is the match confidence below which a TMDB match is
// logged with its runner-up candidates for review.
const lowTmdbConfidence = 70

// lookupTmdbMovie resolves a file's TMDB metadata, returning nil when TMDB is
// not configured, the search finds nothing, or a lookup fails. TMDB is
// optional -- a failure leaves the movie with filename-derived metadata -- so
// failures are logged rather than returned. A canceled scan is not a TMDB
// problem and is not logged.
func (s *Scanner) lookupTmdbMovie(ctx context.Context, path, searchTitle string, year int) *tmdb.TmdbMovie {
	if s.tmdb == nil {
		return nil
	}

	var searchResults []tmdb.TmdbMovie
	var err error
	if year > 0 {
		searchResults, err = s.tmdb.SearchMoviesByTitleAndYear(ctx, searchTitle, year)
	} else {
		searchResults, err = s.tmdb.SearchMoviesByTitleAndYear(ctx, searchTitle)
	}
	if err != nil {
		if ctx.Err() == nil {
			s.logger.Warn(
				"TMDB movie search failed",
				"path", path,
				"parsed_title", searchTitle,
				"parsed_year", year,
				"error", err,
			)
		}
		return nil
	}

	ranked := RankTMDBMovies(searchResults, searchTitle, year)
	if len(ranked) == 0 {
		return nil
	}
	bestMatch := ranked[0]

	err = s.tmdb.GetTmdbMovieByID(ctx, bestMatch.Movie)
	if err != nil {
		if ctx.Err() == nil {
			s.logger.Warn(
				"TMDB movie detail lookup failed",
				"path", path,
				"parsed_title", searchTitle,
				"tmdb_id", bestMatch.Movie.TmdbID,
				"error", err,
			)
		}
		return nil
	}

	if bestMatch.Confidence < lowTmdbConfidence {
		s.logger.Warn(
			"low-confidence TMDB movie match",
			"path", path,
			"parsed_title", searchTitle,
			"parsed_year", year,
			"tmdb_id", bestMatch.Movie.TmdbID,
			"tmdb_title", bestMatch.Movie.Title,
			"tmdb_release_date", bestMatch.Movie.ReleaseDate,
			"confidence", fmt.Sprintf("%.1f", bestMatch.Confidence),
			"alternatives", summarizeTmdbCandidates(ranked),
		)
	}

	return bestMatch.Movie
}

func extractYearFromReleaseDate(releaseDate string) int {
	if releaseDate == "" {
		return 0
	}

	parsed, err := helpers.ParseDate(releaseDate)
	if err != nil {
		return 0
	}

	return parsed.Year()
}

// ---------------------------------------------------------------------------
// TMDB match ranking
// ---------------------------------------------------------------------------

// Exact year matches must outrank TMDB popularity and vote averages.
const tmdbYearMatchScore = 10000.0

type TMDBMovieMatch struct {
	Movie      *tmdb.TmdbMovie
	Score      float64
	Confidence float64
}

// RankTMDBMovies orders TMDB results by the scanner's title and year score.
func RankTMDBMovies(results []tmdb.TmdbMovie, targetTitle string, targetYear int) []*TMDBMovieMatch {
	if len(results) == 0 {
		return nil
	}

	normalizedTarget := normalizeComparableMovieTitle(targetTitle)
	targetSequel := sequelIndicator(normalizedTarget)

	scoredMatches := make([]*TMDBMovieMatch, 0, len(results))
	for i := range results {
		movie := &results[i]
		score := scoreTmdbCandidate(normalizedTarget, targetSequel, targetYear, movie)
		scoredMatches = append(scoredMatches, &TMDBMovieMatch{
			Movie:      movie,
			Score:      score,
			Confidence: clampTmdbConfidence(score),
		})
	}

	slices.SortFunc(scoredMatches, func(a, b *TMDBMovieMatch) int {
		switch {
		case a.Score > b.Score:
			return -1
		case a.Score < b.Score:
			return 1
		default:
			return strings.Compare(a.Movie.Title, b.Movie.Title)
		}
	})

	return scoredMatches
}

// scoreTmdbCandidate scores one candidate against an already-normalized target
// title and its sequel indicator, both of which are loop-invariant across a
// ranking pass.
func scoreTmdbCandidate(normalizedTarget, targetSequel string, targetYear int, movie *tmdb.TmdbMovie) float64 {
	normalizedTitle := normalizeComparableMovieTitle(movie.Title)
	normalizedOriginalTitle := normalizeComparableMovieTitle(movie.OriginalTitle)

	score := 0.0

	switch {
	case normalizedTitle == normalizedTarget:
		score += 60
	case normalizedOriginalTitle == normalizedTarget:
		score += 56
	case normalizedTarget != "" && (strings.Contains(normalizedTitle, normalizedTarget) || strings.Contains(normalizedTarget, normalizedTitle)):
		score += 38
	}

	score += tokenOverlapScore(normalizedTarget, normalizedTitle) * 35
	score += tokenOverlapScore(normalizedTarget, normalizedOriginalTitle) * 20

	if targetSequel == sequelIndicator(normalizedTitle) {
		score += 8
	} else if targetSequel != "" {
		score -= 12
	}

	movieYear := extractYearFromReleaseDate(movie.ReleaseDate)
	if targetYear > 0 {
		switch {
		case movieYear == targetYear:
			score += tmdbYearMatchScore
		case movieYear > 0 && absInt(movieYear-targetYear) == 1:
			score += 12
		case movieYear > 0:
			score -= 15
		}
	}

	score += min(movie.Popularity/25, 8)
	score += min(movie.VoteAverage/2, 5)

	return score
}

// Replacers are immutable and build a trie on construction, so they are built
// once here rather than per call: ranking a 20-result TMDB search normalizes
// titles dozens of times per scanned file.
var (
	audioLayoutReplacer    = strings.NewReplacer("5.1", " ", "7.1", " ", "2.0", " ")
	titleSeparatorReplacer = strings.NewReplacer(".", " ", "_", " ", "-", " ", "(", " ", ")", " ", "[", " ", "]", " ")
)

// NormalizeTitleForSearch removes filename release noise before a TMDB search.
func NormalizeTitleForSearch(title string) string {
	title = audioLayoutReplacer.Replace(title)
	normalized := titleSeparatorReplacer.Replace(strings.ToLower(strings.TrimSpace(title)))
	tokens := strings.Fields(normalized)
	cleaned := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.Trim(token, ".,!?:;'+\"")
		token = strings.ReplaceAll(token, "'", "")
		token = strings.ReplaceAll(token, "-", "")
		if token == "" {
			continue
		}
		if helpers.IsMovieReleaseNoiseToken(token) {
			continue
		}
		if isBracketedReleaseGroupToken(token) {
			continue
		}
		if isTechnicalToken(token) {
			continue
		}
		cleaned = append(cleaned, token)
	}
	return strings.Join(cleaned, " ")
}

func normalizeComparableMovieTitle(title string) string {
	title = NormalizeTitleForSearch(title)
	var b strings.Builder
	for _, r := range title {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func isBracketedReleaseGroupToken(token string) bool {
	return strings.HasPrefix(token, "yts") || strings.HasPrefix(token, "rarbg")
}

func isTechnicalToken(token string) bool {
	if strings.Contains(token, "aac") || strings.Contains(token, "x26") || strings.Contains(token, "h26") {
		return true
	}

	bitDepth := strings.TrimSuffix(token, "bit")
	if bitDepth != token && len(bitDepth) >= 1 && len(bitDepth) <= 2 {
		for _, digit := range bitDepth {
			if digit < '0' || digit > '9' {
				return false
			}
		}
		return true
	}

	return false
}

func tokenOverlapScore(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}

	aTokens := strings.Fields(a)
	bTokens := strings.Fields(b)
	if len(aTokens) == 0 || len(bTokens) == 0 {
		return 0
	}

	seen := make(map[string]bool)
	for _, token := range aTokens {
		seen[token] = true
	}

	matches := 0
	for _, token := range bTokens {
		if seen[token] {
			matches++
		}
	}

	denominator := max(len(aTokens), len(bTokens))
	return float64(matches) / float64(denominator)
}

func clampTmdbConfidence(score float64) float64 {
	switch {
	case score < 0:
		return 0
	case score > 100:
		return 100
	default:
		return score
	}
}

// summarizeTmdbCandidates renders the top few already-ranked candidates for a
// log line. It takes the ranked slice so a low-confidence match does not pay
// for a second full ranking pass.
func summarizeTmdbCandidates(ranked []*TMDBMovieMatch) string {
	limit := min(len(ranked), 3)
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("%s (%s, %.1f)", ranked[i].Movie.Title, ranked[i].Movie.ReleaseDate, ranked[i].Confidence))
	}

	return strings.Join(parts, "; ")
}

func sequelIndicator(title string) string {
	tokens := strings.Fields(title)
	for _, token := range tokens {
		switch token {
		case "2", "ii", "3", "iii", "4", "iv", "5", "v":
			return token
		}
	}
	return ""
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
