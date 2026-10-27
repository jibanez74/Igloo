package scanner

import (
	"reflect"
	"slices"
	"testing"
)

func TestOutstandingCarriesFileIssuesOutsideARunOfChanges(t *testing.T) {
	previous := NewReport()
	previous.Issue("/library/A/a.mkv", PhaseLocal, "failed")
	previous.Issue("/library/B/b.mkv", PhaseLocal, "deferred")
	previous.Issue("/library/disk2", PhaseCleanup, "not mounted")
	// A run of changes may settle these without naming their key.
	previous.Issue("/library/Show", PhaseEnrichment, "unmatched")
	previous.Issue("/library/Private", PhaseDiscovery, "unreadable")
	previous.Issue("", PhaseEnrichment, "provider down")
	var outstanding Outstanding
	outstanding.Keep(&previous)

	for _, tc := range []struct {
		changes *Changes
		want    []string
	}{
		// The run looks at B again; A it does not, so A's issue stays.
		{&Changes{Paths: []string{"B"}}, []string{"cleanup:/library/disk2", "local:/library/A/a.mkv"}},
		// A walk of the whole library raises its own.
		{&Changes{CatchUp: true}, nil},
		{nil, nil},
	} {
		report := NewReport()
		outstanding.Carry(&report, tc.changes, tc.changes.Resolve("/library"))
		var got []string
		for key := range report.carried {
			got = append(got, key)
		}
		slices.Sort(got)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%+v carried %q, want %q", tc.changes, got, tc.want)
		}
	}
}

// A run's state and the order it publishes its issues in are its own: issues
// it only carried describe an earlier run.
func TestCarriedIssuesNeitherSetTheStateNorCrowdOutTheRunsOwn(t *testing.T) {
	previous := NewReport()
	for _, name := range []string{"a", "b", "c"} {
		previous.Issue("/library/"+name+".mkv", PhaseLocal, "failed")
	}
	var outstanding Outstanding
	outstanding.Keep(&previous)
	changes := &Changes{Paths: []string{"z.mkv"}}

	report := NewReport()
	outstanding.Carry(&report, changes, changes.Resolve("/library"))
	progress := Progress{State: StateRunning}
	report.Publish(&progress)
	report.Finish(&progress, false)
	if progress.State != StateCompleted || progress.IssueCount != 3 {
		t.Fatalf("a run with only carried issues: state %s, %d issues", progress.State, progress.IssueCount)
	}

	report = NewReport()
	outstanding.Carry(&report, changes, changes.Resolve("/library"))
	progress = Progress{State: StateRunning}
	report.Publish(&progress)
	report.Issue("/library/z.mkv", PhaseLocal, "failed")
	// A carried issue the run raises again, with the same text, is its own.
	report.Issue("/library/a.mkv", PhaseLocal, "failed")
	report.Publish(&progress)
	report.Finish(&progress, false)
	if progress.State != StateCompletedWithIssues {
		t.Fatalf("a run with an issue of its own: state %s", progress.State)
	}
	var order []string
	for _, issue := range progress.Issues {
		order = append(order, issue.Filename)
	}
	if !slices.Equal(order, []string{"a.mkv", "z.mkv", "b.mkv", "c.mkv"}) {
		t.Fatalf("published order %q, want the run's own first", order)
	}
	// Dropping a carried issue republishes without it.
	report.DropIssue("/library/b.mkv", PhaseLocal)
	report.Publish(&progress)
	order = nil
	for _, issue := range progress.Issues {
		order = append(order, issue.Filename)
	}
	if !slices.Equal(order, []string{"a.mkv", "z.mkv", "c.mkv"}) || progress.IssueCount != 3 {
		t.Fatalf("after dropping a carried issue: %q (%d)", order, progress.IssueCount)
	}
}

// A library configured with a relative path keys its issues by it, and they
// carry the same.
func TestOutstandingCarriesIssuesOfARelativeLibraryRoot(t *testing.T) {
	previous := NewReport()
	previous.Issue("media/movies/A/a.mkv", PhaseLocal, "failed")
	var outstanding Outstanding
	outstanding.Keep(&previous)
	changes := &Changes{Paths: []string{"B"}}
	report := NewReport()
	outstanding.Carry(&report, changes, changes.Resolve("media/movies"))
	if len(report.carried) != 1 {
		t.Fatalf("carried %v", report.carried)
	}
	report = NewReport()
	changes = &Changes{Paths: []string{"A"}}
	outstanding.Carry(&report, changes, changes.Resolve("media/movies"))
	if len(report.carried) != 0 {
		t.Fatalf("an issue the run looks at again was carried: %v", report.carried)
	}
}
