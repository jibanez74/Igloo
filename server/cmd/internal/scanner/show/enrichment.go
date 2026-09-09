package show

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner/movie"
	"igloo/cmd/internal/tmdb"
)

var errStaleMetadata = errors.New("catalog ownership or TMDB identity changed during enrichment")

func (s *Scanner) enrich(ctx context.Context, state *scanState) error {
	if s.Tmdb == nil {
		return ctx.Err()
	}
	var after int64
	for {
		shows, err := s.Queries.GetPendingShows(ctx, after)
		if err != nil {
			return err
		}
		if len(shows) == 0 {
			return ctx.Err()
		}
		for _, show := range shows {
			after = show.ID
			err = s.enrichShow(ctx, state, show)
			contextErr := ctx.Err()
			if contextErr != nil {
				return contextErr
			}
			if err != nil {
				s.Logger.Warn("show enrichment pending", "show_id", show.ID, "error", err)
			}
		}
	}
}

func (s *Scanner) enrichShow(ctx context.Context, state *scanState, show database.Show) error {
	seasons, err := s.Queries.GetShowSeasons(ctx, show.ID)
	if err != nil {
		return err
	}
	eligible := false
	for _, season := range seasons {
		eligible = eligible || state.seasons[season.ID]
	}
	if !eligible {
		return nil
	}
	pending, err := s.Queries.HasShowRetry(ctx, show.ID)
	if err != nil {
		return err
	}
	if pending || !show.TmdbID.Valid {
		remote, err := s.lookupShow(ctx, show)
		if err != nil {
			return err
		}
		err = s.commitMetadata(ctx, show, nil, nil, func(q *database.Queries) error { return applyShow(ctx, q, show.ID, remote) })
		if err != nil {
			return err
		}
		state.enriched++
		show.TmdbID = sql.NullInt64{Int64: int64(remote.ID), Valid: true}
	}
	for _, season := range seasons {
		if !state.seasons[season.ID] {
			continue
		}
		err = s.enrichSeason(ctx, state, show, season)
		contextErr := ctx.Err()
		if contextErr != nil {
			return contextErr
		}
		if err != nil {
			s.Logger.Warn("show season enrichment pending", "show_id", show.ID, "season", season.SeasonNumber, "error", err)
		}
	}
	return nil
}

func (s *Scanner) enrichSeason(ctx context.Context, state *scanState, show database.Show, season database.ShowSeason) error {
	episodes, err := s.Queries.GetShowEpisodes(ctx, season.ID)
	if err != nil {
		return err
	}
	pending, err := s.Queries.HasShowSeasonRetry(ctx, season.ID)
	if err != nil {
		return err
	}
	pendingEpisodes := make([]database.ShowEpisode, 0)
	for _, ep := range episodes {
		if !state.episodes[ep.ID] {
			continue
		}
		retry, err := s.Queries.HasShowEpisodeRetry(ctx, ep.ID)
		if err != nil {
			return err
		}
		if retry {
			pendingEpisodes = append(pendingEpisodes, ep)
		}
	}
	if !pending && len(pendingEpisodes) == 0 {
		return nil
	}
	remote, err := s.Tmdb.GetSeasonDetails(ctx, int(show.TmdbID.Int64), int(season.SeasonNumber))
	if err != nil {
		return err
	}
	validIdentity := remote != nil && remote.ID > 0 && remote.SeasonNumber == int(season.SeasonNumber)
	if !validIdentity {
		return errors.New("invalid TMDB season identity or numbering")
	}
	changedIdentity := season.TmdbID.Valid && season.TmdbID.Int64 != int64(remote.ID)
	if changedIdentity {
		return errors.New("invalid or changed TMDB season identity")
	}
	byNumber := make(map[int]tmdb.TVEpisode, len(remote.Episodes))
	for _, ep := range remote.Episodes {
		_, duplicate := byNumber[ep.EpisodeNumber]
		if ep.ID <= 0 || ep.EpisodeNumber <= 0 || ep.SeasonNumber != remote.SeasonNumber || duplicate {
			return errors.New("invalid TMDB season episode numbering")
		}
		byNumber[ep.EpisodeNumber] = ep
	}
	if pending {
		err = s.commitMetadata(ctx, show, &season, nil, func(q *database.Queries) error { return applySeason(ctx, q, season.ID, remote) })
		if err != nil {
			return err
		}
		state.enriched++
		season.TmdbID = sql.NullInt64{Int64: int64(remote.ID), Valid: true}
	}
	for _, ep := range pendingEpisodes {
		metadata, found := byNumber[int(ep.EpisodeNumber)]
		if !found {
			s.Logger.Warn("TMDB episode missing", "show_id", show.ID, "season", season.SeasonNumber, "episode", ep.EpisodeNumber)
			continue
		}
		err = s.enrichEpisode(ctx, show, season, ep, metadata)
		contextErr := ctx.Err()
		if contextErr != nil {
			return contextErr
		}
		if err != nil {
			s.Logger.Warn("show episode enrichment pending", "episode_id", ep.ID, "error", err)
		} else {
			state.enriched++
		}
	}
	return nil
}

func (s *Scanner) enrichEpisode(ctx context.Context, show database.Show, season database.ShowSeason, ep database.ShowEpisode, remote tmdb.TVEpisode) error {
	changedIdentity := ep.TmdbID.Valid && ep.TmdbID.Int64 != int64(remote.ID)
	if changedIdentity {
		return errors.New("TMDB episode identity changed")
	}
	credits, err := s.Tmdb.GetEpisodeCredits(ctx, int(show.TmdbID.Int64), int(season.SeasonNumber), int(ep.EpisodeNumber))
	if err != nil {
		return err
	}
	if credits == nil || credits.ID != remote.ID {
		return errors.New("TMDB episode credits identity mismatch")
	}
	return s.commitMetadata(ctx, show, &season, &ep, func(q *database.Queries) error { return applyEpisode(ctx, q, ep.ID, remote, credits) })
}

// Network requests finish before acquiring the shared writer mutex. Recheck
// every owning identity before replacing descriptive fields or clearing retries.
func (s *Scanner) commitMetadata(ctx context.Context, show database.Show, season *database.ShowSeason, episode *database.ShowEpisode, apply func(*database.Queries) error) error {
	s.ScannerDBMu.Lock()
	defer s.ScannerDBMu.Unlock()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.Queries.WithTx(tx)
	current, err := q.GetShow(ctx, show.ID)
	if err != nil {
		return err
	}
	if current.DirectoryPath != show.DirectoryPath || current.TmdbID != show.TmdbID {
		return errStaleMetadata
	}
	if season != nil {
		current, err := q.GetShowSeason(ctx, season.ID)
		if err != nil {
			return err
		}
		if current.ShowID != show.ID || current.SeasonNumber != season.SeasonNumber || current.TmdbID != season.TmdbID {
			return errStaleMetadata
		}
	}
	if episode != nil {
		current, err := q.GetShowEpisode(ctx, episode.ID)
		if err != nil {
			return err
		}
		if current.SeasonID != season.ID || current.EpisodeNumber != episode.EpisodeNumber || current.TmdbID != episode.TmdbID {
			return errStaleMetadata
		}
	}
	err = apply(q)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Scanner) lookupShow(ctx context.Context, show database.Show) (*tmdb.TVShow, error) {
	id := int(show.TmdbID.Int64)
	if !show.TmdbID.Valid {
		title, year, full := parseShowTitle(filepath.Base(show.DirectoryPath))
		interpretations := []struct {
			title string
			year  int
		}{{movie.NormalizeTitleForSearch(title), year}}
		if full != "" {
			interpretations = append(interpretations, struct {
				title string
				year  int
			}{movie.NormalizeTitleForSearch(full), 0})
		}
		candidates := map[int]*movie.TMDBMovieMatch{}
		for _, query := range interpretations {
			results, err := s.Tmdb.SearchShowsByTitleAndYear(ctx, query.title, query.year)
			contextErr := ctx.Err()
			if contextErr != nil {
				return nil, contextErr
			}
			if err != nil {
				continue
			}
			mapped := make([]tmdb.TmdbMovie, 0, len(results))
			for _, r := range results {
				if r.ID > 0 {
					mapped = append(mapped, tmdb.TmdbMovie{TmdbID: r.ID, Title: r.Name, OriginalTitle: r.OriginalName, ReleaseDate: r.FirstAirDate, Popularity: r.Popularity, VoteAverage: r.VoteAverage})
				}
			}
			for _, target := range interpretations {
				for _, match := range movie.RankTMDBMovies(mapped, target.title, target.year) {
					previous, exists := candidates[match.Movie.TmdbID]
					betterMatch := !exists || compareMatches(match, previous) < 0
					if betterMatch {
						candidates[match.Movie.TmdbID] = match
					}
				}
			}
		}
		ranked := make([]*movie.TMDBMovieMatch, 0, len(candidates))
		for _, candidate := range candidates {
			ranked = append(ranked, candidate)
		}
		slices.SortFunc(ranked, compareMatches)
		if len(ranked) == 0 {
			return nil, tmdb.ErrNoShowsFound
		}
		id = ranked[0].Movie.TmdbID
	}
	result, err := s.Tmdb.GetShowDetails(ctx, id)
	if err != nil {
		return nil, err
	}
	validIdentity := result != nil && result.ID == id && strings.TrimSpace(result.Name) != ""
	if !validIdentity {
		return nil, fmt.Errorf("invalid TMDB show identity %d", id)
	}
	return result, nil
}
func compareMatches(a, b *movie.TMDBMovieMatch) int {
	if a.Score > b.Score {
		return -1
	}
	if a.Score < b.Score {
		return 1
	}
	if a.Movie.Title != b.Movie.Title {
		return strings.Compare(a.Movie.Title, b.Movie.Title)
	}
	if a.Movie.TmdbID < b.Movie.TmdbID {
		return -1
	}
	if a.Movie.TmdbID > b.Movie.TmdbID {
		return 1
	}
	return 0
}
