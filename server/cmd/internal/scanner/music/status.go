package music

import (
	"igloo/cmd/internal/scanner"
)

// Status is the current or latest run. The counters mirror docs/openapi.json.
// Discovery and processing are interleaved, so Total grows during the local
// phase. The provider tallies count unique artists and albums, one outcome
// each per scan, across inline resolution and the enrichment passes.
type Status struct {
	scanner.Progress
	scanner.Counts
	// Lookups is the background worker's current or latest pass, which runs
	// between scans.
	Lookups LookupStatus `json:"lookups"`
}

// scanReport is owned by the scan goroutine. The provider tallies are not kept
// here: they derive from the scan context and syncCounts copies them before
// every publish.
type scanReport struct {
	scanner.Report
	status Status
	scan   *musicScanContext
}

func newScanReport(status Status) *scanReport {
	return &scanReport{Report: scanner.NewReport(), status: status}
}

func (r *scanReport) syncCounts() {
	if r.scan == nil {
		return
	}
	r.status.Enriched = r.scan.enrichmentCounts[musicStatusMatched]
	r.status.EnrichmentFailed = r.scan.enrichmentCounts[musicStatusFailed]
	r.status.EnrichmentUnmatched = r.scan.enrichmentCounts[musicStatusUnmatched]
}

// tracksLinked is the number of tracks paired with an Apple Music or Spotify
// track this run, for catalog revisions and the completion log.
func (r *scanReport) tracksLinked() int {
	if r.scan == nil {
		return 0
	}
	return r.scan.tracksLinked
}

// metadataChanges is what the catalog revision counts beyond the file and
// enrichment counters: track pairings, dropped album matches, and the
// matched tallies those drops took back out of Enriched.
func (r *scanReport) metadataChanges() int {
	if r.scan == nil {
		return 0
	}
	return r.scan.tracksLinked + r.scan.matchesDropped + r.scan.matchesRevoked
}

func (s *Scanner) Status() Status {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	result := s.status
	result.Progress = result.Progress.Clone()
	result.Lookups = s.lookupStatus
	result.Lookups.State = s.lookups.state()
	if result.Lookups.Stage != nil {
		stage := *result.Lookups.Stage
		result.Lookups.Stage = &stage
	}
	return result
}

func (s *Scanner) beginReport(scope scanner.ScanScope) {
	s.statusMu.Lock()
	s.status = Status{Progress: scanner.BeginProgress(scanner.PhaseLocal, scope)}
	s.revision.Stamp(&s.status.Progress, s.status.Counts, 0)
	s.statusMu.Unlock()
}

func (s *Scanner) publish(report *scanReport) {
	report.syncCounts()
	report.Publish(&report.status.Progress)
	s.statusMu.Lock()
	s.status = report.status
	s.revision.Stamp(&s.status.Progress, s.status.Counts, report.metadataChanges()+report.Moves())
	s.statusMu.Unlock()
}

func (s *Scanner) phase(report *scanReport, phase scanner.ScanPhase) {
	if report.status.Phase != phase {
		s.logger.Info("music scan phase", "run", report.status.RunID, "phase", phase)
	}
	report.status.Phase = phase
	s.publish(report)
}
