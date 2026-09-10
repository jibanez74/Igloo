package movie

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/tmdb"
)

type enrichmentJob struct {
	file     scanner.ScanFile
	baseline movieScanEntry
}

type enrichmentResult struct {
	job         enrichmentJob
	resolved    *resolvedMovie
	inspection  *scanner.FileInspection
	err         error
	providerErr error
}

func (s *Scanner) prepareEnrichment(ctx context.Context, job enrichmentJob) enrichmentResult {
	started := time.Now()
	defer func() {
		elapsed := time.Since(started)
		if elapsed >= 5*time.Second {
			s.logger.Info("slow movie enrichment", "path", job.file.Path, "elapsed", elapsed)
		}
	}()
	result := enrichmentResult{job: job}
	result.inspection, result.err = scanner.InspectFileMetadata(ctx, job.file.Path, nil, s.now)
	if result.err != nil {
		return result
	}
	if result.inspection.Outcome == scanner.FileDeferred || result.inspection.Fingerprint != job.baseline.FileFingerprint {
		result.err = &scanner.FileDeferral{Reason: scanner.FileChanged}
		return result
	}
	observed, err := s.queries.GetMovieByPath(ctx, job.file.Path)
	if err != nil {
		result.err = err
		return result
	}
	if observed.ID != job.baseline.ID || observed.TmdbID != job.baseline.TmdbID {
		result.err = errors.New("catalog identity changed before enrichment")
		return result
	}
	titleYear := movieTitleYear(job.file.Path)
	searchTitle := NormalizeTitleForSearch(titleYear.Title)
	if searchTitle == "" {
		searchTitle = titleYear.Title
	}
	details, err := s.lookupTmdbMovie(ctx, job.file.Path, searchTitle, titleYear.Year, observed.TmdbID)
	result.err, result.providerErr = err, err
	if err == nil {
		result.resolved = &resolvedMovie{
			observed: observed, metadataOnly: true, inspection: result.inspection,
			baseline: job.baseline.FileFingerprint, pending: job.baseline.PendingRetry,
			params: database.UpsertMovieParams{FilePath: job.file.Path}, tmdbMovie: details,
		}
	}

	return result
}

func (s *Scanner) enrichMovies(ctx context.Context, scan *movieScanContext, report *scanReport, files []localFile) {
	candidates := make([]enrichmentJob, 0)
	for _, file := range files {
		if file.state != "imported" && file.state != "unchanged" {
			continue
		}
		entry := scan.movieIndex[file.file.Path]
		if entry.ID != 0 && (entry.PendingRetry || !entry.TmdbID.Valid) && !scan.attempted[file.file.Path] {
			candidates = append(candidates, enrichmentJob{file: file.file, baseline: entry})
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].file.Path < candidates[j].file.Path })
	report.status.EnrichmentTotal = len(candidates)
	report.status.PendingEnrichment = 0
	for _, entry := range scan.movieIndex {
		if entry.PendingRetry || !entry.TmdbID.Valid {
			report.status.PendingEnrichment++
		}
	}
	s.publish(report)
	if s.tmdb == nil {
		return
	}
	jobs := make(chan enrichmentJob)
	results := make(chan enrichmentResult, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range jobs {
				results <- s.prepareEnrichment(ctx, job)
			}
		}()
	}
	defer workers.Wait()
	next, active, consecutive := 0, 0, 0
	stopped := false
	for next < len(candidates) || active > 0 {
		var dispatch chan enrichmentJob
		var job enrichmentJob
		contextErr := ctx.Err()
		canDispatch := next < len(candidates) && active < 2 && !stopped && contextErr == nil
		if canDispatch {
			job = candidates[next]
			dispatch = jobs
		}
		if active == 0 && dispatch == nil {
			break
		}
		select {
		case dispatch <- job:
			scan.attempted[job.file.Path] = true
			report.active[job.file.Path] = true
			next++
			active++
			s.publish(report)
		case result := <-results:
			active--
			delete(report.active, result.job.file.Path)
			contextErr := ctx.Err()
			if contextErr == nil {
				report.status.EnrichmentProcessed++
				authentication, transient := tmdb.ProviderFailure(result.providerErr)
				if transient {
					consecutive++
				} else {
					consecutive = 0
				}
				if authentication || consecutive >= 3 {
					stopped = true
					report.issue("", "enrichment", "TMDB enrichment stopped after provider failures. Pending movies will retry on a later scan.")
				}
				before := scan.enriched
				if result.err == nil {
					result.err = s.persistResolvedMovie(ctx, scan, result.resolved)
				}
				contextErr := ctx.Err()
				if scan.enriched > before || contextErr == nil {
					if result.err != nil {
						report.status.EnrichmentFailed++
						report.issue(result.job.file.Path, "enrichment", "Descriptions could not be updated. Local movie data remains available; enrichment will retry later.")
						s.logger.Warn("movie enrichment failed", "path", result.job.file.Path, "error", result.err)
					} else if scan.enriched > before {
						report.status.Enriched++
						report.status.PendingEnrichment--
					} else if result.resolved.applied && result.resolved.tmdbMovie == nil {
						report.status.EnrichmentUnmatched++
						report.issue(result.job.file.Path, "enrichment", "TMDB returned no matching movie. You can use Identify or retry on a later scan.")
					}
				}
			}
			if result.inspection != nil {
				err := result.inspection.Close()
				if err != nil {
					s.logger.Warn("close enrichment inspection", "filename", filepath.Base(result.job.file.Path), "error", err)
				}
			}
			s.publish(report)
		}
	}
	close(jobs)
}
