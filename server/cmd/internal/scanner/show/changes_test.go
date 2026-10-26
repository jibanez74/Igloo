package show

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
)

func episodeID(t *testing.T, s *Scanner, path string) int64 {
	t.Helper()
	var id int64
	err := s.DB.QueryRow(`SELECT ef.episode_id FROM show_episode_files ef JOIN show_files f ON f.id = ef.file_id
		WHERE f.file_path = ?`, path).Scan(&id)
	if err != nil {
		t.Fatalf("episode at %s: %v", path, err)
	}
	return id
}

func TestShowChangesRunAppliesOnlyTheChangedPaths(t *testing.T) {
	s, probe, root := setupScanner(t)
	e1 := scannertest.WriteFile(t, filepath.Join(root, "Show (2020)/Season 1/S01E01.mkv"), "e1")
	scannertest.WriteFile(t, filepath.Join(root, "Show (2020)/Season 1/S01E02.mkv"), "e2")
	other := scannertest.WriteFile(t, filepath.Join(root, "Other (2021)/Season 1/S01E01.mkv"), "o1")
	scanOK(t, s, root)
	pilot := episodeID(t, s, e1)
	_, err := s.DB.Exec("INSERT INTO users (id, name, email, password) VALUES (1, 'Viewer', 'viewer@example.test', 'unused')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB.Exec("INSERT INTO show_episode_watch_progress (user_id, episode_id, progress_sec, duration_sec, save_session_id, save_sequence) VALUES (1, ?, 123, 1200, 'session', 7)", pilot)
	if err != nil {
		t.Fatal(err)
	}
	if probe.Calls() != 3 {
		t.Fatalf("library scan probes = %d", probe.Calls())
	}

	// Renaming an episode file lands both names in one run: the new name is
	// processed before cleanup, so the episode and its progress survive.
	renamed := filepath.Join(root, "Show (2020)/Season 1/S01E01 - Pilot.mkv")
	err = os.Rename(e1, renamed)
	if err != nil {
		t.Fatal(err)
	}
	scannertest.WriteFile(t, filepath.Join(root, "Show (2020)/Season 1/S01E03.mkv"), "e3")
	// An extras folder is never scanned, so naming it is not a failure.
	scannertest.WriteFile(t, filepath.Join(root, "Show (2020)/Season 1/Extras/Behind.mkv"), "x")
	err = os.Remove(other)
	if err != nil {
		t.Fatal(err)
	}

	s.scanChanges(root, scanner.Changes{Paths: []string{
		"Show (2020)/Season 1/S01E01.mkv",
		"Show (2020)/Season 1/S01E01 - Pilot.mkv",
		"Show (2020)/Season 1/S01E03.mkv",
		"Show (2020)/Season 1/Extras",
		"Show (2020)/Season 1/Extras/Behind.mkv",
	}})
	status := s.Status()
	if status.Scope != scanner.ScopeChanges || status.State != scanner.StateCompleted {
		t.Fatalf("scope=%s state=%s issues=%+v", status.Scope, status.State, status.Issues)
	}
	if status.Total != 2 || status.Imported != 2 || status.Deleted != 1 || status.Failed != 0 {
		t.Fatalf("counts = %+v", status.Counts)
	}
	if probe.Calls() != 5 {
		t.Fatalf("probes = %d", probe.Calls())
	}
	renamedEpisode := episodeID(t, s, renamed)
	if renamedEpisode != pilot {
		t.Fatal("renaming the episode file replaced the episode")
	}
	var progress, duration float64
	var session string
	var sequence int
	err = s.DB.QueryRow("SELECT progress_sec, duration_sec, save_session_id, save_sequence FROM show_episode_watch_progress WHERE user_id = 1 AND episode_id = ?", pilot).Scan(&progress, &duration, &session, &sequence)
	if err != nil {
		t.Fatal(err)
	}
	if progress != 123 || duration != 1200 || session != "session" || sequence != 7 {
		t.Fatalf("rename changed watch progress: %v %v %q %d", progress, duration, session, sequence)
	}
	// The other show's file is gone but was not reported, so it stays.
	if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_files") != 4 {
		t.Fatalf("show_files = %d", scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_files"))
	}

	// Removing a whole season folder removes every row beneath it.
	err = os.RemoveAll(filepath.Join(root, "Show (2020)/Season 1"))
	if err != nil {
		t.Fatal(err)
	}
	s.scanChanges(root, scanner.Changes{Paths: []string{"Show (2020)/Season 1"}})
	if s.Status().Deleted != 3 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_files") != 1 {
		t.Fatalf("season removal: %+v", s.Status().Counts)
	}
	if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM shows") != 1 {
		t.Fatal("the emptied show was not pruned")
	}
}

func TestShowCatchUpRefusesToEmptyALibraryThatLooksUnmounted(t *testing.T) {
	s, _, root := setupScanner(t)
	scannertest.WriteFile(t, filepath.Join(root, "Show (2020)/Season 1/S01E01.mkv"), "e1")
	scanOK(t, s, root)
	err := os.RemoveAll(filepath.Join(root, "Show (2020)"))
	if err != nil {
		t.Fatal(err)
	}
	s.scanChanges(root, scanner.Changes{CatchUp: true})
	status := s.Status()
	if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_files") != 1 || status.Deleted != 0 {
		t.Fatalf("catch-up removed rows: %+v", status.Counts)
	}
	if status.IssueCount != 1 || status.Issues[0].Reason != scanner.ReasonCleanupSkippedEmpty {
		t.Fatalf("issues = %+v", status.Issues)
	}
}

func TestShowChangesRunHandsBackDeferredFiles(t *testing.T) {
	s, _, root := setupScanner(t)
	path := scannertest.WriteFile(t, filepath.Join(root, "Show (2020)/Season 1/S01E01.mkv"), "fresh")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Now = func() time.Time { return info.ModTime().Add(time.Second) }
	var deferred []string
	var eligibleAt time.Time
	s.scanChanges(root, scanner.Changes{
		Paths: []string{"Show (2020)/Season 1/S01E01.mkv"},
		OnDeferred: func(path string, eligible time.Time) {
			deferred = append(deferred, path)
			eligibleAt = eligible
		},
	})
	status := s.Status()
	if status.Deferred != 1 || status.Imported != 0 || !reflect.DeepEqual(deferred, []string{"Show (2020)/Season 1/S01E01.mkv"}) {
		t.Fatalf("deferred=%q %+v", deferred, status.Counts)
	}
	// Eligible ChangeQuietPeriod after the later of mtime and ctime.
	if eligibleAt.Before(info.ModTime().Add(scanner.ChangeQuietPeriod)) || eligibleAt.After(time.Now().Add(scanner.ChangeQuietPeriod)) {
		t.Fatalf("eligible at %v", eligibleAt)
	}
	if status.Issues[0].Reason != scanner.ReasonFileDeferredWatched {
		t.Fatalf("issue = %+v", status.Issues)
	}
}
