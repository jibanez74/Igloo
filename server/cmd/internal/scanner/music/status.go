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
// track this run, for the completion log.
func (r *scanReport) tracksLinked() int {
	if r.scan == nil {
		return 0
	}
	return r.scan.tracksLinked
}

func (s *Scanner) Status() Status {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	result := s.status
	result.Progress = result.Progress.Clone()
	return result
}

func (s *Scanner) beginReport() {
	s.statusMu.Lock()
	s.status = Status{Progress: scanner.BeginProgress(scanner.PhaseLocal, scanner.ScopeLibrary)}
	s.statusMu.Unlock()
}

func (s *Scanner) publish(report *scanReport) {
	report.syncCounts()
	report.Publish(&report.status.Progress)
	s.statusMu.Lock()
	s.status = report.status
	s.statusMu.Unlock()
}

func (s *Scanner) phase(report *scanReport, phase scanner.ScanPhase) {
	if report.status.Phase != phase {
		s.logger.Info("music scan phase", "run", report.status.RunID, "phase", phase)
	}
	report.status.Phase = phase
	s.publish(report)
}
