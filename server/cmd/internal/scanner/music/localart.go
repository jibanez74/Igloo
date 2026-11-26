package music

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
)

// PictureExtractor copies the picture an audio file embeds as stream
// streamIndex to destPath; ffmpeg.FFmpegInterface is one.
type PictureExtractor interface {
	ExtractAttachedPicture(ctx context.Context, sourcePath string, streamIndex int, destPath string) error
}

// localCoverDir is where, under the static directory, an album's own art is
// copied: local/albums/<album id>/cover-<hash><ext>. The artwork syncer manages
// other folders, so turning download_images off never touches these.
var localCoverDir = filepath.Join("local", "albums")

// localCoverStem starts the copy's name, cover-<hash of its bytes>; its
// extension follows the type its bytes sniff as.
const localCoverStem = "cover"

// maxLocalCoverSize bounds a copy: scans and booklets can be large, but not
// this large.
const maxLocalCoverSize = 20 << 20

// maxEmbeddedAttempts bounds the tracks a pass extracts a picture from for
// one album: each attempt starts FFmpeg.
const maxEmbeddedAttempts = 3

// coverNames and coverExtensions rank the images a folder may hold, best
// first. Names are matched without case.
var (
	coverNames      = []string{"cover", "folder", "front", "albumart"}
	coverExtensions = []string{".jpg", ".jpeg", ".png", ".webp"}
)

// embeddedPicture picks the picture a track embeds, by stream index: its
// front cover, any other cover, or else the first attached picture. MP3 and
// FLAC name the picture's role in its comment tag.
func embeddedPicture(streams []ffprobe.Stream) sql.NullInt64 {
	best, bestRank := -1, 3
	for _, stream := range streams {
		isPicture := stream.Disposition.AttachedPic == 1
		if !isPicture {
			continue
		}
		comment := strings.ToLower(stream.Tags.Comment)
		rank := 2
		if strings.Contains(comment, "cover") {
			rank = 1
		}
		if comment == "cover (front)" {
			rank = 0
		}
		if rank < bestRank {
			best, bestRank = stream.Index, rank
		}
	}
	if best < 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(best), Valid: true}
}

// artOutcome is what one album's local-art step found.
type artOutcome int

const (
	// artNone: the album has no art of its own.
	artNone artOutcome = iota
	// artFound: the album shows its own art, copied now or before.
	artFound
	// artFailed: the album's art could not be read or copied; a cover
	// copied before stays while its file is still in the static directory.
	artFailed
)

// artSource is one image an album may use: a folder image, or a picture a
// track embeds.
type artSource struct {
	path        string
	stream      int
	embedded    bool
	fingerprint string
}

// errSettingsChanged stops a pass whose static directory or library roots
// changed under it: its overlap check no longer holds. The next pass, which
// takes the albums it did not reach, checks again.
var errSettingsChanged = errors.New("static_dir or library roots changed during the pass")

// localArt is one pass's local-art context.
type localArt struct {
	staticDir string
	// roots are the library roots the pass checked the static directory
	// against.
	roots []string
	// current reports whether the settings still match staticDir and roots.
	current func() bool
	// owners caches each folder's verdict for the album that asked.
	owners map[string]int64
}

// check stops a write or removal once the settings the pass checked changed,
// or once the library roots lead somewhere that overlaps the copies' folder,
// as a root symlink retargeted during the pass can.
func (art *localArt) check() error {
	if !art.current() {
		return errSettingsChanged
	}
	_, overlaps := helpers.OverlappingLibrary(filepath.Join(art.staticDir, "local"), art.roots)
	if overlaps {
		return errSettingsChanged
	}
	return nil
}

// syncAlbumArt points an album's cover_local at a copy of its own art: the
// first image in its folder, then in its disc folders, else a picture a
// track embeds. It reports whether the catalog changed.
func (s *Scanner) syncAlbumArt(ctx context.Context, art *localArt, albumID int64) (artOutcome, bool, error) {
	current, err := s.queries.GetMusicAlbumLocalCover(ctx, albumID)
	if errors.Is(err, sql.ErrNoRows) {
		return artNone, false, nil
	}
	if err != nil {
		return artFailed, false, err
	}
	// A folder that cannot be read, such as one on a library that is not
	// mounted, says nothing about the album's art: the copy stays.
	sources, unreadable, err := s.albumArtSources(ctx, art, albumID)
	if err != nil {
		changed := s.dropMissingCopy(ctx, art, albumID, current)
		return artFailed, changed, err
	}
	if len(sources) == 0 && unreadable != nil {
		changed := s.dropMissingCopy(ctx, art, albumID, current)
		return artFailed, changed, unreadable
	}
	if len(sources) == 0 {
		changed, err := s.clearAlbumArt(ctx, art, albumID, current)
		return artNone, changed, err
	}

	// The fingerprint covers every source up to the one copied, so a copy
	// made from a fallback is kept while the better sources before it stay
	// as they were, and the static directory, so a directory used before
	// holds no copy taken for current.
	hasCopy := current.CoverLocal.Valid && current.LocalCoverFingerprint.Valid && localCoverExists(art.staticDir, current.CoverLocal.String)
	for i := range sources {
		isUnchanged := hasCopy && current.LocalCoverFingerprint.String == sourcesFingerprint(art.staticDir, sources[:i+1])
		if isUnchanged {
			return artFound, false, nil
		}
	}

	var copyErr error
	// A better source that failed for a reason that may pass, such as a
	// read error or an FFmpeg timeout, is tried again on the next pass:
	// the fingerprint of a fallback copied after one never matches.
	retryBetter := false
	embeddedAttempts := 0
	for i, source := range sources {
		if source.embedded {
			if s.pictures == nil || embeddedAttempts == maxEmbeddedAttempts {
				continue
			}
			embeddedAttempts++
		}
		copied, err := s.copyAlbumArt(ctx, art, albumID, source)
		if ctx.Err() != nil {
			return artFailed, false, ctx.Err()
		}
		if errors.Is(err, errSettingsChanged) {
			return artFailed, false, err
		}
		if err != nil {
			s.logger.Debug("failed to copy album art", "album_id", albumID, "source", source.path, "error", err)
			copyErr = errors.Join(copyErr, err)
			isContent := errors.Is(err, helpers.ErrUnsupportedImageType) || errors.Is(err, helpers.ErrImageTooLarge)
			retryBetter = retryBetter || !isContent
			continue
		}
		fingerprint := sourcesFingerprint(art.staticDir, sources[:i+1])
		if retryBetter {
			fingerprint += retryMarker
		}
		changed, err := s.saveAlbumArt(ctx, art, albumID, sql.NullString{String: copied.url, Valid: true}, helpers.NullString(fingerprint))
		if err != nil {
			return artFailed, false, err
		}
		// Only once the column points at the new copy do the earlier ones
		// go: until then one is what the album shows. The folder must still
		// be the static directory's, not a library's.
		rel, _ := filepath.Rel(art.staticDir, copied.dir)
		if art.check() == nil && checkLocalCoverDir(art.staticDir, rel) == nil {
			removeOtherFiles(copied.dir, copied.name)
		}
		return artFound, changed, nil
	}
	if copyErr != nil {
		s.logger.Debug("failed to copy the album's own art", "album_id", albumID, "error", copyErr)
	}
	changed := s.dropMissingCopy(ctx, art, albumID, current)
	return artFailed, changed, nil
}

// dropMissingCopy points an album that failed back at its provider cover
// when its copy is not in the static directory, as after the directory
// moved while the album's folder could not be read: a column pointing at
// nothing would hide the provider cover. The next pass that reads the
// folder copies the art again.
func (s *Scanner) dropMissingCopy(ctx context.Context, art *localArt, albumID int64, current database.GetMusicAlbumLocalCoverRow) bool {
	isMissing := current.CoverLocal.Valid && localCoverMissing(art.staticDir, current.CoverLocal.String)
	if !isMissing {
		return false
	}
	changed, err := s.saveAlbumArt(ctx, art, albumID, sql.NullString{}, sql.NullString{})
	if err != nil {
		s.logger.Debug("failed to drop a missing album art copy", "album_id", albumID, "error", err)
	}
	return changed
}

// albumArtSources lists the images an album may use, best first. A folder
// that cannot be read is an error; unreadable reports cover images that could
// not be looked at, which the album may still have.
func (s *Scanner) albumArtSources(ctx context.Context, art *localArt, albumID int64) (sources []artSource, unreadable, err error) {
	tracks, err := s.queries.ListMusicAlbumArtTracks(ctx, sql.NullInt64{Int64: albumID, Valid: true})
	if err != nil {
		return nil, nil, err
	}

	// Its tracks may lie in more than one folder; the one holding most of
	// them comes first.
	counts := make(map[string]int)
	discs := make(map[string][]string)
	discParents := discFoldersOf(tracks)
	for _, track := range tracks {
		folder, disc := albumFolder(track.FilePath, discParents)
		counts[folder]++
		if disc != "" && !slices.Contains(discs[folder], disc) {
			discs[folder] = append(discs[folder], disc)
		}
	}
	folders := make([]string, 0, len(counts))
	for folder := range counts {
		folders = append(folders, folder)
	}
	slices.SortFunc(folders, func(a, b string) int {
		return cmp.Or(cmp.Compare(counts[b], counts[a]), cmp.Compare(a, b))
	})

	for _, folder := range folders {
		owned, err := art.ownsFolder(ctx, s.queries, albumID, folder)
		if err != nil {
			return nil, nil, err
		}
		if owned {
			images, skipped, err := folderImages(folder)
			if err != nil {
				return nil, nil, err
			}
			unreadable = errors.Join(unreadable, skipped)
			sources = append(sources, images...)
		}
		discFolders := discs[folder]
		slices.Sort(discFolders)
		for _, disc := range discFolders {
			owned, err := art.ownsFolder(ctx, s.queries, albumID, disc)
			if err != nil {
				return nil, nil, err
			}
			if owned {
				images, skipped, err := folderImages(disc)
				if err != nil {
					return nil, nil, err
				}
				unreadable = errors.Join(unreadable, skipped)
				sources = append(sources, images...)
			}
		}
	}

	for _, track := range tracks {
		if !track.PictureStream.Valid {
			continue
		}
		fingerprint := "embedded:" + track.FilePath + ":" + strconv.FormatInt(track.PictureStream.Int64, 10) + ":" +
			strconv.FormatInt(track.Size, 10) + ":" + strconv.FormatInt(track.MtimeNs.Int64, 10)
		sources = append(sources, artSource{path: track.FilePath, stream: int(track.PictureStream.Int64), embedded: true, fingerprint: fingerprint})
	}
	// No art and tracks that are not there, as in a folder that is the
	// mount point of a disk that is not mounted, says nothing about the
	// album's art either.
	if len(sources) == 0 {
		for _, track := range tracks {
			// Followed like the walk follows it: a link to a disk that is
			// away is a track that is not there.
			info, statErr := os.Stat(track.FilePath)
			if statErr == nil && !info.Mode().IsRegular() {
				statErr = fmt.Errorf("%s is not a regular file", track.FilePath)
			}
			if statErr != nil {
				unreadable = errors.Join(unreadable, statErr)
				break
			}
		}
	}
	return sources, unreadable, nil
}

// ownsFolder reports whether every track directly in folder, or in a disc
// folder directly beneath it, belongs to albumID, so that folder's image is
// the album's: a flat folder holding several albums gives none of them its
// image.
func (art *localArt) ownsFolder(ctx context.Context, q *database.Queries, albumID int64, folder string) (bool, error) {
	owner, known := art.owners[folder]
	if known {
		return owner == albumID, nil
	}

	lower, upper := scanner.ChildBounds(folder)
	rows, err := q.ListMusicTrackAlbumsUnder(ctx, database.ListMusicTrackAlbumsUnderParams{Lower: lower, Upper: upper})
	if err != nil {
		return false, err
	}
	// -1 marks a folder no single album owns.
	owner = 0
	for _, row := range rows {
		if !isNearTrack(folder, row.FilePath) {
			continue
		}
		trackOwner := int64(-1)
		if row.AlbumID.Valid {
			trackOwner = row.AlbumID.Int64
		}
		if owner == 0 {
			owner = trackOwner
		}
		if owner != trackOwner {
			owner = -1
		}
	}
	art.owners[folder] = owner
	return owner == albumID, nil
}

// discFoldersOf maps each folder holding the tracks' disc folders to how
// many of them it holds.
func discFoldersOf(tracks []database.ListMusicAlbumArtTracksRow) map[string]int {
	seen := make(map[string]bool)
	parents := make(map[string]int)
	for _, track := range tracks {
		dir := filepath.Dir(track.FilePath)
		if seen[dir] || !isDiscFolder(filepath.Base(dir)) {
			continue
		}
		seen[dir] = true
		parents[filepath.Dir(dir)]++
	}
	return parents
}

// albumFolder is the folder of the album a track at path belongs to: the
// track's own folder, or its parent when the track is in a disc folder and
// the album keeps two or more disc folders there; disc then names the
// track's. A lone disc-like folder, such as an album named "Vol. 4" in its
// artist's folder, is the album's own folder.
func albumFolder(path string, discParents map[string]int) (folder, disc string) {
	dir := filepath.Dir(path)
	parent := filepath.Dir(dir)
	isDisc := isDiscFolder(filepath.Base(dir)) && discParents[parent] >= 2
	if isDisc {
		return parent, dir
	}
	return dir, ""
}

// isNearTrack reports whether the track at path lies directly in folder or in
// a disc folder directly beneath it.
func isNearTrack(folder, path string) bool {
	rel, err := filepath.Rel(folder, path)
	if err != nil {
		return false
	}
	dir := filepath.Dir(rel)
	return dir == "." || (!strings.Contains(dir, string(filepath.Separator)) && isDiscFolder(dir))
}

// folderImages lists the cover images in folder, best first. A symlink is
// followed, as the library's tracks are. A folder that cannot be read is an
// error rather than no art; an image in it that cannot be looked at is left
// out and reported in skipped.
func folderImages(folder string) (sources []artSource, skipped, err error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, nil, err
	}
	type ranked struct {
		source artSource
		rank   int
	}
	var found []ranked
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		ext := filepath.Ext(name)
		nameRank := slices.Index(coverNames, strings.TrimSuffix(name, ext))
		extRank := slices.Index(coverExtensions, ext)
		if nameRank < 0 || extRank < 0 {
			continue
		}
		path := filepath.Join(folder, entry.Name())
		info, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			// A link whose target is away, as on another disk that is
			// not mounted, is unreadable rather than gone.
			_, linkErr := os.Lstat(path)
			if linkErr == nil {
				skipped = errors.Join(skipped, err)
			}
			continue
		}
		if err != nil {
			skipped = errors.Join(skipped, err)
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		// The mode counts too: an image that could not be opened is copied
		// once its permissions let it be.
		fingerprint := "file:" + path + ":" + strconv.FormatInt(info.Size(), 10) + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10) + ":" + info.Mode().String()
		found = append(found, ranked{source: artSource{path: path, fingerprint: fingerprint}, rank: nameRank*len(coverExtensions) + extRank})
	}
	slices.SortStableFunc(found, func(a, b ranked) int { return cmp.Compare(a.rank, b.rank) })
	sources = make([]artSource, 0, len(found))
	for _, f := range found {
		sources = append(sources, f.source)
	}
	return sources, skipped, nil
}

// retryMarker ends the fingerprint of a copy made after a better source
// failed for a reason that may pass, so no pass finds it unchanged.
const retryMarker = ":retry"

// sourcesFingerprint names the sources a copy in staticDir was chosen from,
// the last of them copied.
func sourcesFingerprint(staticDir string, sources []artSource) string {
	digest := sha256.New()
	io.WriteString(digest, staticDir)
	for _, source := range sources {
		io.WriteString(digest, "\x00"+source.fingerprint)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

// localCopy is a copy of an album's art: its folder, file name and URL.
type localCopy struct {
	dir, name, url string
}

// copyAlbumArt copies source into the album's folder, versioned by a hash of
// its bytes. Earlier copies stay until the column points at this one.
func (s *Scanner) copyAlbumArt(ctx context.Context, art *localArt, albumID int64, source artSource) (localCopy, error) {
	rel := filepath.Join(localCoverDir, strconv.FormatInt(albumID, 10))
	dir := filepath.Join(art.staticDir, rel)
	err := art.check()
	if err != nil {
		return localCopy{}, err
	}
	err = checkLocalCoverDir(art.staticDir, rel)
	if err != nil {
		return localCopy{}, err
	}
	_, err = helpers.GetOrCreateDir(dir)
	if err != nil {
		return localCopy{}, err
	}

	path := source.path
	if source.embedded {
		// A name of its own, created here, so nothing already at a known
		// name is written through.
		temp, err := os.CreateTemp(dir, helpers.TEMP_FILE_PREFIX+"embedded-*")
		if err != nil {
			return localCopy{}, err
		}
		extracted := temp.Name()
		defer os.Remove(extracted)
		err = temp.Close()
		if err != nil {
			return localCopy{}, err
		}
		err = s.pictures.ExtractAttachedPicture(ctx, source.path, source.stream, extracted)
		if err != nil {
			return localCopy{}, err
		}
		path = extracted
	}

	content, err := readCapped(path, maxLocalCoverSize)
	if err != nil {
		return localCopy{}, fmt.Errorf("copy %s: %w", filepath.Base(source.path), err)
	}
	// Each version gets a name of its own, so the file behind the URL the
	// column holds now is never overwritten before the column moves on.
	sum := sha256.Sum256(content)
	version := hex.EncodeToString(sum[:4])
	// An extraction can take a while; the settings, or the folders on the
	// way to the copy, may have changed since.
	err = art.check()
	if err != nil {
		return localCopy{}, err
	}
	err = checkLocalCoverDir(art.staticDir, rel)
	if err != nil {
		return localCopy{}, err
	}
	// The same bytes copied before keep their file: it may be the one the
	// column points at, which is never rewritten.
	stem := localCoverStem + "-" + version
	saved := existingCopy(dir, stem)
	if saved == "" {
		saved, err = helpers.SaveImage(dir, stem, bytes.NewReader(content), maxLocalCoverSize)
	}
	if err != nil {
		return localCopy{}, fmt.Errorf("copy %s: %w", filepath.Base(source.path), err)
	}

	name := filepath.Base(saved)
	url := helpers.STATIC_URL_PREFIX + filepath.ToSlash(filepath.Join(rel, name)) + "?v=" + version
	return localCopy{dir: dir, name: name, url: url}, nil
}

// existingCopy returns the path of a regular file named stem plus an image
// extension in dir, or "" when there is none.
func existingCopy(dir, stem string) string {
	for _, ext := range []string{".jpg", ".png", ".gif", ".webp"} {
		path := filepath.Join(dir, stem+ext)
		info, err := os.Lstat(path)
		if err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	return ""
}

// readCapped reads the file at path, failing with helpers.ErrImageTooLarge
// past limit bytes.
func readCapped(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		return nil, helpers.ErrImageTooLarge
	}
	return content, nil
}

// saveAlbumArt points cover_local at value and records the fingerprint of
// what it was copied from, while the static directory is still the one the
// copy is in. It writes nothing for an album deleted meanwhile.
func (s *Scanner) saveAlbumArt(ctx context.Context, art *localArt, albumID int64, value, fingerprint sql.NullString) (bool, error) {
	changed := false
	err := s.tx.Run(ctx, func(q *database.Queries) error {
		changed = false
		err := art.check()
		if err != nil {
			return err
		}
		n, err := q.SetMusicAlbumLocalCover(ctx, database.SetMusicAlbumLocalCoverParams{ID: albumID, CoverLocal: value})
		if err != nil {
			return err
		}
		changed = n > 0
		return q.SaveMusicAlbumLocalCoverFingerprint(ctx, database.SaveMusicAlbumLocalCoverFingerprintParams{AlbumID: albumID, Fingerprint: fingerprint})
	}, nil)
	if err != nil {
		// Nothing committed.
		return false, err
	}
	return changed, nil
}

// clearAlbumArt drops the copy of an album that no longer has art of its
// own, so its cover is its provider's again.
func (s *Scanner) clearAlbumArt(ctx context.Context, art *localArt, albumID int64, current database.GetMusicAlbumLocalCoverRow) (bool, error) {
	hadCopy := current.CoverLocal.Valid || current.LocalCoverFingerprint.Valid
	if !hadCopy {
		return false, nil
	}
	changed, err := s.saveAlbumArt(ctx, art, albumID, sql.NullString{}, sql.NullString{})
	if err != nil {
		return false, err
	}
	rel := filepath.Join(localCoverDir, strconv.FormatInt(albumID, 10))
	if art.check() == nil && checkLocalCoverDir(art.staticDir, rel) == nil {
		_ = os.RemoveAll(filepath.Join(art.staticDir, rel))
	}
	return changed, nil
}

// pruneLocalCovers removes the folders of albums that no longer point at a
// copy: deleted albums, and albums whose art went away while a write failed.
// It reports how many it removed.
func (s *Scanner) pruneLocalCovers(ctx context.Context, art *localArt) (int, error) {
	// The roots may lead elsewhere by now, through a symlink the settings
	// do not show: check the overlap again before deleting anything.
	_, overlaps := helpers.OverlappingLibrary(filepath.Join(art.staticDir, "local"), s.libraryDirs())
	if overlaps {
		return 0, errSettingsChanged
	}
	err := checkLocalCoverDir(art.staticDir, localCoverDir)
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(filepath.Join(art.staticDir, localCoverDir))
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	ids, err := s.queries.ListMusicAlbumsWithLocalCover(ctx)
	if err != nil {
		return 0, err
	}
	kept := make(map[string]bool, len(ids))
	for _, id := range ids {
		kept[strconv.FormatInt(id, 10)] = true
	}
	removed := 0
	for _, entry := range entries {
		if ctx.Err() != nil {
			return removed, ctx.Err()
		}
		name := entry.Name()
		isAlbumFolder := entry.IsDir() && strings.Trim(name, "0123456789") == "" && name != ""
		if !isAlbumFolder || kept[name] {
			continue
		}
		err = art.check()
		if err != nil {
			return removed, err
		}
		err = os.RemoveAll(filepath.Join(art.staticDir, localCoverDir, name))
		if err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// errLinkedLocalCoverDir refuses a folder on the way to a copy that is a
// symlink or not a directory: Igloo creates these folders itself, so one that
// is not a real directory leads somewhere else, perhaps into a library.
var errLinkedLocalCoverDir = errors.New("local cover folder is not a directory")

// checkLocalCoverDir reports whether every folder of rel under staticDir is
// a real directory or does not exist yet.
func checkLocalCoverDir(staticDir, rel string) error {
	current := staticDir
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("%w: %s", errLinkedLocalCoverDir, current)
		}
	}
	return nil
}

// localCoverExists reports whether the copy a cover_local URL names is a
// regular file in a real folder of the static directory.
func localCoverExists(staticDir, value string) bool {
	rest, isLocal := strings.CutPrefix(value, helpers.STATIC_URL_PREFIX)
	if !isLocal {
		return false
	}
	rel, _, _ := strings.Cut(rest, "?v=")
	rel = filepath.FromSlash(rel)
	if checkLocalCoverDir(staticDir, filepath.Dir(rel)) != nil {
		return false
	}
	info, err := os.Lstat(filepath.Join(staticDir, rel))
	return err == nil && info.Mode().IsRegular()
}

// localCoverMissing reports whether the copy a cover_local URL names is known
// to be gone: absent, or not a regular file. A static directory that cannot
// be read right now says nothing.
func localCoverMissing(staticDir, value string) bool {
	rest, isLocal := strings.CutPrefix(value, helpers.STATIC_URL_PREFIX)
	if !isLocal {
		return true
	}
	rel, _, _ := strings.Cut(rest, "?v=")
	info, err := os.Lstat(filepath.Join(staticDir, filepath.FromSlash(rel)))
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	return err == nil && !info.Mode().IsRegular()
}

// removeOtherFiles removes every file in dir except keep: earlier copies, or
// the temporary file of an interrupted one.
func removeOtherFiles(dir, keep string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.Name() == keep || entry.IsDir() {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
}
