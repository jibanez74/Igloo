package music

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
)

// scanChanges runs one run of watcher-detected changes synchronously.
func (s *Scanner) scanChanges(directory string, changes scanner.Changes) {
	scope := scanner.ScopeChanges
	if changes.CatchUp {
		scope = scanner.ScopeLibrary
	}
	s.beginReport(scope)()
	s.runMusicScan(directory, &changes)
}

func albumQueryTitles(stub *musicScannerSpotifyStub) []string {
	titles := make([]string, 0, len(stub.albumQueries))
	for _, query := range stub.albumQueries {
		titles = append(titles, query.Title)
	}
	slices.Sort(titles)
	return slices.Compact(titles)
}

func TestMusicChangesRunKeepsEnrichmentToTheTouchedAlbum(t *testing.T) {
	s := setupMusicScanner(t)
	root := t.TempDir()
	tags := func(title, artist, album, track string) *ffprobe.FfprobeResult {
		return testMusicMetadataWithTags(ffprobe.FormatTags{Title: title, Artist: artist, Album: album, Track: track})
	}
	a1 := scannertest.WriteFile(t, filepath.Join(root, "Artist A", "Album A", "01.flac"), "a1")
	a2 := filepath.Join(root, "Artist A", "Album A", "02.flac")
	b1 := scannertest.WriteFile(t, filepath.Join(root, "Artist B", "Album B", "01.flac"), "b1")
	s.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		a1: tags("One", "Artist A", "Album A", "1"),
		a2: tags("Two", "Artist A", "Album A", "2"),
		b1: tags("Uno", "Artist B", "Album B", "1"),
	})
	// Every lookup misses the way a 404 does, so both albums stay retry
	// candidates and every scan's retry pass would search them again.
	stub := &musicScannerSpotifyStub{artistErr: entityLookupFailure, albumErr: entityLookupFailure}
	s.spotify = stub
	s.scan(root)
	if s.Status().Imported != 2 {
		t.Fatalf("library scan: %+v", s.Status().Counts)
	}

	stub.albumQueries, stub.albumCalls, stub.artistCalls = nil, 0, 0
	scannertest.WriteFile(t, a2, "a2")
	s.scanChanges(root, scanner.Changes{Paths: []string{"Artist A/Album A/02.flac"}})
	status := s.Status()
	if status.Scope != scanner.ScopeChanges || status.Total != 1 || status.Imported != 1 {
		t.Fatalf("changes run: scope=%s %+v", status.Scope, status.Counts)
	}
	titles := albumQueryTitles(stub)
	if !slices.Equal(titles, []string{"Album A"}) {
		t.Fatalf("a run of changes searched %q; only the touched album may be searched", titles)
	}

	// A library scan still retries every pending album.
	stub.albumQueries = nil
	s.scan(root)
	titles = albumQueryTitles(stub)
	if !slices.Equal(titles, []string{"Album A", "Album B"}) {
		t.Fatalf("library scan searched %q", titles)
	}
}

func TestMusicChangesRunRemovesOnlyTheReportedTracks(t *testing.T) {
	s := setupMusicScanner(t)
	root := t.TempDir()
	a1 := scannertest.WriteFile(t, filepath.Join(root, "Album A", "01.flac"), "a1")
	b1 := scannertest.WriteFile(t, filepath.Join(root, "Album B", "01.flac"), "b1")
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.scan(root)
	for _, path := range []string{a1, b1} {
		err := os.Remove(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	s.scanChanges(root, scanner.Changes{Paths: []string{"Album A"}})
	if s.Status().Deleted != 1 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks") != 1 {
		t.Fatalf("deleted=%d", s.Status().Deleted)
	}

	s.scanChanges(root, scanner.Changes{CatchUp: true})
	status := s.Status()
	if status.Deleted != 0 || status.IssueCount != 1 || status.Issues[0].Reason != scanner.ReasonCleanupSkippedEmpty {
		t.Fatalf("catch-up over an empty root: %+v", status)
	}
}
