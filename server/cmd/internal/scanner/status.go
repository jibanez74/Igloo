package scanner

import (
	"crypto/rand"
	"path/filepath"
	"sort"
	"time"
)

// ScanState and ScanPhase are the values published by a scanner's Status; they
// mirror the enums in docs/openapi.json. Each scanner emits the subset of
// phases its OpenAPI schema lists.
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

// Issue texts published by more than one scanner. They are user-facing and
// contain no directory paths or raw errors. A scanner that needs to say
// something different keeps its own local constant instead.
//
// ReasonFileDeferredNextScan states the quiet period in words; it must be kept
// in step with FileQuietPeriod.
const (
	ReasonDiscoveryEntry       = "A library entry could not be inspected. Existing records are preserved."
	ReasonLibraryUnavailable   = "The library directory is unavailable."
	ReasonDiscoveryInterrupted = "Library discovery was interrupted; missing files were not removed."
	ReasonCleanupUnsafe        = "Cleanup stopped because the library could not be safely checked."
	ReasonFileDeferredNextScan = "The file is still changing or has not been quiet for 60 seconds. It is retried on the next scan."
)

// MaxPublishedIssues caps the issue summaries carried by a status; IssueCount
// still reports every outstanding issue.
const MaxPublishedIssues = 100

// Progress is the scanner-agnostic part of a status report. Scanners embed it
// next to their own counters. It contains no directory paths or raw errors.
type Progress struct {
	RunID       string     `json:"run_id"`
	State       ScanState  `json:"state"`
	Phase       ScanPhase  `json:"phase"`
	StartedAt   *time.Time `json:"started_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	ActiveFiles []string   `json:"active_files"`
	IssueCount  int        `json:"issue_count"`
	Issues      []Issue    `json:"issues"`
}

type Issue struct {
	Filename string    `json:"filename"`
	Phase    ScanPhase `json:"phase"`
	Reason   string    `json:"reason"`
}

// BeginProgress starts a new run in the given phase.
func BeginProgress(phase ScanPhase) Progress {
	now := time.Now().UTC()
	return Progress{RunID: rand.Text(), State: StateRunning, Phase: phase, StartedAt: &now, UpdatedAt: &now}
}

// Clone returns a copy that shares nothing with p. A zero value reports idle so
// a process that never ran a scan still publishes a valid status.
func (p Progress) Clone() Progress {
	if p.StartedAt != nil {
		value := *p.StartedAt
		p.StartedAt = &value
	}
	if p.UpdatedAt != nil {
		value := *p.UpdatedAt
		p.UpdatedAt = &value
	}
	if p.FinishedAt != nil {
		value := *p.FinishedAt
		p.FinishedAt = &value
	}
	p.ActiveFiles = append([]string{}, p.ActiveFiles...)
	p.Issues = append([]Issue{}, p.Issues...)
	if p.State == "" {
		p.State, p.Phase = StateIdle, PhaseIdle
	}
	return p
}

// Report is the issue and active-file ledger of one run. It is owned by the
// scan goroutine and only its Publish output crosses to other goroutines.
type Report struct {
	issues map[string]Issue
	active map[string]bool
	// issueVersion changes whenever issues is mutated; Publish rebuilds the
	// sorted, truncated Issues slice only when it differs from publishedIssues.
	// Most publishes report file progress and leave issues untouched, so this
	// keeps Publish off an O(issues log issues) sort on every call.
	issueVersion    uint64
	publishedIssues uint64
}

func NewReport() Report {
	return Report{issues: make(map[string]Issue), active: make(map[string]bool)}
}

// Issue records one issue per phase and path. A scan-wide issue carries no
// path and reports an empty filename.
func (r *Report) Issue(path string, phase ScanPhase, reason string) {
	key := string(phase) + ":" + path
	filename := ""
	if path != "" {
		filename = filepath.Base(path)
	}
	entry := Issue{Filename: filename, Phase: phase, Reason: reason}
	existing, ok := r.issues[key]
	if ok && existing == entry {
		return
	}
	r.issues[key] = entry
	r.issueVersion++
}

func (r *Report) DropIssue(path string, phase ScanPhase) {
	key := string(phase) + ":" + path
	_, ok := r.issues[key]
	if !ok {
		return
	}
	delete(r.issues, key)
	r.issueVersion++
}

func (r *Report) IssueCount() int {
	return len(r.issues)
}

func (r *Report) Activate(path string) {
	r.active[path] = true
}

func (r *Report) Deactivate(path string) {
	delete(r.active, path)
}

// Publish stamps progress with the current time, active filenames and issue
// summary. The caller then copies its status under its own lock.
func (r *Report) Publish(progress *Progress) {
	now := time.Now().UTC()
	progress.UpdatedAt = &now
	progress.ActiveFiles = make([]string, 0, len(r.active))
	for path := range r.active {
		progress.ActiveFiles = append(progress.ActiveFiles, filepath.Base(path))
	}
	sort.Strings(progress.ActiveFiles)
	progress.IssueCount = len(r.issues)
	if r.issueVersion == r.publishedIssues {
		return
	}
	keys := make([]string, 0, len(r.issues))
	for key := range r.issues {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	limit := min(MaxPublishedIssues, len(keys))
	progress.Issues = make([]Issue, 0, limit)
	for _, key := range keys[:limit] {
		progress.Issues = append(progress.Issues, r.issues[key])
	}
	r.publishedIssues = r.issueVersion
}

// Finish resolves the terminal state: cancellation wins, a failure set earlier
// stays, and a run still marked running completed, with issues when any remain.
func (r *Report) Finish(progress *Progress, canceled bool) {
	if canceled {
		progress.State = StateCanceled
	}
	if progress.State == StateRunning {
		progress.State = StateCompleted
		if len(r.issues) > 0 {
			progress.State = StateCompletedWithIssues
		}
	}
	r.active = make(map[string]bool)
	now := time.Now().UTC()
	progress.FinishedAt = &now
}

// ProgressLogInterval is how often a running scan logs its progress. All three
// scanners pass it to StartProgressLog.
const ProgressLogInterval = 10 * time.Second

// StartProgressLog calls log every interval until the returned stop function
// is called; stop waits for an in-flight log call to return.
func StartProgressLog(interval time.Duration, log func()) func() {
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				log()
			}
		}
	}()
	return func() {
		close(done)
		<-finished
	}
}
