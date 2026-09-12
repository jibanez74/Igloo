package movie

import (
	"context"
	"errors"
	"sort"
	"time"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/tmdbmatch"
	"igloo/cmd/internal/tmdb"
)

type enrichmentJob struct {
	file     scanner.ScanFile
	baseline movieScanEntry
}

type enrichmentResult struct {
	job         enrichmentJob
	resolved    *enrichedMovie
	inspection  *scanner.FileInspection
	err         error
	providerErr error
}

// prepareEnrichment runs on a worker and never touches the database; the
// persisting transaction re-checks the catalog identity it was given.
func (s *Scanner) prepareEnrichment(ctx context.Context, job enrichmentJob) enrichmentResult {
	defer s.logSlow("slow movie enrichment", time.Now(), "path", job.file.Path)
	result := enrichmentResult{job: job}
	result.inspection, result.err = scanner.InspectFileMetadata(ctx, job.file.Path, nil, s.now)
	if result.err != nil {
		return result
	}
	changed := result.inspection.Outcome == scanner.FileDeferred || result.inspection.Fingerprint != job.baseline.FileFingerprint
	if changed {
		result.err = &scanner.FileDeferral{Reason: scanner.FileChanged}
		return result
	}
	titleYear := movieTitleYear(job.file.Path)
	searchTitle := tmdbmatch.NormalizeTitleForSearch(titleYear.Title)
	if searchTitle == "" {
		searchTitle = titleYear.Title
	}
	// A file whose name is nothing but release noise has nothing to search for;
	// it is recorded as a miss instead of querying TMDB for an empty title.
	var details *tmdb.TmdbMovie
	var err error
	searchable := searchTitle != "" || job.baseline.TmdbID.Valid
	if searchable {
		details, err = s.lookupTmdbMovie(ctx, job.file.Path, searchTitle, titleYear.Year, job.baseline.TmdbID)
	}
	result.err, result.providerErr = err, err
	if err == nil {
		result.resolved = &enrichedMovie{baseline: job.baseline, inspection: result.inspection, tmdbMovie: details}
	}
	return result
}

func (s *Scanner) enrichMovies(ctx context.Context, scan *movieScanContext, report *scanReport, files []localFile) {
	now := s.now()
	candidates := make([]enrichmentJob, 0)
	for _, file := range files {
		if file.state != fileImported && file.state != fileUnchanged {
			continue
		}
		entry := scan.movieIndex[file.file.Path]
		if entry.ID != 0 && entry.enrichmentEligible(now) {
			candidates = append(candidates, enrichmentJob{file: file.file, baseline: entry})
		}
	}
	// Publish the total only once there is a provider to work through it;
	// otherwise the run ends showing a progress bar stuck at 0/N.
	if s.tmdb == nil {
		return
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].file.Path < candidates[j].file.Path })
	report.status.EnrichmentTotal = len(candidates)
	s.publish(report)
	consecutive := 0
	stopped := false
	scanner.RunWorkers(ctx, scanWorkers, candidates, func() bool { return !stopped }, s.prepareEnrichment,
		func(job enrichmentJob) {
			report.Activate(job.file.Path)
			s.publish(report)
		},
		func(result enrichmentResult) {
			report.Deactivate(result.job.file.Path)
			contextErr := ctx.Err()
			if contextErr == nil {
				report.status.EnrichmentProcessed++
				authentication, transient := tmdb.ProviderFailure(result.providerErr)
				if transient {
					consecutive++
				} else {
					consecutive = 0
				}
				if authentication || consecutive >= scanner.MaxConsecutiveProviderFailures {
					stopped = true
					report.Issue("", scanner.PhaseEnrichment, "TMDB enrichment stopped after provider failures. Pending movies will retry on a later scan.")
				}
				outcome := enrichmentSkipped
				if result.err == nil {
					outcome, result.err = s.persistEnrichment(ctx, scan, result.resolved)
				}
				canceledDuringPersist := result.err != nil && ctx.Err() != nil
				if !canceledDuringPersist {
					s.recordEnrichment(report, result, outcome)
				}
			}
			s.closeInspection(result.inspection, result.job.file.Path)
			s.publish(report)
		})
}

// recordEnrichment classifies one finished enrichment, mirroring recordLocal.
// A deferral means the file changed or is still inside the quiet period: the
// movie stays pending and retries on a later scan, so it is not a failure and
// raises no issue for the user.
func (s *Scanner) recordEnrichment(report *scanReport, result enrichmentResult, outcome enrichmentOutcome) {
	var deferred *scanner.FileDeferral
	isDeferred := errors.As(result.err, &deferred)
	switch {
	case isDeferred:
		s.logger.Debug("deferred movie enrichment", "path", result.job.file.Path, "reason", deferred.Reason)
	case result.err != nil:
		report.status.EnrichmentFailed++
		report.Issue(result.job.file.Path, scanner.PhaseEnrichment, "Descriptions could not be updated. Local movie data remains available; enrichment will retry later.")
		s.logger.Warn("movie enrichment failed", "path", result.job.file.Path, "error", result.err)
	case outcome == enrichmentUnmatched:
		report.status.EnrichmentUnmatched++
		report.Issue(result.job.file.Path, scanner.PhaseEnrichment, "TMDB returned no matching movie. You can use Identify or retry on a later scan.")
	}
}
