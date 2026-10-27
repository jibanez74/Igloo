package scanner

import (
	"reflect"
	"testing"
)

func TestOutstandingCarriesFileIssuesOutsideARunOfChanges(t *testing.T) {
	previous := NewReport()
	previous.Issue("/library/A/a.mkv", PhaseLocal, "failed")
	previous.Issue("/library/B/b.mkv", PhaseLocal, "deferred")
	previous.Issue("", PhaseEnrichment, "provider down")
	previous.Issue("Show: Season 1", PhaseEnrichment, "unmatched")
	var outstanding Outstanding
	outstanding.Keep(&previous)

	for _, tc := range []struct {
		changes *Changes
		want    []string
	}{
		// The run looks at B again; A it does not, so A's issue stays.
		{&Changes{Paths: []string{"B"}}, []string{"local:/library/A/a.mkv"}},
		// A walk of the whole library raises its own.
		{&Changes{CatchUp: true}, nil},
		{nil, nil},
	} {
		report := NewReport()
		outstanding.Carry(&report, tc.changes, tc.changes.Resolve("/library"))
		var got []string
		for key := range report.issues {
			got = append(got, key)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%+v carried %q, want %q", tc.changes, got, tc.want)
		}
	}
}
