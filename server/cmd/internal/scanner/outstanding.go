package scanner

import (
	"path/filepath"
	"strings"
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
// beneath its paths are its to raise or settle. A walk of the whole library
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
		phase, path, _ := strings.Cut(key, ":")
		absolute := filepath.IsAbs(path)
		if !absolute || !carriedPhases[ScanPhase(phase)] || covered(changed, path) {
			continue
		}
		report.issues[key] = issue
		report.carried[key] = true
		report.markIssueChanged(key)
	}
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

// Keep remembers a finished run's issues for the next run.
func (o *Outstanding) Keep(report *Report) {
	o.issues = make(map[string]Issue, len(report.issues))
	for key, issue := range report.issues {
		o.issues[key] = issue
	}
}
