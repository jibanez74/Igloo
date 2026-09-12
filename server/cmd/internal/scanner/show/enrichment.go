package show

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/tmdbmatch"
	"igloo/cmd/internal/tmdb"
)

var errStaleMetadata = errors.New("catalog ownership or TMDB identity changed during enrichment")

// attemptEnrichment records one entity's enrichment attempt and its outcome.
// Entities are shows, seasons, and episodes, so these counts are deliberately
// unrelated to the local file counts. A failure leaves the entity's retry
// marker in place and its previous metadata untouched.
func (s *Scanner) attemptEnrichment(report *scanReport, name string, err error) error {
	report.status.EnrichmentTotal++
	report.status.EnrichmentProcessed++
	if err != nil {
		report.status.EnrichmentFailed++
		report.Issue(name, scanner.PhaseEnrichment, reasonEnrichment)
	} else {
		report.status.Enriched++
	}
	s.publish(report)
	return err
}

func (s *Scanner) enrich(ctx context.Context, report *scanReport) error {
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
			err = s.enrichShow(ctx, report, show)
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

func (s *Scanner) enrichShow(ctx context.Context, report *scanReport, show database.Show) error {
	seasons, err := s.Queries.GetShowSeasons(ctx, show.ID)
	if err != nil {
		return err
	}
	eligible := false
	for _, season := range seasons {
		eligible = eligible || report.scan.seasons[season.ID]
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
		if err == nil {
			err = s.commitMetadata(ctx, show, nil, nil, func(q *database.Queries) error { return applyShow(ctx, q, show.ID, remote) })
		}
		err = s.attemptEnrichment(report, show.DirectoryPath, err)
		if err != nil {
			return err
		}
		show.TmdbID = sql.NullInt64{Int64: int64(remote.ID), Valid: true}
	}
	for _, season := range seasons {
		if !report.scan.seasons[season.ID] {
			continue
		}
		err = s.enrichSeason(ctx, report, show, season)
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

func (s *Scanner) enrichSeason(ctx context.Context, report *scanReport, show database.Show, season database.ShowSeason) error {
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
		if !report.scan.episodes[ep.ID] {
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
		err = s.attemptEnrichment(report, seasonName(show, season), err)
		if err != nil {
			return err
		}
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
		err = s.attemptEnrichment(report, episodeName(show, season, ep), err)
		if err != nil {
			s.Logger.Warn("show episode enrichment pending", "episode_id", ep.ID, "error", err)
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
	return s.tx.Run(ctx, func(q *database.Queries) error {
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
		return apply(q)
	}, nil)
}

func (s *Scanner) lookupShow(ctx context.Context, show database.Show) (*tmdb.TVShow, error) {
	id := int(show.TmdbID.Int64)
	if !show.TmdbID.Valid {
		title, year, full := parseShowTitle(filepath.Base(show.DirectoryPath))
		interpretations := []tmdbmatch.Interpretation{{Title: tmdbmatch.NormalizeTitleForSearch(title), Year: year}}
		if full != "" {
			interpretations = append(interpretations, tmdbmatch.Interpretation{Title: tmdbmatch.NormalizeTitleForSearch(full)})
		}
		var candidates tmdbmatch.Candidates
		for _, query := range interpretations {
			results, err := s.Tmdb.SearchShowsByTitleAndYear(ctx, query.Title, query.Year)
			contextErr := ctx.Err()
			if contextErr != nil {
				return nil, contextErr
			}
			if err != nil {
				continue
			}
			mapped := make([]tmdb.TmdbMovie, 0, len(results))
			for _, r := range results {
				mapped = append(mapped, tmdb.TmdbMovie{TmdbID: r.ID, Title: r.Name, OriginalTitle: r.OriginalName, ReleaseDate: r.FirstAirDate, Popularity: r.Popularity, VoteAverage: r.VoteAverage})
			}
			for _, target := range interpretations {
				candidates.Add(tmdbmatch.Rank(mapped, target.Title, target.Year))
			}
		}
		best := candidates.Best()
		if best == nil {
			return nil, tmdb.ErrNoShowsFound
		}
		id = best.Movie.TmdbID
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

// seasonName and episodeName label enrichment issues with catalog coordinates
// rather than a file path: one entity can span several files.
func seasonName(show database.Show, season database.ShowSeason) string {
	return fmt.Sprintf("%s S%02d", show.LocalName, season.SeasonNumber)
}

func episodeName(show database.Show, season database.ShowSeason, episode database.ShowEpisode) string {
	return fmt.Sprintf("%s S%02dE%02d", show.LocalName, season.SeasonNumber, episode.EpisodeNumber)
}
