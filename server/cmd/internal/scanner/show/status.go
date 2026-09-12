package show

import (
	"igloo/cmd/internal/scanner"
)

// Status is the current or latest run. The counters mirror docs/openapi.json.
// Local counts are physical files; the enrichment counts are entities (shows,
// seasons, and episodes), which is why they do not add up to the file totals.
type Status struct {
	scanner.Progress
	Total               int `json:"total"`
	Processed           int `json:"processed"`
	Imported            int `json:"imported"`
	Updated             int `json:"updated"`
	Unchanged           int `json:"unchanged"`
	Failed              int `json:"failed"`
	Deferred            int `json:"deferred"`
	Deleted             int `json:"deleted"`
	Episodes            int `json:"episodes"`
	EnrichmentTotal     int `json:"enrichment_total"`
	EnrichmentProcessed int `json:"enrichment_processed"`
	Enriched            int `json:"enriched"`
	EnrichmentFailed    int `json:"enrichment_failed"`
	EnrichmentUnmatched int `json:"enrichment_unmatched"`
	PendingEnrichment   int `json:"pending_enrichment"`
}

// showScanEntry is one catalog file as the scan index saw it: identity,
// season ownership, the fingerprint baseline, and the episodes it links.
type showScanEntry struct {
	scanner.FileFingerprint
	ID             int64
	SeasonID       int64
	FilePath       string
	HasFingerprint bool
	Episodes       []int64
}

// showScanContext is the catalog state one run owns: the index keyed by
// cleaned path, written only after commit on the scan goroutine, and the
// seasons and episodes this run touched locally, which is what makes their
// metadata eligible for enrichment.
type showScanContext struct {
	index    map[string]showScanEntry
	seasons  map[int64]bool
	episodes map[int64]bool
}

func newShowScanContext(index map[string]showScanEntry) *showScanContext {
	return &showScanContext{index: index, seasons: make(map[int64]bool), episodes: make(map[int64]bool)}
}

func (scan *showScanContext) touch(seasonID int64, episodes []int64) {
	scan.seasons[seasonID] = true
	for _, episode := range episodes {
		scan.episodes[episode] = true
	}
}

// scanReport is owned by the scan goroutine. Episode totals are not kept here:
// they derive from the scan context, and syncCounts copies them before every
// publish.
type scanReport struct {
	scanner.Report
	status Status
	scan   *showScanContext
}

func newScanReport(status Status) *scanReport {
	return &scanReport{Report: scanner.NewReport(), status: status}
}

func (r *scanReport) syncCounts() {
	if r.scan == nil {
		return
	}
	r.status.Episodes = len(r.scan.episodes)
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
	s.status = Status{Progress: scanner.BeginProgress(scanner.PhaseDiscovery)}
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
		s.Logger.Info("show scan phase", "run", report.status.RunID, "phase", phase)
	}
	report.status.Phase = phase
	s.publish(report)
}
