package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
	"maps"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ---------------------------------------------------------------------------
// Scan orchestration
// ---------------------------------------------------------------------------

func (app *Application) ScanMoviesLibrary() {
	if !app.Settings.MoviesDir.Valid || app.Settings.MoviesDir.String == "" {
		app.Logger.Info("skipping movie library scan: movies directory is not configured")
		return
	}

	if !movieScanGuard.TryBegin() {
		app.Logger.Warn("movie library scan is already in progress")
		return
	}

	if app.Wait != nil {
		app.Wait.Add(1)
	}
	go app.runMovieScan()
}

func (app *Application) runMovieScan() {
	if app.Wait != nil {
		defer app.Wait.Done()
	}
	defer movieScanGuard.Finish()

	if !app.Settings.MoviesDir.Valid || app.Settings.MoviesDir.String == "" {
		app.Logger.Info("skipping movie library scan: movies directory is not configured")
		return
	}

	app.Logger.Info(fmt.Sprintf("scanning movies directory: %s", app.Settings.MoviesDir.String))

	ctx := app.scanContext()
	errorCount := 0
	moviesScanned := 0
	moviesSkipped := 0
	startTime := time.Now()

	scanIndex, err := app.loadMovieScanIndex(ctx)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("failed to load movie scan index: %s", err.Error()))
		return
	}
	scan := newMovieScanContext(scanIndex)

	batch := make([]helpers.ScanFile, 0, scannerBatchSize)
	flushBatch := func() {
		if len(batch) == 0 {
			return
		}

		scanned, skipped, errors := app.processMoviesBatch(ctx, scan, batch)
		moviesScanned += scanned
		moviesSkipped += skipped
		errorCount += errors
		batch = batch[:0]
	}

	err = helpers.WalkMediaLibraryContext(
		ctx,
		app.Settings.MoviesDir.String,
		helpers.ValidVideoExtensions,
		func(err error) {
			app.Logger.Error(err.Error())
			errorCount++
		},
		func(file helpers.ScanFile) error {
			if scan.movieUnchanged(file.Path, file.Size) {
				moviesSkipped++
				return nil
			}

			batch = append(batch, file)

			if len(batch) >= scannerBatchSize {
				flushBatch()
			}

			return nil
		},
	)

	if err != nil {
		if errors.Is(err, context.Canceled) {
			app.Logger.Info("movie library scan canceled")
			return
		}
		app.Logger.Error(fmt.Sprintf("unexpected error walking movies directory: %s", err.Error()))
		return
	}

	flushBatch()

	app.Logger.Info(fmt.Sprintf("movies scanner completed: %d scanned, %d skipped, %d errors in %s",
		moviesScanned, moviesSkipped, errorCount, helpers.FormatDuration(time.Since(startTime))))
}

func (app *Application) processMoviesBatch(ctx context.Context, scan *movieScanContext, files []helpers.ScanFile) (scanned, skipped, errCount int) {
	for _, file := range files {
		if ctx.Err() != nil {
			return scanned, skipped, errCount
		}

		if scan.movieUnchanged(file.Path, file.Size) {
			skipped++
			continue
		}

		resolved, err := app.resolveMovieFile(ctx, file)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to process %s: %s", file.Path, err.Error()))
			errCount++
			continue
		}

		err = app.persistResolvedMovie(ctx, scan, resolved)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to process %s: %s", file.Path, err.Error()))
			errCount++
			continue
		}

		scanned++
	}

	return scanned, skipped, errCount
}

func (app *Application) loadMovieScanIndex(ctx context.Context) (map[string]int64, error) {
	rows, err := app.Queries.GetMovieScanIndex(ctx)
	if err != nil {
		return nil, err
	}

	return helpers.BuildScanIndex(rows, func(row database.GetMovieScanIndexRow) (string, int64) {
		return row.FilePath, row.Size
	}), nil
}

// ---------------------------------------------------------------------------
// Scan context
// ---------------------------------------------------------------------------

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
	}
}

func (scan *movieScanContext) clone() *movieScanContext {
	return &movieScanContext{
		movieIndex: scan.movieIndex, // shared; never written inside the transaction
		genreIDs:   maps.Clone(scan.genreIDs),
	}
}

func (scan *movieScanContext) mergeFrom(other *movieScanContext) {
	maps.Copy(scan.genreIDs, other.genreIDs)
}

func (scan *movieScanContext) movieUnchanged(path string, size int64) bool {
	return helpers.ScanIndexUnchanged(scan.movieIndex, path, size)
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

func (app *Application) resolveMovieFile(ctx context.Context, file helpers.ScanFile) (*resolvedMovie, error) {
	titleYear, err := helpers.GetTitleAndYearFromFileName(filepath.Base(file.Path))
	if err != nil {
		baseName := filepath.Base(file.Path)
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

	info, err := app.Ffprobe.GetMetadata(file.Path)
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

type tmdbMatchCandidate struct {
	Movie      *tmdb.TmdbMovie
	Score      float64
	Confidence float64
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
	title = normalizeMovieTitleForSearch(title)
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
	scored := rankTmdbMatches(results, targetTitle, targetYear)
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

func (app *Application) persistResolvedMovie(ctx context.Context, scan *movieScanContext, resolved *resolvedMovie) error {
	txScan := scan.clone()

	app.ScannerDBMu.Lock()
	defer app.ScannerDBMu.Unlock()

	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)
	err = app.persistResolvedMovieTx(ctx, qtx, txScan, resolved)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit movie: %w", err)
	}

	// movieIndex is shared (never written inside the transaction) and is only
	// updated here, after a successful commit, so a movie whose transaction
	// failed is never recorded as scanned/unchanged.
	scan.movieIndex[filepath.Clean(resolved.params.FilePath)] = resolved.params.Size
	scan.mergeFrom(txScan)

	return nil
}

func (app *Application) persistResolvedMovieTx(ctx context.Context, qtx *database.Queries, scan *movieScanContext, resolved *resolvedMovie) error {
	movie, err := qtx.UpsertMovie(ctx, resolved.params)
	if err != nil {
		return fmt.Errorf("upsert movie failed: %w", err)
	}

	if resolved.tmdbMovie != nil {
		err = qtx.DeleteMovieCast(ctx, movie.ID)
		if err != nil {
			return fmt.Errorf("delete existing cast failed: %w", err)
		}

		err = qtx.DeleteMovieCrew(ctx, movie.ID)
		if err != nil {
			return fmt.Errorf("delete existing crew failed: %w", err)
		}

		err = processProductionCompanies(ctx, qtx, movie.ID, resolved.tmdbMovie.ProductionCompanies)
		if err != nil {
			return fmt.Errorf("process production companies failed: %w", err)
		}

		err = processCast(ctx, qtx, movie.ID, resolved.tmdbMovie.Credits.Cast)
		if err != nil {
			return fmt.Errorf("process cast failed: %w", err)
		}

		err = processCrew(ctx, qtx, movie.ID, resolved.tmdbMovie.Credits.Crew)
		if err != nil {
			return fmt.Errorf("process crew failed: %w", err)
		}

		err = processMovieGenres(ctx, qtx, scan, movie.ID, resolved.tmdbMovie.Genres)
		if err != nil {
			return fmt.Errorf("process genres failed: %w", err)
		}

		err = processExtraVideos(ctx, qtx, movie.ID, resolved.tmdbMovie.Videos.Results)
		if err != nil {
			return fmt.Errorf("process extra videos failed: %w", err)
		}
	}

	videoStreamCount, err := app.processMovieStreams(ctx, qtx, movie.ID, resolved.streams)
	if err != nil {
		return fmt.Errorf("process movie streams failed: %w", err)
	}
	if videoStreamCount == 0 {
		return fmt.Errorf("no video stream found - invalid movie file")
	}

	err = processChapters(ctx, qtx, movie.ID, resolved.chapters)
	if err != nil {
		return fmt.Errorf("process chapters failed: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// TMDB metadata entities
// ---------------------------------------------------------------------------

func processProductionCompanies(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	companies []struct {
		ID            int    `json:"id"`
		LogoPath      string `json:"logo_path"`
		Name          string `json:"name"`
		OriginCountry string `json:"origin_country"`
	},
) error {
	err := qtx.DeleteMovieProductionCompanies(ctx, movieID)
	if err != nil {
		return fmt.Errorf("delete movie production companies failed: %w", err)
	}

	for _, company := range companies {
		upserted, err := qtx.UpsertProductionCompany(ctx, database.UpsertProductionCompanyParams{
			Name:    company.Name,
			TmdbID:  int64(company.ID),
			Logo:    helpers.NullString(company.LogoPath),
			Country: helpers.NullString(company.OriginCountry),
		})
		if err != nil {
			return fmt.Errorf("upsert production company failed: %w", err)
		}

		err = qtx.CreateMovieProductionCompany(ctx, database.CreateMovieProductionCompanyParams{
			MovieID:             movieID,
			ProductionCompanyID: upserted.ID,
		})
		if err != nil {
			return fmt.Errorf("create movie production company relationship failed: %w", err)
		}
	}

	return nil
}

func processCast(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	cast []struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Character   string `json:"character"`
		ProfilePath string `json:"profile_path"`
		Order       int    `json:"order"`
	},
) error {
	for _, castMember := range cast {
		artist, err := getOrCreateArtist(ctx, qtx, castMember.ID, castMember.Name, castMember.ProfilePath)
		if err != nil {
			return fmt.Errorf("get or create artist failed: %w", err)
		}

		_, err = qtx.UpsertCast(ctx, database.UpsertCastParams{
			MovieID:   movieID,
			ArtistID:  artist.ID,
			Character: castMember.Character,
			CastOrder: int64(castMember.Order),
		})

		if err != nil {
			return fmt.Errorf("upsert cast failed: %w", err)
		}
	}

	return nil
}

func processCrew(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	crew []struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Job         string `json:"job"`
		Department  string `json:"department"`
		ProfilePath string `json:"profile_path"`
	},
) error {
	for _, crewMember := range crew {
		artist, err := getOrCreateArtist(ctx, qtx, crewMember.ID, crewMember.Name, crewMember.ProfilePath)
		if err != nil {
			return fmt.Errorf("get or create artist failed: %w", err)
		}

		_, err = qtx.UpsertCrew(ctx, database.UpsertCrewParams{
			MovieID:    movieID,
			ArtistID:   artist.ID,
			Job:        crewMember.Job,
			Department: crewMember.Department,
		})

		if err != nil {
			return fmt.Errorf("upsert crew failed: %w", err)
		}
	}

	return nil
}

func getOrCreateArtist(
	ctx context.Context,
	qtx *database.Queries,
	tmdbID int,
	name string,
	profilePath string,
) (*database.Artist, error) {
	upserted, err := qtx.UpsertArtist(ctx, database.UpsertArtistParams{
		Name:    name,
		TmdbID:  int64(tmdbID),
		Profile: helpers.NullString(profilePath),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert artist failed: %w", err)
	}

	return &upserted, nil
}

func processMovieGenres(
	ctx context.Context,
	qtx *database.Queries,
	scan *movieScanContext,
	movieID int64,
	genres []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	},
) error {
	err := qtx.DeleteMovieGenres(ctx, movieID)
	if err != nil {
		return fmt.Errorf("delete movie genres failed: %w", err)
	}

	for _, genre := range genres {
		genreID, err := getOrCreateMovieGenreID(ctx, qtx, scan, genre.Name)
		if err != nil {
			return fmt.Errorf("get or create genre failed: %w", err)
		}

		err = qtx.CreateMovieGenre(ctx, database.CreateMovieGenreParams{
			MovieID: movieID,
			GenreID: genreID,
		})

		if err != nil {
			return fmt.Errorf("create movie genre relationship failed: %w", err)
		}
	}

	return nil
}

func getOrCreateMovieGenreID(ctx context.Context, qtx *database.Queries, scan *movieScanContext, tag string) (int64, error) {
	cacheKey := helpers.NormalizedScanCacheKey(tag, "movie")
	if scan != nil {
		if genreID, ok := scan.genreIDs[cacheKey]; ok {
			return genreID, nil
		}
	}

	dbGenre, err := qtx.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
		Tag:       tag,
		GenreType: "movie",
	})
	if err != nil {
		return 0, err
	}

	if scan != nil {
		scan.genreIDs[cacheKey] = dbGenre.ID
	}
	return dbGenre.ID, nil
}

func processExtraVideos(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	results []tmdb.TmdbVideoResult,
) error {
	err := qtx.DeleteMovieExtraVideos(ctx, movieID)
	if err != nil {
		return fmt.Errorf("delete movie extra videos failed: %w", err)
	}

	for _, v := range results {
		if v.Key == "" || v.ID == "" {
			continue
		}

		title := strings.TrimSpace(v.Name)
		if title == "" {
			title = v.Key
		}

		extra, err := qtx.UpsertExtraVideo(ctx, database.UpsertExtraVideoParams{
			Title:      title,
			ExternalID: helpers.NullString(v.ID),
			Key:        v.Key,
			Type:       mapTmdbVideoType(v.Type),
			Site:       mapTmdbVideoSite(v.Site),
			Official:   v.Official,
		})
		if err != nil {
			return fmt.Errorf("upsert extra video failed: %w", err)
		}

		err = qtx.CreateMovieExtraVideo(ctx, database.CreateMovieExtraVideoParams{
			MovieID:      movieID,
			ExtraVideoID: extra.ID,
		})

		if err != nil {
			return fmt.Errorf("create movie extra video link failed: %w", err)
		}
	}

	return nil
}

func mapTmdbVideoType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "trailer", "teaser":
		return "trailer"
	case "featurette", "behind the scenes", "clip", "bloopers", "interview":
		return "special_feature"
	default:
		return "other"
	}
}

func mapTmdbVideoSite(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "youtube":
		return "youtube"
	case "vimeo":
		return "vimeo"
	default:
		return "other"
	}
}

// ---------------------------------------------------------------------------
// Streams and chapters
// ---------------------------------------------------------------------------

func (app *Application) processMovieStreams(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	streams []ffprobe.Stream,
) (videoStreamCount int, err error) {
	app.invalidateSubtitleVTTCache(movieID)

	err = qtx.DeleteMovieVideoStreams(ctx, movieID)
	if err != nil {
		return 0, fmt.Errorf("delete movie video streams failed: %w", err)
	}
	err = qtx.DeleteMovieAudioStreams(ctx, movieID)
	if err != nil {
		return 0, fmt.Errorf("delete movie audio streams failed: %w", err)
	}
	err = qtx.DeleteMovieSubtitles(ctx, movieID)
	if err != nil {
		return 0, fmt.Errorf("delete movie subtitles failed: %w", err)
	}

	for _, stream := range streams {
		switch stream.CodecType {
		case "video":
			if stream.Disposition.AttachedPic == 1 {
				continue
			}
			if helpers.IsCoverArtVideoCodec(stream.CodecName) {
				continue
			}
			err = insertVideoStream(ctx, qtx, movieID, stream)
			if err != nil {
				return 0, err
			}
			videoStreamCount++
		case "audio":
			err = insertAudioStream(ctx, qtx, movieID, stream)
			if err != nil {
				return 0, err
			}
		case "subtitle":
			err = insertSubtitleStream(ctx, qtx, movieID, stream)
			if err != nil {
				return 0, err
			}
		}
	}

	return videoStreamCount, nil
}

func insertVideoStream(ctx context.Context, qtx *database.Queries, movieID int64, stream ffprobe.Stream) error {
	var codecLevel sql.NullInt64
	if stream.Level > 0 {
		codecLevel = sql.NullInt64{Int64: int64(stream.Level), Valid: true}
	}
	var bitDepth sql.NullInt64
	if stream.BitDepth != "" {
		parsed, err := strconv.ParseInt(stream.BitDepth, 10, 64)
		if err == nil {
			bitDepth = sql.NullInt64{Int64: parsed, Valid: true}
		}
	}
	var codedWidth, codedHeight sql.NullInt64
	if stream.CodedWidth > 0 {
		codedWidth = sql.NullInt64{Int64: int64(stream.CodedWidth), Valid: true}
	}
	if stream.CodedHeight > 0 {
		codedHeight = sql.NullInt64{Int64: int64(stream.CodedHeight), Valid: true}
	}

	_, err := qtx.InsertVideoStream(ctx, database.InsertVideoStreamParams{
		MovieID:        movieID,
		StreamIndex:    int64(stream.Index),
		Codec:          stream.CodecName,
		CodecProfile:   helpers.NullString(stream.Profile),
		CodecLevel:     codecLevel,
		BitRate:        helpers.ParseBitRate(stream.BitRate),
		Width:          int64(stream.Width),
		Height:         int64(stream.Height),
		CodedWidth:     codedWidth,
		CodedHeight:    codedHeight,
		AspectRatio:    helpers.NullString(stream.AspectRatio),
		FrameRate:      helpers.ParseFrameRate(stream.FrameRate),
		AvgFrameRate:   helpers.NullString(stream.AvgFrameRate),
		BitDepth:       bitDepth,
		PixelFormat:    helpers.NullString(stream.PixelFormat),
		ColorRange:     helpers.NullString(stream.ColorRange),
		ColorSpace:     helpers.NullString(stream.ColorSpace),
		ColorPrimaries: helpers.NullString(stream.ColorPrimaries),
		ColorTransfer:  helpers.NullString(stream.ColorTransfer),
		Language:       helpers.NullString(stream.Tags.Language),
		Title:          helpers.NullString(stream.Tags.Title),
	})
	if err != nil {
		return fmt.Errorf("insert video stream failed: %w", err)
	}
	return nil
}

func insertAudioStream(ctx context.Context, qtx *database.Queries, movieID int64, stream ffprobe.Stream) error {
	var sampleRate sql.NullInt64
	if stream.SampleRate != "" {
		parsed, err := strconv.ParseInt(stream.SampleRate, 10, 64)
		if err == nil {
			sampleRate = sql.NullInt64{Int64: parsed, Valid: true}
		}
	}

	_, err := qtx.InsertAudioStream(ctx, database.InsertAudioStreamParams{
		MovieID:       movieID,
		StreamIndex:   int64(stream.Index),
		Codec:         stream.CodecName,
		CodecProfile:  helpers.NullString(stream.Profile),
		BitRate:       helpers.ParseBitRate(stream.BitRate),
		SampleRate:    sampleRate,
		Channels:      int64(stream.Channels),
		ChannelLayout: helpers.NullString(stream.ChannelLayout),
		Language:      helpers.NullString(stream.Tags.Language),
		Title:         helpers.NullString(stream.Tags.Title),
	})
	if err != nil {
		return fmt.Errorf("insert audio stream failed: %w", err)
	}
	return nil
}

func insertSubtitleStream(ctx context.Context, qtx *database.Queries, movieID int64, stream ffprobe.Stream) error {
	_, err := qtx.InsertSubtitle(ctx, database.InsertSubtitleParams{
		MovieID:     movieID,
		StreamIndex: int64(stream.Index),
		Codec:       stream.CodecName,
		Language:    helpers.NullString(stream.Tags.Language),
		Title:       helpers.NullString(stream.Tags.Title),
		IsForced:    false,
		IsDefault:   false,
	})
	if err != nil {
		return fmt.Errorf("insert subtitle failed: %w", err)
	}
	return nil
}

func processChapters(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	chapters []ffprobe.Chapter,
) error {
	err := qtx.DeleteMovieChapters(ctx, helpers.NullInt64(movieID))
	if err != nil {
		return fmt.Errorf("delete movie chapters failed: %w", err)
	}

	for _, chapter := range chapters {
		_, err := qtx.InsertChapter(ctx, database.InsertChapterParams{
			MovieID:   helpers.NullInt64(movieID),
			Title:     chapter.Tags.Title,
			StartTime: chapterStartTimeSeconds(chapter),
			Thumb:     sql.NullString{},
		})
		if err != nil {
			return fmt.Errorf("insert chapter failed: %w", err)
		}
	}

	return nil
}
