package scanner

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// Every issue is keyed by phase and path, so two failing entries stay two
// issues instead of the second overwriting the first.
func TestReportIssuesDoNotCollide(t *testing.T) {
	report := NewReport()

	report.Issue("/library/a.mkv", PhaseDiscovery, "unreadable")
	report.Issue("/library/b.mkv", PhaseDiscovery, "unreadable")
	report.Issue("/library/a.mkv", PhaseLocal, "unreadable")

	if report.IssueCount() != 3 {
		t.Fatalf("issues collided: %v", report.issues)
	}
}

// A scan-wide issue carries no path. filepath.Base("") is ".", which reached the
// scan status API as a filename.
func TestReportIssueWithoutPathHasNoFilename(t *testing.T) {
	report := NewReport()
	report.Issue("", PhaseEnrichment, "Enrichment stopped after provider failures.")
	report.Issue("/movies/Example.mkv", PhaseLocal, "Unable to inspect this movie.")

	for key, issue := range report.issues {
		switch issue.Phase {
		case PhaseEnrichment:
			if issue.Filename != "" {
				t.Fatalf("scan-wide issue %q filename = %q, want empty", key, issue.Filename)
			}
		case PhaseLocal:
			if issue.Filename != "Example.mkv" {
				t.Fatalf("file issue %q filename = %q, want the base name", key, issue.Filename)
			}
		}
	}
}

// Publish rebuilds the sorted issue list only when the issue set changed, so
// per-file progress publishes stay off an O(n log n) sort.
func TestPublishSkipsUnchangedIssues(t *testing.T) {
	report := NewReport()
	progress := BeginProgress(PhaseLocal)

	report.Issue("/library/a.mkv", PhaseLocal, "unreadable")
	report.Publish(&progress)
	first := report.publishedIssues
	report.Activate("/library/b.mkv")
	report.Publish(&progress)
	if report.publishedIssues != first {
		t.Fatalf("unchanged issues rebuilt the published list")
	}
	if progress.IssueCount != 1 || len(progress.Issues) != 1 || len(progress.ActiveFiles) != 1 || progress.ActiveFiles[0] != "b.mkv" {
		t.Fatalf("issue lost across publishes: %+v", progress)
	}

	report.DropIssue("/library/a.mkv", PhaseLocal)
	report.Deactivate("/library/b.mkv")
	report.Publish(&progress)
	if report.publishedIssues == first || progress.IssueCount != 0 || len(progress.Issues) != 0 || len(progress.ActiveFiles) != 0 {
		t.Fatalf("dropped issue not republished: %+v", progress)
	}
}

func TestPublishCapsIssueSummaries(t *testing.T) {
	report := NewReport()
	progress := BeginProgress(PhaseLocal)
	for i := 0; i < MaxPublishedIssues+10; i++ {
		report.Issue(fmt.Sprintf("/library/%03d.mkv", i), PhaseLocal, "unreadable")
	}
	report.Publish(&progress)
	if progress.IssueCount != MaxPublishedIssues+10 || len(progress.Issues) != MaxPublishedIssues {
		t.Fatalf("issue_count=%d issues=%d", progress.IssueCount, len(progress.Issues))
	}
	if progress.Issues[0].Filename != "000.mkv" {
		t.Fatalf("issues are not sorted: %+v", progress.Issues[0])
	}
}

func TestCloneIsolatesAndReportsIdle(t *testing.T) {
	var zero Progress
	idle := zero.Clone()
	if idle.State != StateIdle || idle.Phase != PhaseIdle || idle.ActiveFiles == nil || idle.Issues == nil {
		t.Fatalf("zero progress did not clone to idle with empty slices: %+v", idle)
	}

	report := NewReport()
	progress := BeginProgress(PhaseDiscovery)
	report.Issue("/library/a.mkv", PhaseLocal, "unreadable")
	report.Activate("/library/b.mkv")
	report.Publish(&progress)
	clone := progress.Clone()
	clone.Issues[0].Reason = "changed"
	clone.ActiveFiles[0] = "changed"
	*clone.UpdatedAt = clone.UpdatedAt.Add(time.Hour)
	if progress.Issues[0].Reason == "changed" || progress.ActiveFiles[0] == "changed" || progress.UpdatedAt.Equal(*clone.UpdatedAt) {
		t.Fatalf("clone shares memory with the original: %+v", progress)
	}
	if progress.RunID == "" || progress.State != StateRunning || progress.StartedAt == nil {
		t.Fatalf("BeginProgress did not start a run: %+v", progress)
	}
}

func TestFinishResolvesTerminalState(t *testing.T) {
	tests := []struct {
		name     string
		state    ScanState
		issues   int
		canceled bool
		want     ScanState
	}{
		{"completed", StateRunning, 0, false, StateCompleted},
		{"completed with issues", StateRunning, 1, false, StateCompletedWithIssues},
		{"canceled overrides issues", StateRunning, 1, true, StateCanceled},
		{"failed stays failed", StateFailed, 1, false, StateFailed},
		{"canceled overrides failed", StateFailed, 1, true, StateCanceled},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := NewReport()
			progress := BeginProgress(PhaseLocal)
			progress.State = tc.state
			for i := 0; i < tc.issues; i++ {
				report.Issue(fmt.Sprintf("/library/%d.mkv", i), PhaseLocal, "unreadable")
			}
			report.Activate("/library/active.mkv")
			report.Finish(&progress, tc.canceled)
			report.Publish(&progress)
			if progress.State != tc.want || progress.FinishedAt == nil || len(progress.ActiveFiles) != 0 {
				t.Fatalf("state=%s finished=%v active=%v want %s", progress.State, progress.FinishedAt, progress.ActiveFiles, tc.want)
			}
		})
	}
}

func TestStartProgressLogStopsAndWaits(t *testing.T) {
	var calls atomic.Int32
	fired := make(chan struct{}, 1)
	stop := StartProgressLog(time.Millisecond, func() {
		calls.Add(1)
		select {
		case fired <- struct{}{}:
		default:
		}
	})
	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("progress log never fired")
	}
	stop()
	// stop joins the ticker goroutine, so the count must be final once it
	// returns; a stop that only signalled would keep ticking during the wait.
	final := calls.Load()
	time.Sleep(10 * time.Millisecond)
	if final < 1 || calls.Load() != final {
		t.Fatalf("progress log fired after stop: %d then %d", final, calls.Load())
	}
}
