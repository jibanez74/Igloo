package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
	"math"
	"mime"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

type tmdbMatchCandidate struct {
	Movie      *tmdb.TmdbMovie
	Score      float64
	Confidence float64
}

type resolvedMovie struct {
	params    database.UpsertMovieParams
	tmdbMovie *tmdb.TmdbMovie
	streams   []ffprobe.Stream
	chapters  []ffprobe.Chapter
	fileSize  int64
}

var movieReleaseNoiseTokens = map[string]bool{
	"1080p": true, "720p": true, "480p": true, "2160p": true, "4k": true,
	"bluray": true, "brrip": true, "webrip": true, "web": true, "webdl": true, "web-dl": true,
	"dvdrip": true, "hdrip": true, "remux": true, "repack": true, "proper": true, "remastered": true,
	"x264": true, "x265": true, "h264": true, "h265": true, "hevc": true, "av1": true,
	"10bit": true, "8bit": true, "hdr": true, "sdr": true,
	"aac": true, "aac5": true, "aac51": true, "ddp": true, "ac3": true, "dts": true, "dtshd": true,
	"atmos": true, "truehd": true,
	"yts": true, "ytsmx": true, "mx": true,
}

func (app *Application) processMovieFile(ctx context.Context, qtx *database.Queries, path, ext string, fileSize int64) error {
	resolved, err := app.resolveMovieFile(ctx, movieFile{path: path, ext: ext, size: fileSize})
	if err != nil {
		return err
	}

	_, err = app.persistResolvedMovieTx(ctx, qtx, newMovieScanContext(nil), resolved)
	return err
}

func (app *Application) resolveMovieFile(ctx context.Context, file movieFile) (*resolvedMovie, error) {
	titleYear, err := helpers.GetTitleAndYearFromFileName(filepath.Base(file.path))
	if err != nil {
		baseName := filepath.Base(file.path)
		ext := filepath.Ext(baseName)
		titleYear = &helpers.TitleYearResponse{
			Title: strings.TrimSuffix(baseName, ext),
			Year:  0,
		}
	}

	searchTitle := normalizeMovieTitleForSearch(titleYear.Title)
	if searchTitle == "" {
		searchTitle = titleYear.Title
	}

	var tmdbMovie *tmdb.TmdbMovie

	if app.Tmdb != nil {
		var searchResults []tmdb.TmdbMovie
		var searchErr error
		if titleYear.Year > 0 {
			searchResults, searchErr = app.Tmdb.SearchMoviesByTitleAndYear(ctx, searchTitle, titleYear.Year)
		} else {
			searchResults, searchErr = app.Tmdb.SearchMoviesByTitleAndYear(ctx, searchTitle)
		}
		if searchErr == nil && len(searchResults) > 0 {
			bestMatch := selectBestTmdbMatch(searchResults, searchTitle, titleYear.Year)
			if bestMatch != nil {
				err = app.Tmdb.GetTmdbMovieByID(ctx, bestMatch.Movie)
				if err == nil {
					tmdbMovie = bestMatch.Movie
					if bestMatch.Confidence < 70 {
						app.Logger.Warn(
							"low-confidence TMDB movie match",
							"path", file.path,
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

	info, err := app.Ffprobe.GetMetadata(file.path)
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed (required): %w", err)
	}

	mimeType := mime.TypeByExtension("." + file.ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	params := database.UpsertMovieParams{
		Title:     titleYear.Title,
		FilePath:  file.path,
		FileName:  filepath.Base(file.path),
		Container: file.ext,
		MimeType:  mimeType,
		Adult:     false,
	}

	params.Size = file.size
	if info.Format.Size != "" {
		size, err := strconv.ParseInt(info.Format.Size, 10, 64)
		if err == nil && size > 0 {
			params.Size = size
		}
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
			if year := extractYearFromReleaseDate(tmdbMovie.ReleaseDate); year > 0 {
				params.Year = helpers.NullInt64(int64(year))
			}
		}
	} else {
		if titleYear.Year > 0 {
			params.Year = helpers.NullInt64(int64(titleYear.Year))
		}
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
		fileSize:  params.Size,
	}, nil
}

func (app *Application) persistResolvedMovieTx(ctx context.Context, qtx *database.Queries, scan *movieScanContext, resolved *resolvedMovie) (int64, error) {
	params := resolved.params

	movie, err := qtx.UpsertMovie(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("upsert movie failed: %w", err)
	}

	if resolved.tmdbMovie != nil {
		err = qtx.DeleteMovieCast(ctx, movie.ID)
		if err != nil {
			return 0, fmt.Errorf("delete existing cast failed: %w", err)
		}

		err = qtx.DeleteMovieCrew(ctx, movie.ID)
		if err != nil {
			return 0, fmt.Errorf("delete existing crew failed: %w", err)
		}

		if err := app.processProductionCompanies(ctx, qtx, scan, movie.ID, resolved.tmdbMovie.ProductionCompanies); err != nil {
			return 0, fmt.Errorf("process production companies failed: %w", err)
		}

		if err := app.processCast(ctx, qtx, scan, movie.ID, resolved.tmdbMovie.Credits.Cast); err != nil {
			return 0, fmt.Errorf("process cast failed: %w", err)
		}

		if err := app.processCrew(ctx, qtx, scan, movie.ID, resolved.tmdbMovie.Credits.Crew); err != nil {
			return 0, fmt.Errorf("process crew failed: %w", err)
		}

		if err := app.processMovieGenres(ctx, qtx, scan, movie.ID, resolved.tmdbMovie.Genres); err != nil {
			return 0, fmt.Errorf("process genres failed: %w", err)
		}

		if err := app.processExtraVideos(ctx, qtx, scan, movie.ID, resolved.tmdbMovie.Videos.Results); err != nil {
			return 0, fmt.Errorf("process extra videos failed: %w", err)
		}
	}

	videoStreamCount, err := app.processMovieStreams(ctx, qtx, movie.ID, resolved.streams)
	if err != nil {
		return 0, fmt.Errorf("process movie streams failed: %w", err)
	}
	if videoStreamCount == 0 {
		return 0, fmt.Errorf("no video stream found - invalid movie file")
	}

	if err := app.processChapters(ctx, qtx, movie.ID, resolved.chapters); err != nil {
		return 0, fmt.Errorf("process chapters failed: %w", err)
	}

	return movie.ID, nil
}

func selectBestTmdbMatch(results []tmdb.TmdbMovie, targetTitle string, targetYear int) *tmdbMatchCandidate {
	scoredMatches := rankTmdbMatches(results, targetTitle, targetYear)
	if len(scoredMatches) == 0 {
		return nil
	}
	return scoredMatches[0]
}

func rankTmdbMatches(results []tmdb.TmdbMovie, targetTitle string, targetYear int) []*tmdbMatchCandidate {
	if len(results) == 0 {
		return nil
	}

	scoredMatches := make([]*tmdbMatchCandidate, 0, len(results))
	for i := range results {
		movie := &results[i]
		score := scoreTmdbCandidate(targetTitle, targetYear, movie)
		scoredMatches = append(scoredMatches, &tmdbMatchCandidate{
			Movie:      movie,
			Score:      score,
			Confidence: clampTmdbConfidence(score),
		})
	}

	slices.SortFunc(scoredMatches, func(a, b *tmdbMatchCandidate) int {
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

func normalizeMovieTitleForSearch(title string) string {
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
		if movieReleaseNoiseTokens[token] {
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
			score += helpers.TMDB_YEAR_MATCH_SCORE
		case movieYear > 0 && absInt(movieYear-targetYear) == 1:
			score += 12
		case movieYear > 0:
			score -= 15
		}
	}

	score += minFloat(movie.Popularity/25, 8)
	score += minFloat(movie.VoteAverage/2, 5)

	return score
}

func normalizeComparableMovieTitle(title string) string {
	title = normalizeMovieTitleForSearch(title)
	var b strings.Builder
	for _, r := range title {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
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

	denominator := maxInt(len(aTokens), len(bTokens))
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
	scored := rankTmdbMatches(results, targetTitle, targetYear)
	limit := minInt(len(scored), 3)
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

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
