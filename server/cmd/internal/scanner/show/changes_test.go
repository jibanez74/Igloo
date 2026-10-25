package show

import (
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
)

func episodeID(t *testing.T, s *Scanner, season, episode int) int64 {
	t.Helper()
	var id int64
	err := s.DB.QueryRow(`SELECT e.id FROM show_episodes e JOIN show_seasons s ON s.id = e.season_id
		WHERE s.season_number = ? AND e.episode_number = ?`, season, episode).Scan(&id)
	if err != nil {
		t.Fatalf("episode S%02dE%02d: %v", season, episode, err)
	}
	return id
}

func TestShowChangesRunAppliesOnlyTheChangedPaths(t *testing.T) {
	s, probe, root := setupScanner(t)
	e1 := scannertest.WriteFile(t, filepath.Join(root, "Show (2020)/Season 1/S01E01.mkv"), "e1")
	scannertest.WriteFile(t, filepath.Join(root, "Show (2020)/Season 1/S01E02.mkv"), "e2")
	other := scannertest.WriteFile(t, filepath.Join(root, "Other (2021)/Season 1/S01E01.mkv"), "o1")
	scanOK(t, s, root)
	pilot := episodeID(t, s, 1, 1)
	if probe.Calls() != 3 {
		t.Fatalf("library scan probes = %d", probe.Calls())
	}

	// Renaming an episode file lands both names in one run: the new name is
	// processed before cleanup, so the episode and its progress survive.
	renamed := filepath.Join(root, "Show (2020)/Season 1/S01E01 - Pilot.mkv")
	err := os.Rename(e1, renamed)
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
	if episodeID(t, s, 1, 1) != pilot {
		t.Fatal("renaming the episode file replaced the episode")
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
