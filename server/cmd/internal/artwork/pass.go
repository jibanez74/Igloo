package artwork

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
)

// pass brings the copies in line with the setting: one for every image while
// it is on, none while it is off.
func (s *Syncer) pass(ctx context.Context) {
	staticDir := s.staticDir()
	enabled := s.enabled()
	if enabled {
		s.keepCopies(ctx, staticDir)
		return
	}
	s.removeCopies(ctx, staticDir)
}

// removeCopies points every column back at its source, then deletes the
// copies. A failed revert keeps the files the columns still point at.
func (s *Syncer) removeCopies(ctx context.Context, staticDir string) {
	isClean := staticDir == s.cleanedDir
	if isClean {
		return
	}
	clear(s.failures)
	s.lastStaticDir = ""

	var reverted int64
	err := s.tx.Run(ctx, func(q *database.Queries) error {
		for _, c := range columns {
			n, err := c.revert(q, ctx)
			if err != nil {
				return fmt.Errorf("revert %s artwork: %w", c.label, err)
			}
			reverted += n
		}
		return nil
	}, nil)
	if err != nil {
		s.logFailure(ctx, "failed to point artwork back at its providers", err)
		return
	}

	// A folder that overlaps a library was never written, so there is
	// nothing to remove from it.
	removed, complete := s.prune(ctx, staticDir, nil, s.overlappingDirs(staticDir))
	if complete && ctx.Err() == nil {
		s.cleanedDir = staticDir
	}
	changed := reverted > 0 || removed > 0
	if changed {
		s.logger.Info("removed downloaded artwork", "reverted", reverted, "removed_files", removed, "static_dir", staticDir)
	}
}

// target is where an image's copy belongs and what it is made from.
type target struct {
	folder  string // slash-separated, under the static directory
	url     string
	version string
}

// target reports where img's copy belongs; ok is false for an image that
// stays remote.
func (s *Syncer) target(img image) (target, bool) {
	hasSource := img.key != "" && img.source.Valid
	if !hasSource {
		return target{}, false
	}

	sourceURL := img.column.sourceURL(s.tmdbImageBaseURL(), img.source.String)
	if sourceURL == "" {
		return target{}, false
	}

	return target{
		folder:  img.column.dir + "/" + img.key,
		url:     sourceURL,
		version: version(img.column.size, img.source.String),
	}, true
}

// download is one copy to fetch and the images that will point at it.
type download struct {
	target
	stem   string
	label  string
	images []image
}

// keptCopy is a copy a column still points at.
type keptCopy struct {
	rel     string
	version string
}

// plan is what a pass does, decided from the listed images.
type plan struct {
	reverts   []update
	reuses    []update
	downloads []*download
	// kept holds the paths of the copies the columns point at, for prune.
	kept map[string]bool
	// localSeen counts the images pointing at a copy, localMissing those
	// whose file is gone.
	localSeen    int
	localMissing int
	remembered   int
}

// update writes value to an image's column; rel is the copy it points at,
// empty for a revert.
type update struct {
	img   image
	value sql.NullString
	rel   string
}

// keepCopies makes sure every image has a current copy and that the columns
// point at it, then removes the copies nothing points at.
func (s *Syncer) keepCopies(ctx context.Context, staticDir string) {
	s.cleanedDir = ""
	started := s.now()
	skipped := s.overlappingDirs(staticDir)
	for dir, library := range skipped {
		s.logger.Error("not managing artwork in a folder that overlaps a media library; choose another static directory", "folder", filepath.Join(staticDir, dir), "library", library)
	}

	images, err := s.listImages(ctx)
	if err != nil {
		s.logFailure(ctx, "failed to list library artwork", err)
		return
	}

	p := s.plan(staticDir, images, skipped)
	// A static directory that held copies on the last pass and holds none now
	// is most likely an unmounted disk; healing would download everything
	// again beneath the mount point.
	allMissing := p.localSeen > 0 && p.localMissing == p.localSeen
	sameDir := staticDir == s.lastStaticDir
	if allMissing && sameDir {
		s.logger.Warn("skipping artwork pass: no downloaded artwork is left in the static directory; is it mounted?", "static_dir", staticDir, "copies", p.localSeen)
		return
	}

	applyFailed := false
	reverted, err := s.apply(ctx, p.reverts, p.kept)
	if err != nil {
		applyFailed = true
		s.logFailure(ctx, "failed to point stale artwork back at its providers", err)
	}
	reused, err := s.apply(ctx, p.reuses, p.kept)
	if err != nil {
		applyFailed = true
		s.logFailure(ctx, "failed to point artwork at existing copies", err)
	}

	downloaded, failed, err := s.downloadAll(ctx, staticDir, p)
	if err != nil {
		applyFailed = true
		s.logFailure(ctx, "failed to point artwork at downloaded copies", err)
	}

	// A copy is only removed once the whole catalog was read, every write
	// landed, and the setting is still on; otherwise a copy a column points
	// at could be taken for an orphan.
	removed := 0
	canPrune := !applyFailed && ctx.Err() == nil && s.enabled()
	if canPrune {
		removed, _ = s.prune(ctx, staticDir, p.kept, skipped)
		s.lastStaticDir = staticDir
	}

	changed := downloaded > 0 || reused > 0 || failed > 0 || reverted > 0 || removed > 0
	summary := []any{
		"downloaded", downloaded, "reused", reused, "failed", failed,
		"skipped_after_failure", p.remembered, "reverted", reverted, "removed_files", removed,
		"duration", s.now().Sub(started).Round(time.Millisecond),
	}
	if changed {
		s.logger.Info("artwork pass finished", summary...)
		return
	}
	s.logger.Debug("artwork pass finished", summary...)
}

func (s *Syncer) listImages(ctx context.Context) ([]image, error) {
	var images []image
	for _, t := range tables {
		var afterID int64
		for {
			page, lastID, rows, err := t.list(ctx, s.queries, afterID)
			if err != nil {
				return nil, fmt.Errorf("list %s artwork: %w", t.name, err)
			}
			images = append(images, page...)
			isLastPage := rows < pageSize
			if isLastPage {
				break
			}
			afterID = lastID
		}
	}

	return images, nil
}

// plan sorts the images into copies to keep, columns to point back at their
// source, copies to reuse and copies to download.
func (s *Syncer) plan(staticDir string, images []image, skipped map[string]string) plan {
	p := plan{kept: make(map[string]bool)}
	keptStems := make(map[string]keptCopy)
	type candidate struct {
		img  image
		want target
	}
	var candidates []candidate

	for _, img := range images {
		_, isSkipped := skipped[img.column.dir]
		if isSkipped {
			continue
		}

		want, ok := s.target(img)
		rel, ver, isLocal := parseLocalURL(img.value.String)
		if isLocal {
			p.localSeen++
			exists := fileExists(staticDir, rel)
			if !exists {
				p.localMissing++
			}

			stemPath := strings.TrimSuffix(rel, path.Ext(rel))
			isCurrent := ok && exists && ver == want.version &&
				stemPath == want.folder+"/"+img.stem && helpers.IsImageExtension(path.Ext(rel))
			if isCurrent {
				p.kept[rel] = true
				keptStems[stemPath] = keptCopy{rel: rel, version: ver}
				continue
			}
			p.reverts = append(p.reverts, update{img: img, value: img.source})
		}

		if ok {
			candidates = append(candidates, candidate{img: img, want: want})
		}
	}

	// Rows are listed by id, so the first image of a stem is the oldest row's,
	// and its source wins a stem that two sources share.
	jobs := make(map[string]*download)
	now := s.now()
	for _, c := range candidates {
		img, want := c.img, c.want
		stemPath := want.folder + "/" + img.stem

		kept, isKept := keptStems[stemPath]
		if isKept {
			if kept.version == want.version {
				value := sql.NullString{String: localURL(kept.rel, kept.version), Valid: true}
				p.reuses = append(p.reuses, update{img: img, value: value, rel: kept.rel})
			}
			continue
		}

		job, isQueued := jobs[stemPath]
		if isQueued {
			if job.version == want.version {
				job.images = append(job.images, img)
			}
			continue
		}

		failedAt, failedBefore := s.failures[stemPath+"|"+want.version]
		isRemembered := failedBefore && now.Sub(failedAt) < failureRetryAfter
		if isRemembered {
			p.remembered++
			continue
		}

		job = &download{target: want, stem: img.stem, label: img.column.label, images: []image{img}}
		jobs[stemPath] = job
		p.downloads = append(p.downloads, job)
	}

	return p
}

// apply writes the updates in transactions of applyBatchSize, records in kept
// the copies a column now points at, and reports how many columns changed.
// An update whose row has a new source by now is skipped: the source wins.
func (s *Syncer) apply(ctx context.Context, updates []update, kept map[string]bool) (int, error) {
	applied := 0
	for start := 0; start < len(updates); start += applyBatchSize {
		batch := updates[start:min(start+applyBatchSize, len(updates))]
		batchApplied := 0
		var landed []string
		err := s.tx.Run(ctx, func(q *database.Queries) error {
			for _, u := range batch {
				n, err := u.img.column.set(ctx, q, u.img.rowID, u.value, u.img.source)
				if err != nil {
					return fmt.Errorf("update %s artwork: %w", u.img.column.label, err)
				}
				if n > 0 {
					batchApplied++
					if u.rel != "" {
						landed = append(landed, u.rel)
					}
				}
			}
			return nil
		}, nil)
		if err != nil {
			return applied, err
		}
		applied += batchApplied
		for _, rel := range landed {
			kept[rel] = true
		}
	}

	return applied, nil
}

type downloadResult struct {
	job  *download
	path string
	err  error
}

// downloadAll fetches the planned copies and points their images at them,
// stopping when the setting is turned off or the provider keeps failing. It
// reports the copies saved and the downloads that failed; the error is a
// failed column write.
func (s *Syncer) downloadAll(ctx context.Context, staticDir string, p plan) (int, int, error) {
	if len(p.downloads) == 0 {
		return 0, 0, nil
	}
	s.logger.Info("downloading artwork", "images", len(p.downloads), "static_dir", staticDir)

	breaker := scanner.ProviderBreaker{Classify: classifyDownloadError}
	var pending []update
	var applyErr error
	downloaded, failed := 0, 0
	lastProgress := s.now()

	flush := func(force bool) {
		isDue := len(pending) >= applyBatchSize || (force && len(pending) > 0)
		if !isDue || applyErr != nil {
			return
		}
		_, applyErr = s.apply(ctx, pending, p.kept)
		pending = pending[:0]
	}

	scanner.RunWorkers(ctx, downloadWorkers, p.downloads,
		func() bool { return !breaker.Stopped && applyErr == nil && s.enabled() },
		func(ctx context.Context, job *download) downloadResult {
			dir := filepath.Join(staticDir, filepath.FromSlash(job.folder))
			saved, err := helpers.DownloadImage(ctx, s.client, job.url, dir, job.stem)
			return downloadResult{job: job, path: saved, err: err}
		},
		func(*download) {},
		func(result downloadResult) {
			job := result.job
			if result.err != nil {
				canceled := ctx.Err() != nil
				if canceled {
					return
				}
				failed++
				breaker.Observe(result.err)
				isPermanent := isPermanentDownloadFailure(result.err)
				if isPermanent {
					s.failures[job.folder+"/"+job.stem+"|"+job.version] = s.now()
				}
				s.logger.Warn("failed to download artwork", "image", job.label, "url", job.url, "error", result.err)
				return
			}

			breaker.Observe(nil)
			downloaded++
			rel := job.folder + "/" + filepath.Base(result.path)
			value := sql.NullString{String: localURL(rel, job.version), Valid: true}
			for _, img := range job.images {
				pending = append(pending, update{img: img, value: value, rel: rel})
			}
			flush(false)

			isProgressDue := s.now().Sub(lastProgress) >= progressLogInterval
			if isProgressDue {
				lastProgress = s.now()
				s.logger.Info("artwork download progress", "downloaded", downloaded, "failed", failed, "total", len(p.downloads))
			}
		},
	)
	flush(true)

	if breaker.Stopped {
		s.logger.Warn("stopped downloading artwork after repeated failures; the rest is retried on the next pass", "downloaded", downloaded, "failed", failed)
	}

	return downloaded, failed, applyErr
}

// classifyDownloadError marks transport failures, rate limits and server
// errors as transient, so a run of them stops the pass; a success or any
// other failure ends the run. Nothing is fatal.
func classifyDownloadError(err error) (fatal, transient bool) {
	if err == nil {
		return false, false
	}

	var statusErr *helpers.ImageStatusError
	isStatus := errors.As(err, &statusErr)
	if isStatus {
		isServerSide := statusErr.StatusCode == http.StatusTooManyRequests || statusErr.StatusCode >= http.StatusInternalServerError
		return false, isServerSide
	}

	isContent := errors.Is(err, helpers.ErrUnsupportedImageType) || errors.Is(err, helpers.ErrImageTooLarge)
	return false, !isContent
}

// isPermanentDownloadFailure reports a failure that asking again soon would
// repeat: the provider refused the image, or sent something Igloo does not
// store.
func isPermanentDownloadFailure(err error) bool {
	var statusErr *helpers.ImageStatusError
	isStatus := errors.As(err, &statusErr)
	if isStatus {
		isClientSide := statusErr.StatusCode >= http.StatusBadRequest && statusErr.StatusCode < http.StatusInternalServerError
		return isClientSide && statusErr.StatusCode != http.StatusTooManyRequests
	}

	return errors.Is(err, helpers.ErrUnsupportedImageType) || errors.Is(err, helpers.ErrImageTooLarge)
}

// prune removes the managed files that are not in kept (every one when kept
// is nil) from the managed folders not in skipped, then the folders left
// empty. It reports how many files it removed, and whether it got through
// every folder without a filesystem error.
func (s *Syncer) prune(ctx context.Context, staticDir string, kept map[string]bool, skipped map[string]string) (int, bool) {
	removed := 0
	complete := true
	for _, dir := range managedDirs() {
		_, isSkipped := skipped[dir]
		if isSkipped {
			continue
		}

		root := filepath.Join(staticDir, dir)
		keys, err := os.ReadDir(root)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			s.logger.Error("failed to read artwork folder", "path", root, "error", err)
			complete = false
			continue
		}

		for _, key := range keys {
			if ctx.Err() != nil {
				return removed, false
			}
			isKeyFolder := key.IsDir() && helpers.IsPlainPathSegment(key.Name(), "")
			if !isKeyFolder {
				continue
			}
			n, folderComplete := s.pruneFolder(dir, filepath.Join(root, key.Name()), key.Name(), kept)
			removed += n
			complete = complete && folderComplete
		}

		if kept == nil {
			// Fails, as it should, while anything else is in the folder.
			_ = os.Remove(root)
		}
	}

	return removed, complete
}

func (s *Syncer) pruneFolder(dir, folder, key string, kept map[string]bool) (int, bool) {
	files, err := os.ReadDir(folder)
	if err != nil {
		s.logger.Error("failed to read artwork folder", "path", folder, "error", err)
		return 0, false
	}

	removed := 0
	complete := true
	for _, file := range files {
		isOurs := file.Type().IsRegular() && isManagedFile(dir, file.Name())
		if !isOurs {
			continue
		}
		if kept[dir+"/"+key+"/"+file.Name()] {
			continue
		}

		err = os.Remove(filepath.Join(folder, file.Name()))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			s.logger.Error("failed to remove artwork", "path", filepath.Join(folder, file.Name()), "error", err)
			complete = false
			continue
		}
		removed++
	}

	// Fails, as it should, while anything is left in the folder.
	_ = os.Remove(folder)

	return removed, complete
}

// overlappingDirs reports the managed folders that are, contain or lie inside
// a media library, mapped to that library. The syncer never writes or deletes
// there: a library's own files could be taken for copies. Symlinks are
// resolved on both sides, since a managed folder that links into a library is
// that library; a path that cannot be resolved counts as overlapping.
func (s *Syncer) overlappingDirs(staticDir string) map[string]string {
	overlapping := make(map[string]string)
	for _, dir := range managedDirs() {
		managed, err := resolvedPath(filepath.Join(staticDir, dir))
		if err != nil {
			overlapping[dir] = err.Error()
			continue
		}

		for _, library := range s.libraryDirs() {
			libraryPath, err := resolvedPath(library)
			if err != nil {
				overlapping[dir] = library
				continue
			}
			if pathsOverlap(managed, libraryPath) {
				overlapping[dir] = library
			}
		}
	}

	return overlapping
}

// resolvedPath is the absolute path p leads to with every symlink in it
// resolved. A path that does not exist yet resolves through its nearest
// existing ancestor, since a folder created there lands wherever that
// ancestor leads.
func resolvedPath(p string) (string, error) {
	current, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}

	var missing []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Join(append([]string{resolved}, missing...)...), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append([]string{filepath.Base(current)}, missing...)
		current = parent
	}
}

func pathsOverlap(a, b string) bool {
	separator := string(filepath.Separator)
	return a == b || strings.HasPrefix(a, b+separator) || strings.HasPrefix(b, a+separator)
}

func fileExists(staticDir, rel string) bool {
	info, err := os.Stat(filepath.Join(staticDir, filepath.FromSlash(rel)))
	return err == nil && info.Mode().IsRegular()
}

// logFailure logs err unless shutdown caused it.
func (s *Syncer) logFailure(ctx context.Context, message string, err error) {
	if ctx.Err() != nil {
		return
	}
	s.logger.Error(message, "error", err)
}
