package movie

import (
	"context"
	"sort"
	"time"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/tmdb"
)

// maxConsecutiveProviderFailures stops new enrichment dispatch for the rest of
// the scan; pending movies retry on a later scan.
const maxConsecutiveProviderFailures = 3

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
	searchTitle := NormalizeTitleForSearch(titleYear.Title)
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
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].file.Path < candidates[j].file.Path })
	report.status.EnrichmentTotal = len(candidates)
	s.publish(report)
	if s.tmdb == nil {
		return
	}
	consecutive := 0
	stopped := false
	scanner.RunWorkers(ctx, scanWorkers, candidates, func() bool { return !stopped }, s.prepareEnrichment,
		func(job enrichmentJob) {
			report.active[job.file.Path] = true
			s.publish(report)
		},
		func(result enrichmentResult) {
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
				if authentication || consecutive >= maxConsecutiveProviderFailures {
					stopped = true
					report.issue("", PhaseEnrichment, "TMDB enrichment stopped after provider failures. Pending movies will retry on a later scan.")
				}
				outcome := enrichmentSkipped
				if result.err == nil {
					outcome, result.err = s.persistEnrichment(ctx, scan, result.resolved)
				}
				canceledDuringPersist := result.err != nil && ctx.Err() != nil
				if !canceledDuringPersist {
					switch {
					case result.err != nil:
						report.status.EnrichmentFailed++
						report.issue(result.job.file.Path, PhaseEnrichment, "Descriptions could not be updated. Local movie data remains available; enrichment will retry later.")
						s.logger.Warn("movie enrichment failed", "path", result.job.file.Path, "error", result.err)
					case outcome == enrichmentUnmatched:
						report.status.EnrichmentUnmatched++
						report.issue(result.job.file.Path, PhaseEnrichment, "TMDB returned no matching movie. You can use Identify or retry on a later scan.")
					}
				}
			}
			s.closeInspection(result.inspection, result.job.file.Path)
			s.publish(report)
		})
}
