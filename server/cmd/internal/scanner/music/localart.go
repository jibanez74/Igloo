package music

import (
	"cmp"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
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
// copied: local/albums/<album id>/cover<ext>. The artwork syncer manages
// other folders, so turning download_images off never touches these.
var localCoverDir = filepath.Join("local", "albums")

// localCoverStem names the copy; its extension follows the type its bytes
// sniff as.
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
	// artFailed: art was found but none of it could be copied; a cover
	// copied before stays.
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

// errStaticDirChanged stops a pass whose static directory changed under it;
// the settings save that changed it requests another.
var errStaticDirChanged = errors.New("static_dir changed during the pass")

// localArt is one pass's local-art context.
type localArt struct {
	staticDir string
	// owners caches each folder's verdict for the album that asked.
	owners map[string]int64
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
	sources, err := s.albumArtSources(ctx, art, albumID)
	if err != nil {
		return artFailed, false, err
	}
	if len(sources) == 0 {
		changed, err := s.clearAlbumArt(ctx, art, albumID, current)
		return artNone, changed, err
	}

	isUnchanged := current.CoverLocal.Valid && current.LocalCoverFingerprint.Valid &&
		current.LocalCoverFingerprint.String == sources[0].fingerprint && localCoverExists(art.staticDir, current.CoverLocal.String)
	if isUnchanged {
		return artFound, false, nil
	}

	var copyErr error
	embeddedAttempts := 0
	for _, source := range sources {
		if source.embedded {
			if s.pictures == nil || embeddedAttempts == maxEmbeddedAttempts {
				continue
			}
			embeddedAttempts++
		}
		url, err := s.copyAlbumArt(ctx, art, albumID, source)
		if ctx.Err() != nil {
			return artFailed, false, ctx.Err()
		}
		if err != nil {
			s.logger.Debug("failed to copy album art", "album_id", albumID, "source", source.path, "error", err)
			copyErr = errors.Join(copyErr, err)
			continue
		}
		changed, err := s.saveAlbumArt(ctx, art, albumID, sql.NullString{String: url, Valid: true}, helpers.NullString(source.fingerprint))
		if err != nil {
			return artFailed, false, err
		}
		return artFound, changed || current.CoverLocal.String != url, nil
	}
	if copyErr != nil {
		s.logger.Warn("failed to copy the album's own art", "album_id", albumID, "error", copyErr)
	}
	return artFailed, false, nil
}

// albumArtSources lists the images an album may use, best first.
func (s *Scanner) albumArtSources(ctx context.Context, art *localArt, albumID int64) ([]artSource, error) {
	tracks, err := s.queries.ListMusicAlbumArtTracks(ctx, sql.NullInt64{Int64: albumID, Valid: true})
	if err != nil {
		return nil, err
	}

	// Its tracks may lie in more than one folder; the one holding most of
	// them comes first.
	counts := make(map[string]int)
	discs := make(map[string][]string)
	for _, track := range tracks {
		folder, disc := albumFolder(track.FilePath)
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

	var sources []artSource
	for _, folder := range folders {
		owned, err := art.ownsFolder(ctx, s.queries, albumID, folder)
		if err != nil {
			return nil, err
		}
		if owned {
			sources = append(sources, folderImages(folder)...)
		}
		discFolders := discs[folder]
		slices.Sort(discFolders)
		for _, disc := range discFolders {
			owned, err := art.ownsFolder(ctx, s.queries, albumID, disc)
			if err != nil {
				return nil, err
			}
			if owned {
				sources = append(sources, folderImages(disc)...)
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
	return sources, nil
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
		rel, err := filepath.Rel(folder, row.FilePath)
		if err != nil {
			continue
		}
		dir := filepath.Dir(rel)
		isNear := dir == "." || (!strings.Contains(dir, string(filepath.Separator)) && isDiscFolder(dir))
		if !isNear {
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

// folderImages lists the cover images in folder, best first. A symlink is
// followed, as the library's tracks are.
func folderImages(folder string) []artSource {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil
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
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		fingerprint := "file:" + path + ":" + strconv.FormatInt(info.Size(), 10) + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
		found = append(found, ranked{source: artSource{path: path, fingerprint: fingerprint}, rank: nameRank*len(coverExtensions) + extRank})
	}
	slices.SortStableFunc(found, func(a, b ranked) int { return cmp.Compare(a.rank, b.rank) })
	sources := make([]artSource, 0, len(found))
	for _, f := range found {
		sources = append(sources, f.source)
	}
	return sources
}

// copyAlbumArt copies source into the album's folder and returns the URL of
// the copy, versioned by a hash of its bytes. Any other file in the folder is
// an earlier copy and is removed.
func (s *Scanner) copyAlbumArt(ctx context.Context, art *localArt, albumID int64, source artSource) (string, error) {
	rel := filepath.Join(localCoverDir, strconv.FormatInt(albumID, 10))
	dir := filepath.Join(art.staticDir, rel)
	err := checkLocalCoverDir(art.staticDir, rel)
	if err != nil {
		return "", err
	}
	_, err = helpers.GetOrCreateDir(dir)
	if err != nil {
		return "", err
	}

	path := source.path
	if source.embedded {
		extracted := filepath.Join(dir, helpers.TEMP_FILE_PREFIX+"embedded")
		defer os.Remove(extracted)
		err = s.pictures.ExtractAttachedPicture(ctx, source.path, source.stream, extracted)
		if err != nil {
			return "", err
		}
		path = extracted
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	saved, err := helpers.SaveImage(dir, localCoverStem, io.TeeReader(file, digest), maxLocalCoverSize)
	if err != nil {
		return "", fmt.Errorf("copy %s: %w", filepath.Base(source.path), err)
	}
	removeOtherFiles(dir, filepath.Base(saved))

	name := filepath.ToSlash(filepath.Join(rel, filepath.Base(saved)))
	return helpers.STATIC_URL_PREFIX + name + "?v=" + shortHash(digest), nil
}

// saveAlbumArt points cover_local at value and records the fingerprint of
// what it was copied from, while the static directory is still the one the
// copy is in. It writes nothing for an album deleted meanwhile.
func (s *Scanner) saveAlbumArt(ctx context.Context, art *localArt, albumID int64, value, fingerprint sql.NullString) (bool, error) {
	changed := false
	err := s.tx.Run(ctx, func(q *database.Queries) error {
		isCurrent := s.staticDir() == art.staticDir
		if !isCurrent {
			return errStaticDirChanged
		}
		n, err := q.SetMusicAlbumLocalCover(ctx, database.SetMusicAlbumLocalCoverParams{ID: albumID, CoverLocal: value})
		if err != nil {
			return err
		}
		changed = n > 0
		return q.SaveMusicAlbumLocalCoverFingerprint(ctx, database.SaveMusicAlbumLocalCoverFingerprintParams{AlbumID: albumID, Fingerprint: fingerprint})
	}, nil)
	return changed, err
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
	if checkLocalCoverDir(art.staticDir, rel) == nil {
		_ = os.RemoveAll(filepath.Join(art.staticDir, rel))
	}
	return changed, nil
}

// pruneLocalCovers removes the folders of albums that no longer point at a
// copy: deleted albums, and albums whose art went away while a write failed.
// It reports how many it removed.
func (s *Scanner) pruneLocalCovers(ctx context.Context, art *localArt) (int, error) {
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

// removeOtherFiles removes every file in dir except keep: an earlier copy of
// another type, or the temporary file of an interrupted one.
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

func shortHash(digest hash.Hash) string {
	return hex.EncodeToString(digest.Sum(nil)[:4])
}
