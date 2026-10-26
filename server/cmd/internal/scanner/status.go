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

// ScanScope tells a library scan apart from a run that applies only the
// changes the library watcher detected.
type ScanScope string

const (
	ScopeLibrary ScanScope = "library"
	ScopeChanges ScanScope = "changes"
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
	ReasonFileDeferredWatched  = "The file is still changing. The library watcher retries it once it is quiet."
	ReasonCleanupSkippedEmpty  = "The library looked empty, so nothing was removed. If its files are really gone, run a library scan."
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
	Scope       ScanScope  `json:"scope"`
	StartedAt   *time.Time `json:"started_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	ActiveFiles []string   `json:"active_files"`
	IssueCount  int        `json:"issue_count"`
	Issues      []Issue    `json:"issues"`
	// CatalogRevision is the scanner's Revision when the report was published.
	CatalogRevision int64 `json:"catalog_revision"`
}

// Counts are the file and enrichment counters every scanner's status carries;
// scanners embed it beside Progress and their own extra counters.
type Counts struct {
	Total               int `json:"total"`
	Processed           int `json:"processed"`
	Imported            int `json:"imported"`
	Updated             int `json:"updated"`
	Unchanged           int `json:"unchanged"`
	Failed              int `json:"failed"`
	Deferred            int `json:"deferred"`
	Deleted             int `json:"deleted"`
	EnrichmentTotal     int `json:"enrichment_total"`
	EnrichmentProcessed int `json:"enrichment_processed"`
	Enriched            int `json:"enriched"`
	EnrichmentFailed    int `json:"enrichment_failed"`
	EnrichmentUnmatched int `json:"enrichment_unmatched"`
}

// Revision counts the catalog changes a scanner made across its runs: every
// file imported, updated or deleted, every entry enriched, and additional
// committed metadata changes supplied by the scanner. A client that
// compares it sees a change even from runs it never saw a report of. Each
// scanner keeps one beside its status, under the same lock.
type Revision struct {
	total   int64
	run     string
	counted int
}

// Stamp counts what progress's run changed since its last stamp and sets the
// running total on progress. A new run starts counting from zero, so the total
// only grows. additionalChanges is cumulative for this run, like counts, and
// their sum must never shrink within a run: a change that lowers a counter,
// such as a match taken back out of Enriched, must add as much back through
// additionalChanges, or the changes after it are not counted.
func (r *Revision) Stamp(progress *Progress, counts Counts, additionalChanges int) {
	if progress.RunID != r.run {
		r.run, r.counted = progress.RunID, 0
	}
	changed := counts.Imported + counts.Updated + counts.Deleted + counts.Enriched + additionalChanges
	if changed > r.counted {
		r.total += int64(changed - r.counted)
		r.counted = changed
	}
	progress.CatalogRevision = r.total
}

type Issue struct {
	Filename string    `json:"filename"`
	Phase    ScanPhase `json:"phase"`
	Reason   string    `json:"reason"`
}

// BeginProgress starts a new run in the given phase and scope.
func BeginProgress(phase ScanPhase, scope ScanScope) Progress {
	now := time.Now().UTC()
	return Progress{RunID: rand.Text(), State: StateRunning, Phase: phase, Scope: scope, StartedAt: &now, UpdatedAt: &now}
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
		p.State, p.Phase, p.Scope = StateIdle, PhaseIdle, ScopeLibrary
	}
	return p
}

// Report is the issue and active-file ledger of one run. It is owned by the
// scan goroutine and only its Publish output crosses to other goroutines.
type Report struct {
	issues map[string]Issue
	active map[string]bool
	// issueVersion changes whenever a mutation can change the published
	// Issues; Publish rebuilds the sorted, truncated slice only when it differs
	// from publishedIssues. Most publishes report file progress and leave
	// issues untouched, and once the published slice is full, a key that sorts
	// after its last entry cannot enter it. Either way Publish stays off an
	// O(issues log issues) sort, which a scan where most files raise an issue
	// (every file deferred after a recursive chmod) would otherwise pay once
	// per file.
	issueVersion    uint64
	publishedIssues uint64
	// publishedLast is the key of the last published issue while the
	// published slice holds MaxPublishedIssues of them, and empty otherwise.
	publishedLast string
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
	r.markIssueChanged(key)
}

func (r *Report) DropIssue(path string, phase ScanPhase) {
	key := string(phase) + ":" + path
	_, ok := r.issues[key]
	if !ok {
		return
	}
	delete(r.issues, key)
	r.markIssueChanged(key)
}

// markIssueChanged records a change to key unless the published slice is full
// and key sorts after its last entry: every published key is then still
// present and still sorts first, so the slice Publish would build is the same.
func (r *Report) markIssueChanged(key string) {
	beyondPublished := r.publishedLast != "" && key > r.publishedLast
	if beyondPublished {
		return
	}
	r.issueVersion++
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
	r.publishedLast = ""
	if limit == MaxPublishedIssues {
		r.publishedLast = keys[limit-1]
	}
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
