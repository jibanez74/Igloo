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

// Carry copies into a run of changes' report the issues the last run left at
// files outside paths, which this run does not look at again; those at or
// beneath paths are its to raise or settle. A walk of the whole library
// looks at every file, so it carries nothing. Issues that name no file, a
// run's own failures and provider lookups, stay with the run that raised
// them.
func (o *Outstanding) Carry(report *Report, changes *Changes, paths []string) {
	scoped := changes.Scoped()
	if !scoped {
		return
	}
	for key, issue := range o.issues {
		_, path, _ := strings.Cut(key, ":")
		absolute := filepath.IsAbs(path)
		if !absolute {
			continue
		}
		covered := false
		for _, changed := range paths {
			covered = covered || PathWithin(changed, path)
		}
		if !covered {
			report.issues[key] = issue
			report.markIssueChanged(key)
		}
	}
}

// Keep remembers a finished run's issues for the next run.
func (o *Outstanding) Keep(report *Report) {
	o.issues = make(map[string]Issue, len(report.issues))
	for key, issue := range report.issues {
		o.issues[key] = issue
	}
}
