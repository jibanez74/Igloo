package movie

import (
	"crypto/rand"
	"path/filepath"
	"sort"
	"time"
)

// ScanState and ScanPhase are the values published by Status; they mirror the
// enums in docs/openapi.json.
type ScanState string

const (
	StateIdle                ScanState = "idle"
	StateRunning             ScanState = "running"
	StateCompleted           ScanState = "completed"
	StateCompletedWithIssues ScanState = "completed-with-issues"
	StateCanceled            ScanState = "canceled"
	StateFailed              ScanState = "failed"
)

type ScanPhase string

const (
	PhaseIdle       ScanPhase = "idle"
	PhaseDiscovery  ScanPhase = "discovery"
	PhaseLocal      ScanPhase = "local"
	PhaseRetryWait  ScanPhase = "retry-wait"
	PhaseCleanup    ScanPhase = "cleanup"
	PhaseEnrichment ScanPhase = "enrichment"
)

// Status is the current or latest run. It contains no directory paths or raw errors.
type Status struct {
	RunID               string     `json:"run_id"`
	State               ScanState  `json:"state"`
	Phase               ScanPhase  `json:"phase"`
	StartedAt           *time.Time `json:"started_at"`
	UpdatedAt           *time.Time `json:"updated_at"`
	FinishedAt          *time.Time `json:"finished_at"`
	ActiveFiles         []string   `json:"active_files"`
	Total               int        `json:"total"`
	Processed           int        `json:"processed"`
	Imported            int        `json:"imported"`
	Updated             int        `json:"updated"`
	Unchanged           int        `json:"unchanged"`
	Failed              int        `json:"failed"`
	Deferred            int        `json:"deferred"`
	Deleted             int        `json:"deleted"`
	EnrichmentTotal     int        `json:"enrichment_total"`
	EnrichmentProcessed int        `json:"enrichment_processed"`
	Enriched            int        `json:"enriched"`
	EnrichmentFailed    int        `json:"enrichment_failed"`
	EnrichmentUnmatched int        `json:"enrichment_unmatched"`
	PendingEnrichment   int        `json:"pending_enrichment"`
	IssueCount          int        `json:"issue_count"`
	Issues              []Issue    `json:"issues"`
}

type Issue struct {
	Filename string    `json:"filename"`
	Phase    ScanPhase `json:"phase"`
	Reason   string    `json:"reason"`
}

// scanReport is owned by the scan goroutine. The enrichment counters are not
// kept here: they derive from the scan context, which changes only on commit,
// and syncCounts copies them before every publish.
type scanReport struct {
	status Status
	issues map[string]Issue
	active map[string]bool
	scan   *movieScanContext
}

func (r *scanReport) syncCounts() {
	if r.scan == nil {
		return
	}
	r.status.Enriched = r.scan.enriched
	r.status.PendingEnrichment = r.scan.pending
}

func (r *scanReport) issue(path string, phase ScanPhase, reason string) {
	r.issues[string(phase)+":"+path] = Issue{Filename: filepath.Base(path), Phase: phase, Reason: reason}
}

func (s *Scanner) Status() Status {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	result := s.status
	if result.StartedAt != nil {
		value := *result.StartedAt
		result.StartedAt = &value
	}
	if result.UpdatedAt != nil {
		value := *result.UpdatedAt
		result.UpdatedAt = &value
	}
	if result.FinishedAt != nil {
		value := *result.FinishedAt
		result.FinishedAt = &value
	}
	result.ActiveFiles = append([]string{}, result.ActiveFiles...)
	result.Issues = append([]Issue{}, result.Issues...)
	if result.State == "" {
		result.State, result.Phase = StateIdle, PhaseIdle
	}
	return result
}

func (s *Scanner) beginReport() {
	now := time.Now().UTC()
	s.statusMu.Lock()
	s.status = Status{RunID: rand.Text(), State: StateRunning, Phase: PhaseDiscovery, StartedAt: &now, UpdatedAt: &now}
	s.statusMu.Unlock()
}

func (s *Scanner) publish(report *scanReport) {
	now := time.Now().UTC()
	report.syncCounts()
	report.status.UpdatedAt = &now
	report.status.ActiveFiles = make([]string, 0, len(report.active))
	for path := range report.active {
		report.status.ActiveFiles = append(report.status.ActiveFiles, filepath.Base(path))
	}
	sort.Strings(report.status.ActiveFiles)
	report.status.IssueCount = len(report.issues)
	keys := make([]string, 0, len(report.issues))
	for key := range report.issues {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	report.status.Issues = make([]Issue, 0, min(100, len(keys)))
	for _, key := range keys[:min(100, len(keys))] {
		report.status.Issues = append(report.status.Issues, report.issues[key])
	}
	s.statusMu.Lock()
	s.status = report.status
	s.statusMu.Unlock()
}

func (s *Scanner) phase(report *scanReport, phase ScanPhase) {
	if report.status.Phase != phase {
		s.logger.Info("movie scan phase", "run", report.status.RunID, "phase", phase)
	}
	report.status.Phase = phase
	s.publish(report)
}
