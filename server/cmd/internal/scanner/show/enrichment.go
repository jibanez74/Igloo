package show

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/tmdbmatch"
	"igloo/cmd/internal/tmdb"
)

var errStaleMetadata = errors.New("catalog ownership or TMDB identity changed during enrichment")

// attemptEnrichment records one entity's enrichment outcome. Entities are
// shows, seasons, and episodes, so these counts are deliberately unrelated to
// the local file counts. A failure leaves the entity's retry marker in place
// and its previous metadata untouched.
func (s *Scanner) attemptEnrichment(report *scanReport, name string, err error) error {
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

// providerBreaker stops enrichment dispatch after an authentication failure
// or a run of transient provider failures; pending entities retry on a later
// scan. Only errors returned by the TMDB client are observed.
type providerBreaker struct {
	consecutive int
	stopped     bool
}

func (b *providerBreaker) observe(err error) {
	authentication, transient := tmdb.ProviderFailure(err)
	if transient {
		b.consecutive++
	} else {
		b.consecutive = 0
	}
	if authentication || b.consecutive >= scanner.MaxConsecutiveProviderFailures {
		b.stopped = true
	}
}

// pendingShow is one show with enrichment work this run may do: a lookup when
// it has no identity or a retry marker, plus the pending seasons and episodes
// among those the local phase touched.
type pendingShow struct {
	show           database.Show
	lookup         bool
	seasons        []database.ShowSeason
	seasonPending  map[int64]bool
	episodePending map[int64]bool
	entities       int
}

// collectPendingShows resolves the run's enrichment work before any request
// is made, so the total is published up front. Shows whose miss backoff has
// not elapsed are skipped silently.
func (s *Scanner) collectPendingShows(ctx context.Context, scan *showScanContext, now time.Time) ([]pendingShow, int, error) {
	result := make([]pendingShow, 0)
	total := 0
	var after int64
	for {
		shows, err := s.Queries.GetPendingShows(ctx, after)
		if err != nil {
			return nil, 0, err
		}
		if len(shows) == 0 {
			return result, total, ctx.Err()
		}
		for _, show := range shows {
			after = show.ID
			seasons, err := s.Queries.GetShowSeasons(ctx, show.ID)
			if err != nil {
				return nil, 0, err
			}
			touched := make([]database.ShowSeason, 0, len(seasons))
			for _, season := range seasons {
				if scan.seasons[season.ID] {
					touched = append(touched, season)
				}
			}
			if len(touched) == 0 {
				continue
			}
			retry, err := s.Queries.GetShowRetry(ctx, show.ID)
			pending := err == nil
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, 0, err
			}
			lookup := pending || !show.TmdbID.Valid
			backedOff := lookup && !scanner.MissBackoffElapsed(retry.Attempts, retry.LastAttemptAt, now)
			if backedOff {
				continue
			}
			item := pendingShow{show: show, lookup: lookup, seasons: touched, seasonPending: make(map[int64]bool), episodePending: make(map[int64]bool)}
			seasonIDs, err := s.Queries.GetShowPendingSeasonIDs(ctx, show.ID)
			if err != nil {
				return nil, 0, err
			}
			for _, id := range seasonIDs {
				if scan.seasons[id] {
					item.seasonPending[id] = true
				}
			}
			episodeIDs, err := s.Queries.GetShowPendingEpisodeIDs(ctx, show.ID)
			if err != nil {
				return nil, 0, err
			}
			for _, id := range episodeIDs {
				if scan.episodes[id] {
					item.episodePending[id] = true
				}
			}
			item.entities = len(item.seasonPending) + len(item.episodePending)
			if lookup {
				item.entities++
			}
			if item.entities == 0 {
				continue
			}
			result = append(result, item)
			total += item.entities
		}
	}
}

func (s *Scanner) enrich(ctx context.Context, report *scanReport) error {
	if s.Tmdb == nil {
		return ctx.Err()
	}
	pending, total, err := s.collectPendingShows(ctx, report.scan, s.Now())
	if err != nil {
		return err
	}
	report.status.EnrichmentTotal = total
	s.publish(report)
	breaker := &providerBreaker{}
	for _, item := range pending {
		if breaker.stopped {
			break
		}
		err = s.enrichShow(ctx, report, breaker, item)
		contextErr := ctx.Err()
		if contextErr != nil {
			return contextErr
		}
		if err != nil {
			s.Logger.Warn("show enrichment pending", "show_id", item.show.ID, "error", err)
		}
	}
	if breaker.stopped {
		report.Issue("", scanner.PhaseEnrichment, reasonStopped)
		s.publish(report)
	}
	return ctx.Err()
}

// enrichShow resolves the show first; its seasons and episodes are attempted
// only once it has an identity. A show that cannot be resolved this run drops
// its descendants from the total rather than counting them as failures.
func (s *Scanner) enrichShow(ctx context.Context, report *scanReport, breaker *providerBreaker, item pendingShow) error {
	show := item.show
	if item.lookup {
		remote, err := s.lookupShow(ctx, breaker, show)
		unmatched := err == nil && remote == nil
		if unmatched {
			report.status.EnrichmentTotal -= item.entities - 1
			err = s.commitMetadata(ctx, show, nil, nil, func(q *database.Queries) error {
				return q.RecordShowTmdbMiss(ctx, database.RecordShowTmdbMissParams{ShowID: show.ID, LastAttemptAt: helpers.NullInt64(s.Now().Unix())})
			})
			if err != nil {
				return s.attemptEnrichment(report, show.DirectoryPath, err)
			}
			report.status.EnrichmentProcessed++
			report.status.EnrichmentUnmatched++
			report.Issue(show.DirectoryPath, scanner.PhaseEnrichment, reasonUnmatched)
			s.publish(report)
			return nil
		}
		if err == nil {
			err = s.commitMetadata(ctx, show, nil, nil, func(q *database.Queries) error { return applyShow(ctx, q, show.ID, remote) })
		}
		err = s.attemptEnrichment(report, show.DirectoryPath, err)
		if err != nil {
			report.status.EnrichmentTotal -= item.entities - 1
			return err
		}
		show.TmdbID = sql.NullInt64{Int64: int64(remote.ID), Valid: true}
	}
	for _, season := range item.seasons {
		if breaker.stopped {
			return nil
		}
		err := s.enrichSeason(ctx, report, breaker, show, season, item)
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

func (s *Scanner) enrichSeason(ctx context.Context, report *scanReport, breaker *providerBreaker, show database.Show, season database.ShowSeason, item pendingShow) error {
	pending := item.seasonPending[season.ID]
	episodes, err := s.Queries.GetShowEpisodes(ctx, season.ID)
	if err != nil {
		return err
	}
	pendingEpisodes := make([]database.ShowEpisode, 0)
	for _, ep := range episodes {
		if item.episodePending[ep.ID] {
			pendingEpisodes = append(pendingEpisodes, ep)
		}
	}
	if !pending && len(pendingEpisodes) == 0 {
		return nil
	}
	entities := len(pendingEpisodes)
	if pending {
		entities++
	}
	lookupCtx, cancel := context.WithTimeout(ctx, scanner.TmdbLookupTimeout)
	remote, err := s.Tmdb.GetSeasonDetails(lookupCtx, int(show.TmdbID.Int64), int(season.SeasonNumber))
	cancel()
	contextErr := ctx.Err()
	if contextErr != nil {
		return contextErr
	}
	breaker.observe(err)
	if err == nil {
		validIdentity := remote != nil && remote.ID > 0 && remote.SeasonNumber == int(season.SeasonNumber)
		if !validIdentity {
			err = errors.New("invalid TMDB season identity or numbering")
		}
	}
	if err == nil {
		changedIdentity := season.TmdbID.Valid && season.TmdbID.Int64 != int64(remote.ID)
		if changedIdentity {
			err = errors.New("invalid or changed TMDB season identity")
		}
	}
	byNumber := make(map[int]tmdb.TVEpisode)
	if err == nil {
		for _, ep := range remote.Episodes {
			_, duplicate := byNumber[ep.EpisodeNumber]
			if ep.ID <= 0 || ep.EpisodeNumber <= 0 || ep.SeasonNumber != remote.SeasonNumber || duplicate {
				err = errors.New("invalid TMDB season episode numbering")
				break
			}
			byNumber[ep.EpisodeNumber] = ep
		}
	}
	if err != nil {
		// One failed response fails every entity that depended on it.
		report.status.EnrichmentProcessed += entities
		report.status.EnrichmentFailed += entities
		report.Issue(seasonName(show, season), scanner.PhaseEnrichment, reasonEnrichment)
		s.publish(report)
		return err
	}
	if pending {
		err = s.commitMetadata(ctx, show, &season, nil, func(q *database.Queries) error { return applySeason(ctx, q, season.ID, remote) })
		err = s.attemptEnrichment(report, seasonName(show, season), err)
		if err != nil {
			report.status.EnrichmentTotal -= len(pendingEpisodes)
			return err
		}
		season.TmdbID = sql.NullInt64{Int64: int64(remote.ID), Valid: true}
	}
	for _, ep := range pendingEpisodes {
		metadata, found := byNumber[int(ep.EpisodeNumber)]
		if !found {
			// TMDB does not list this episode yet; it stays pending without an
			// outcome this run.
			report.status.EnrichmentTotal--
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
	return s.commitMetadata(ctx, show, &season, &ep, func(q *database.Queries) error { return applyEpisode(ctx, q, ep.ID, remote) })
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

// lookupShow resolves a show's TMDB identity within one lookup budget. It
// returns (nil, nil) for a definitive no-match and the provider error when a
// search or the details fetch failed.
func (s *Scanner) lookupShow(ctx context.Context, breaker *providerBreaker, show database.Show) (*tmdb.TVShow, error) {
	lookupCtx, cancel := context.WithTimeout(ctx, scanner.TmdbLookupTimeout)
	defer cancel()
	id := int(show.TmdbID.Int64)
	if !show.TmdbID.Valid {
		title, year, full := parseShowTitle(filepath.Base(show.DirectoryPath))
		interpretations := []tmdbmatch.Interpretation{{Title: tmdbmatch.NormalizeTitleForSearch(title), Year: year}}
		if full != "" {
			interpretations = append(interpretations, tmdbmatch.Interpretation{Title: tmdbmatch.NormalizeTitleForSearch(full)})
		}
		var candidates tmdbmatch.Candidates
		var searchErr error
		for _, query := range interpretations {
			results, err := s.Tmdb.SearchShowsByTitleAndYear(lookupCtx, query.Title, query.Year)
			contextErr := ctx.Err()
			if contextErr != nil {
				return nil, contextErr
			}
			noMatch := errors.Is(err, tmdb.ErrNoShowsFound)
			if noMatch {
				// A definitive answer from the provider is a healthy response.
				breaker.observe(nil)
				continue
			}
			breaker.observe(err)
			if err != nil {
				authentication, _ := tmdb.ProviderFailure(err)
				if authentication {
					return nil, err
				}
				s.Logger.Warn("TMDB show search failed", "show", show.DirectoryPath, "error", err)
				searchErr = err
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
			return nil, searchErr
		}
		id = best.Movie.TmdbID
	}
	result, err := s.Tmdb.GetShowDetails(lookupCtx, id)
	contextErr := ctx.Err()
	if contextErr != nil {
		return nil, contextErr
	}
	breaker.observe(err)
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
