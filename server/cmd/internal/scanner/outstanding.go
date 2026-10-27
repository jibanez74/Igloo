package scanner

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Outstanding keeps the file issues a scanner's last run left. Each run
// publishes its own report, and a run of changes looks only at the paths the
// watcher saw change, so without them a library scan's failed and deferred
// files would vanish from the status at the watcher's next run while still
// failing. Runs are sequential, so it needs no lock.
type Outstanding struct {
	issues map[string]Issue
}

// carriedPhases are the phases whose issues stay true until a run looks at
// their path again: a file that failed or was deferred, and a disk kept
// because it was not mounted. Discovery issues are left out, since a directory
// fixed since is not among the paths of the run that imports its files, and
// so are enrichment issues, which a run of changes may settle without naming
// their key, as it enriches a show it touched.
var carriedPhases = map[ScanPhase]bool{PhaseLocal: true, PhaseCleanup: true}

// Carry lists in a run of changes' report the issues the last run left at
// paths outside the run's, which it does not look at again; those at or
// beneath its paths are its to raise or settle, unless it cannot read them
// (see Unread). A walk of the whole library
// looks at every file, so it carries nothing. Carried issues do not make the
// run's state completed-with-issues, and its own are published first.
func (o *Outstanding) Carry(report *Report, changes *Changes, paths []string) {
	scoped := changes.Scoped()
	if !scoped {
		return
	}
	changed := make(map[string]bool, len(paths))
	for _, path := range paths {
		changed[path] = true
	}
	for key, issue := range o.issues {
		path, stays := lasting(key)
		if !stays {
			continue
		}
		lookedAt := covered(changed, path)
		if lookedAt {
			continue
		}
		report.carry(key, issue)
	}
}

// Unread carries back into a run of changes, once its walk is over, the
// issues the last run left at or beneath the paths this run could not read,
// which it reported as its own discovery issues, for files it did not see and
// that still exist: it covered them without looking at them, so they still
// stand. A file the run saw was looked at, and one gone since, as a broken
// symlink's, is the cleanup's to remove.
func (o *Outstanding) Unread(report *Report, changes *Changes, seen func(path string) bool) {
	scoped := changes.Scoped()
	if !scoped {
		return
	}
	unread := make(map[string]bool)
	for key := range report.issues {
		phase, path, _ := strings.Cut(key, ":")
		discovery := ScanPhase(phase) == PhaseDiscovery && path != ""
		if discovery {
			unread[path] = true
		}
	}
	if len(unread) == 0 {
		return
	}
	for key, issue := range o.issues {
		path, stays := lasting(key)
		_, own := report.issues[key]
		_, carried := report.carried[key]
		if !stays || own || carried {
			continue
		}
		beneath := covered(unread, path)
		if !beneath || seen(path) {
			continue
		}
		_, err := os.Stat(path)
		gone := errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
		if !gone {
			report.carry(key, issue)
		}
	}
}

// lasting returns the path of an issue key when the issue stays true until a
// run looks at that path again.
func lasting(key string) (string, bool) {
	phase, path, _ := strings.Cut(key, ":")
	return path, path != "" && carriedPhases[ScanPhase(phase)]
}

// covered reports whether path is one of changed or lies beneath one.
func covered(changed map[string]bool, path string) bool {
	for {
		if changed[path] {
			return true
		}
		parent := filepath.Dir(path)
		if parent == path {
			return false
		}
		path = parent
	}
}

// Keep remembers a finished run's issues, its own and those it carried, for
// the next run.
func (o *Outstanding) Keep(report *Report) {
	o.issues = make(map[string]Issue, len(report.issues)+len(report.carried))
	maps.Copy(o.issues, report.carried)
	maps.Copy(o.issues, report.issues)
}
