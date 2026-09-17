package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
)

func TestParseShuffleExcludeIDs(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{
			name:   "no parameter yields an empty array",
			values: nil,
			want:   "[]",
		},
		{
			name:   "repeated parameters",
			values: []string{"3", "7", "11"},
			want:   "[3,7,11]",
		},
		{
			name:   "comma separated values in one parameter",
			values: []string{"3,7,11"},
			want:   "[3,7,11]",
		},
		{
			name:   "surrounding whitespace and empty fields are ignored",
			values: []string{" 3 , ,7 ", ""},
			want:   "[3,7]",
		},
		{
			// An exclusion is an optimization. Rejecting the request over one
			// bad id would stop playback for no benefit.
			name:   "unparseable and non-positive ids are skipped, not rejected",
			values: []string{"abc", "0", "-4", "9"},
			want:   "[9]",
		},
		{
			name:   "duplicates are collapsed",
			values: []string{"5", "5", "5,5"},
			want:   "[5]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseShuffleExcludeIDs(tt.values)
			if got != tt.want {
				t.Errorf("parseShuffleExcludeIDs(%q) = %s, want %s", tt.values, got, tt.want)
			}
		})
	}
}

func TestParseShuffleExcludeIDsCapsTheList(t *testing.T) {
	values := make([]string, 0, maxShuffleExcludeIDs+50)
	for i := 1; i <= maxShuffleExcludeIDs+50; i++ {
		values = append(values, fmt.Sprintf("%d", i))
	}

	got := parseShuffleExcludeIDs(values)

	// Cheap structural check: the encoded array must hold exactly the cap.
	var count int
	for _, r := range got {
		if r == ',' {
			count++
		}
	}
	if count+1 != maxShuffleExcludeIDs {
		t.Fatalf("expected %d ids, got %d", maxShuffleExcludeIDs, count+1)
	}
}

// The accepted-id cap alone does not bound the work: skipped values never reach
// it, so a request made entirely of junk would be walked in full.
func TestParseShuffleExcludeIDsBoundsSkippedFields(t *testing.T) {
	junk := make([]string, 0, maxShuffleExcludeFields+100)
	for range maxShuffleExcludeFields + 100 {
		junk = append(junk, "abc")
	}

	// A real id parked past the field budget must not be reached — that is the
	// observable proof the walk stopped.
	junk = append(junk, "42")

	if got := parseShuffleExcludeIDs(junk); got != "[]" {
		t.Errorf("parseShuffleExcludeIDs(junk) = %s, want []", got)
	}

	// The same id inside the budget is still collected, so the bound is not
	// simply dropping everything.
	if got := parseShuffleExcludeIDs([]string{"42"}); got != "[42]" {
		t.Errorf("parseShuffleExcludeIDs([42]) = %s, want [42]", got)
	}
}

// One value holding many comma-separated fields is the same walk, so the budget
// has to apply inside a value as well as across them.
func TestParseShuffleExcludeIDsBoundsOneLongValue(t *testing.T) {
	fields := make([]string, 0, maxShuffleExcludeFields+100)
	for range maxShuffleExcludeFields + 100 {
		fields = append(fields, "abc")
	}
	fields = append(fields, "42")

	if got := parseShuffleExcludeIDs([]string{strings.Join(fields, ",")}); got != "[]" {
		t.Errorf("parseShuffleExcludeIDs(one long value) = %s, want []", got)
	}
}

func seedShuffleTracks(t *testing.T, app *Application, count int) {
	t.Helper()

	for i := 1; i <= count; i++ {
		_, err := app.DB.Exec(
			`INSERT INTO tracks (
				title, sort_title, file_path, file_name, container, mime_type,
				codec, size, track_index, duration, disc, channels,
				channel_layout, bit_rate, profile
			) VALUES (?, ?, ?, ?, 'flac', 'audio/flac', 'flac', 1, ?, 100, 1, '2', 'stereo', 900000, '')`,
			fmt.Sprintf("Song %d", i),
			fmt.Sprintf("song %d", i),
			fmt.Sprintf("/music/%d.flac", i),
			fmt.Sprintf("%d.flac", i),
			i,
		)
		if err != nil {
			t.Fatalf("failed to seed track %d: %v", i, err)
		}
	}
}

func TestGetRandomTracksWithoutExclusionsReturnsTheWholeLibrary(t *testing.T) {
	app := setupSessionTestApp(t)
	seedShuffleTracks(t, app, 5)

	// The empty case is the trap: an empty sqlc.slice renders as `NOT IN (NULL)`,
	// which is NULL rather than true and would silently return nothing. The
	// query uses json_each instead, so "[]" must exclude nothing at all.
	tracks, err := app.Queries.GetRandomTracks(context.Background(), database.GetRandomTracksParams{
		ExcludeIds: parseShuffleExcludeIDs(nil),
		RowLimit:   10,
	})
	if err != nil {
		t.Fatalf("GetRandomTracks failed: %v", err)
	}

	if len(tracks) != 5 {
		t.Fatalf("expected all 5 seeded tracks, got %d", len(tracks))
	}
}

func TestGetRandomTracksOmitsExcludedIDs(t *testing.T) {
	app := setupSessionTestApp(t)
	seedShuffleTracks(t, app, 20)

	excluded := map[int64]bool{}
	for i := int64(1); i <= 15; i++ {
		excluded[i] = true
	}

	tracks, err := app.Queries.GetRandomTracks(context.Background(), database.GetRandomTracksParams{
		ExcludeIds: parseShuffleExcludeIDs([]string{"1,2,3,4,5,6,7,8,9,10,11,12,13,14,15"}),
		RowLimit:   20,
	})
	if err != nil {
		t.Fatalf("GetRandomTracks failed: %v", err)
	}

	if len(tracks) != 5 {
		t.Fatalf("expected the 5 unexcluded tracks, got %d", len(tracks))
	}
	for _, track := range tracks {
		if excluded[track.ID] {
			t.Errorf("track %d was excluded but came back anyway", track.ID)
		}
	}
}

func TestGetRandomTracksReturnsNothingWhenEverythingIsExcluded(t *testing.T) {
	app := setupSessionTestApp(t)
	seedShuffleTracks(t, app, 3)

	// This is how the client learns the library is exhausted, so an empty
	// result here has to stay empty rather than fall back to the full table.
	tracks, err := app.Queries.GetRandomTracks(context.Background(), database.GetRandomTracksParams{
		ExcludeIds: parseShuffleExcludeIDs([]string{"1", "2", "3"}),
		RowLimit:   10,
	})
	if err != nil {
		t.Fatalf("GetRandomTracks failed: %v", err)
	}

	if len(tracks) != 0 {
		t.Fatalf("expected no tracks, got %d", len(tracks))
	}
}

func TestGetRandomTracksHonoursTheLimit(t *testing.T) {
	app := setupSessionTestApp(t)
	seedShuffleTracks(t, app, 30)

	tracks, err := app.Queries.GetRandomTracks(context.Background(), database.GetRandomTracksParams{
		ExcludeIds: parseShuffleExcludeIDs(nil),
		RowLimit:   7,
	})
	if err != nil {
		t.Fatalf("GetRandomTracks failed: %v", err)
	}

	if len(tracks) != 7 {
		t.Fatalf("expected 7 tracks, got %d", len(tracks))
	}
}
