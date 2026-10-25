package watch

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
)

const (
	// pollInterval spaces the directory stats of a polled library. Each poll
	// costs one stat per directory, never per file, so a NAS can still spin
	// its disks down between polls.
	pollInterval = 3 * time.Minute
	// pollSettleInterval re-checks new files sooner while they settle.
	pollSettleInterval = 30 * time.Second
	// recheckInterval spaces the checks for a library that went offline.
	recheckInterval = 30 * time.Second
)

type polledDir struct {
	id      identity
	mtime   int64
	entries map[string]bool // name -> is a directory
}

type polledChange struct {
	rel  string
	kind changeKind
}

// pollReport carries what one poll found back to the session.
type pollReport struct {
	library    *library
	generation int
	changes    []polledChange
	offline    bool
	online     bool
}

// poller watches one library by polling, for filesystems inotify cannot see
// into: network shares and FUSE, and every library on macOS. It snapshots
// each directory's modification time and entry names; a directory whose
// mtime moved is listed again and diffed. It owns every filesystem access for
// its library, so a share that hangs stalls only this goroutine. New files
// are reported once they have been unchanged for the full FileQuietPeriod,
// since a poll cannot see a writer close them.
//
// Limitation: rewriting a file in place changes no directory, so it is only
// picked up by a library scan.
type poller struct {
	library    *library
	generation int
	root       string
	allow      func(root, path string) bool
	extensions map[string]bool
	timing     pollTiming
	now        func() time.Time

	layout   layout
	dirs     map[string]*polledDir
	settling map[string]scanner.FileFingerprint
	offline  bool
}

type pollTiming struct {
	interval time.Duration
	settle   time.Duration
	recheck  time.Duration
	quiet    time.Duration
}

var defaultPollTiming = pollTiming{interval: pollInterval, settle: pollSettleInterval, recheck: recheckInterval, quiet: scanner.FileQuietPeriod}

func (p *poller) run(ctx context.Context, out chan<- pollReport) {
	p.offline = true
	delay := time.Duration(0)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		report := p.poll()
		hasNews := report.offline || report.online || len(report.changes) > 0
		if hasNews {
			report.library, report.generation = p.library, p.generation
			select {
			case out <- report:
			case <-ctx.Done():
				return
			}
		}
		timer.Reset(p.nextDelay())
	}
}

func (p *poller) nextDelay() time.Duration {
	if p.offline {
		return p.timing.recheck
	}
	if len(p.settling) > 0 {
		return min(p.timing.interval, p.timing.settle)
	}
	return p.timing.interval
}

func (p *poller) poll() pollReport {
	if p.offline {
		return p.arm()
	}
	present, changed := p.layout.check()
	if !present {
		return p.goOffline()
	}
	if changed {
		p.offline = true
		return p.arm()
	}
	var report pollReport
	paths := make([]string, 0, len(p.dirs))
	for path := range p.dirs {
		paths = append(paths, path)
	}
	for _, path := range paths {
		lost := p.diff(path, &report)
		if lost {
			return p.goOffline()
		}
	}
	p.settle(&report)
	return report
}

// arm takes the snapshot once the library is present; the session then
// catches up with a walk of the whole library.
func (p *poller) arm() pollReport {
	present, _ := p.layout.check()
	if !present {
		return pollReport{}
	}
	recorded, err := recordRoot(p.root)
	if err != nil {
		return pollReport{}
	}
	p.layout = recorded
	p.dirs = make(map[string]*polledDir)
	p.settling = make(map[string]scanner.FileFingerprint)
	err = p.snapshot(p.root, nil)
	if err != nil {
		return pollReport{}
	}
	p.offline = false
	return pollReport{online: true}
}

func (p *poller) goOffline() pollReport {
	p.offline = true
	p.dirs, p.settling = nil, nil
	return pollReport{offline: true}
}

// snapshot records directory and everything beneath it that the scanner
// would walk; with files set it also collects the media files found.
func (p *poller) snapshot(directory string, files *[]string) error {
	devices := make(map[string]string)
	return filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == directory {
				return err
			}
			return nil
		}
		if !entry.IsDir() {
			isMedia := files != nil && p.extensions[helpers.FileExtension(path)]
			if isMedia {
				*files = append(*files, path)
			}
			return nil
		}
		allowed := path == p.root || p.allow == nil || p.allow(p.root, path)
		if !allowed {
			return fs.SkipDir
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil
		}
		id := identityOf(info)
		devices[path] = id.device
		parent, known := devices[filepath.Dir(path)]
		isMount := path != p.root && known && parent != id.device
		if isMount {
			p.layout.mounts[path] = true
		}
		listing, err := os.ReadDir(path)
		if err != nil {
			return nil
		}
		p.dirs[path] = &polledDir{id: id, mtime: info.ModTime().UnixNano(), entries: p.entries(path, listing)}
		return nil
	})
}

func (p *poller) entries(directory string, listing []fs.DirEntry) map[string]bool {
	entries := make(map[string]bool, len(listing))
	for _, entry := range listing {
		path := filepath.Join(directory, entry.Name())
		if entry.IsDir() {
			allowed := p.allow == nil || p.allow(p.root, path)
			if allowed {
				entries[entry.Name()] = true
			}
			continue
		}
		if p.extensions[helpers.FileExtension(path)] {
			entries[entry.Name()] = false
		}
	}
	return entries
}

// diff lists a directory again when its mtime or identity moved. Removals
// are only reported from a successful listing of the same directory: a
// replaced directory, such as an empty mount point, proves nothing. It
// reports whether the share itself became unavailable.
func (p *poller) diff(directory string, report *pollReport) bool {
	known, ok := p.dirs[directory]
	if !ok {
		return false
	}
	info, err := os.Stat(directory)
	if err != nil {
		return unavailable(err)
	}
	id := identityOf(info)
	mtime := info.ModTime().UnixNano()
	unchanged := id == known.id && mtime == known.mtime
	if unchanged {
		return false
	}
	listing, err := os.ReadDir(directory)
	if err != nil {
		return unavailable(err)
	}
	current := p.entries(directory, listing)
	replaced := id != known.id
	for name, isDir := range current {
		_, existed := known.entries[name]
		if existed {
			continue
		}
		path := filepath.Join(directory, name)
		if isDir {
			var files []string
			_ = p.snapshot(path, &files)
			for _, file := range files {
				p.observe(file)
			}
			continue
		}
		p.observe(path)
	}
	for name, isDir := range known.entries {
		_, exists := current[name]
		if exists {
			continue
		}
		path := filepath.Join(directory, name)
		if isDir {
			p.forget(path)
		}
		delete(p.settling, path)
		if !replaced {
			rel, ok := relative(p.root, path)
			if ok {
				report.changes = append(report.changes, polledChange{rel: rel, kind: changeRemoved})
			}
		}
	}
	known.id, known.mtime, known.entries = id, mtime, current
	return false
}

func (p *poller) observe(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	p.settling[path] = scanner.FingerprintOf(info)
}

func (p *poller) forget(directory string) {
	for path := range p.dirs {
		if within(directory, path) {
			delete(p.dirs, path)
		}
	}
	for path := range p.settling {
		if within(directory, path) {
			delete(p.settling, path)
		}
	}
}

// settle reports each new file whose metadata held still since the last poll
// and is past the quiet period.
func (p *poller) settle(report *pollReport) {
	now := p.now()
	for path, last := range p.settling {
		info, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			delete(p.settling, path)
			continue
		}
		if err != nil {
			continue
		}
		observed := scanner.FingerprintOf(info)
		stable := observed.Size == last.Size && observed.MtimeNS == last.MtimeNS && observed.CtimeNS == last.CtimeNS
		quiet := !now.Before(observed.ChangedAt().Add(p.timing.quiet))
		if !stable || !quiet {
			p.settling[path] = observed
			continue
		}
		delete(p.settling, path)
		rel, ok := relative(p.root, path)
		if ok {
			report.changes = append(report.changes, polledChange{rel: rel, kind: changeSettled})
		}
	}
}
