package movie

import (
	"context"
	"fmt"
	"maps"
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
	// per-movie transaction (getOrCreateMovieGenreID), so clone/mergeFrom isolate
	// it until commit to avoid caching ids from a rolled-back transaction.
	genreIDs map[string]int64
	// artistIDs memoizes TMDB person id -> artist.id within a scan, so a person
	// appearing in many movies (or in several crew roles of one movie) is
	// upserted once per scan instead of once per credit. Same clone/mergeFrom
	// rollback isolation as genreIDs. Artist rows are never deleted, so cached
	// ids stay valid for the whole scan.
	artistIDs map[int64]int64
}

func newMovieScanContext(movieIndex map[string]int64) *movieScanContext {
	if movieIndex == nil {
		movieIndex = make(map[string]int64)
	}

	// Take ownership of movieIndex: loadMovieScanIndex already cleaned its keys
	// and the caller discards its reference, so no defensive copy is needed.
	return &movieScanContext{
		movieIndex: movieIndex,
		genreIDs:   make(map[string]int64),
		artistIDs:  make(map[int64]int64),
	}
}

func (scan *movieScanContext) clone() *movieScanContext {
	return &movieScanContext{
		movieIndex: scan.movieIndex, // shared; never written inside the transaction
		genreIDs:   maps.Clone(scan.genreIDs),
		artistIDs:  maps.Clone(scan.artistIDs),
	}
}

func (scan *movieScanContext) mergeFrom(other *movieScanContext) {
	maps.Copy(scan.genreIDs, other.genreIDs)
	maps.Copy(scan.artistIDs, other.artistIDs)
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

	var tmdbMovie *tmdb.TmdbMovie

	if s.tmdb != nil {
		var searchResults []tmdb.TmdbMovie
		var searchErr error
		if titleYear.Year > 0 {
			searchResults, searchErr = s.tmdb.SearchMoviesByTitleAndYear(ctx, searchTitle, titleYear.Year)
		} else {
			searchResults, searchErr = s.tmdb.SearchMoviesByTitleAndYear(ctx, searchTitle)
		}
		if searchErr == nil && len(searchResults) > 0 {
			bestMatch := selectBestTmdbMatch(searchResults, searchTitle, titleYear.Year)
			if bestMatch != nil {
				err = s.tmdb.GetTmdbMovieByID(ctx, bestMatch.Movie)
				if err == nil {
					tmdbMovie = bestMatch.Movie
					if bestMatch.Confidence < 70 {
						s.logger.Warn(
							"low-confidence TMDB movie match",
							"path", file.Path,
							"parsed_title", searchTitle,
							"parsed_year", titleYear.Year,
							"tmdb_id", bestMatch.Movie.TmdbID,
							"tmdb_title", bestMatch.Movie.Title,
							"tmdb_release_date", bestMatch.Movie.ReleaseDate,
							"confidence", fmt.Sprintf("%.1f", bestMatch.Confidence),
							"alternatives", summarizeTmdbCandidates(searchResults, searchTitle, titleYear.Year),
						)
					}
				}
			}
		}
	}

	info, err := s.ffprobe.GetMetadata(file.Path)
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

func selectBestTmdbMatch(results []tmdb.TmdbMovie, targetTitle string, targetYear int) *TMDBMovieMatch {
	scoredMatches := RankTMDBMovies(results, targetTitle, targetYear)
	if len(scoredMatches) == 0 {
		return nil
	}
	return scoredMatches[0]
}

// RankTMDBMovies orders TMDB results by the scanner's title and year score.
func RankTMDBMovies(results []tmdb.TmdbMovie, targetTitle string, targetYear int) []*TMDBMovieMatch {
	if len(results) == 0 {
		return nil
	}

	scoredMatches := make([]*TMDBMovieMatch, 0, len(results))
	for i := range results {
		movie := &results[i]
		score := scoreTmdbCandidate(targetTitle, targetYear, movie)
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

func scoreTmdbCandidate(targetTitle string, targetYear int, movie *tmdb.TmdbMovie) float64 {
	normalizedTarget := normalizeComparableMovieTitle(targetTitle)
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

	if sequelIndicator(normalizedTarget) == sequelIndicator(normalizedTitle) {
		score += 8
	} else if sequelIndicator(normalizedTarget) != "" {
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

// NormalizeTitleForSearch removes filename release noise before a TMDB search.
func NormalizeTitleForSearch(title string) string {
	title = strings.NewReplacer("5.1", " ", "7.1", " ", "2.0", " ").Replace(title)
	replacer := strings.NewReplacer(".", " ", "_", " ", "-", " ", "(", " ", ")", " ", "[", " ", "]", " ")
	normalized := replacer.Replace(strings.ToLower(strings.TrimSpace(title)))
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
	if strings.HasSuffix(token, "bit") {
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

func summarizeTmdbCandidates(results []tmdb.TmdbMovie, targetTitle string, targetYear int) string {
	scored := RankTMDBMovies(results, targetTitle, targetYear)
	limit := min(len(scored), 3)
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("%s (%s, %.1f)", scored[i].Movie.Title, scored[i].Movie.ReleaseDate, scored[i].Confidence))
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

// ---------------------------------------------------------------------------
// Persistence
// ---------------------------------------------------------------------------
