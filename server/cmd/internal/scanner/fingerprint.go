package scanner

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const FileQuietPeriod = 60 * time.Second

// FileFingerprint is the baseline of a successfully processed regular file.
// Device and inode use decimal text so SQLite never truncates unsigned IDs.
type FileFingerprint struct {
	Size    int64
	MtimeNS int64
	CtimeNS int64
	Device  string
	Inode   string
	SHA256  [sha256.Size]byte
}

type FileOutcome uint8

const (
	FileUnchanged FileOutcome = iota
	FileFingerprintOnly
	FileNeedsProcessing
	FileDeferred
)

type DeferralReason string

const (
	FileTooRecent DeferralReason = "quiet period"
	FileChanged   DeferralReason = "file changed during inspection or processing"
)

// FileDeferral also carries the observed fingerprint when a later validation
// detects a change.
type FileDeferral struct {
	Reason      DeferralReason
	Fingerprint FileFingerprint
}

func (d *FileDeferral) Error() string { return string(d.Reason) }

// FileInspection owns a descriptor for new or changed files. Call Close
// after processing, and Validate after resolution and immediately before commit.
// It has no scan index, directory walker, scheduler, or scan-lifetime state.
type FileInspection struct {
	Outcome      FileOutcome
	Fingerprint  FileFingerprint
	Reason       DeferralReason
	EligibleAt   time.Time
	file         *os.File
	path         string
	resolvedPath string
	entry        FileFingerprint
}

// StoredFingerprint rebuilds the persisted baseline from the nullable scan
// index columns every catalog shares.
func StoredFingerprint(size int64, mtimeNS, ctimeNS sql.NullInt64, device, inode sql.NullString) FileFingerprint {
	return FileFingerprint{Size: size, MtimeNS: mtimeNS.Int64, CtimeNS: ctimeNS.Int64, Device: device.String, Inode: inode.String}
}

// Close releases the descriptor; it is safe on a nil inspection.
func (r *FileInspection) Close() error {
	if r == nil || r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	return err
}

// ReadAt reads the inspected file through the descriptor that was hashed, so
// what it reads is the content Validate vouches for.
func (r *FileInspection) ReadAt(p []byte, off int64) (int, error) {
	return r.file.ReadAt(p, off)
}

// FingerprintOf reads the metadata fingerprint of a stat result, without the
// content hash.
func FingerprintOf(info os.FileInfo) FileFingerprint {
	return fingerprintFromInfo(info)
}

// ChangedAt is the later of a file's mtime and ctime, the moment the quiet
// period counts from.
func (f FileFingerprint) ChangedAt() time.Time {
	return time.Unix(0, max(f.MtimeNS, f.CtimeNS))
}

func sameFileMetadata(a, b FileFingerprint) bool {
	return a.Size == b.Size && a.MtimeNS == b.MtimeNS && a.CtimeNS == b.CtimeNS && a.Device == b.Device && a.Inode == b.Inode
}

// InspectFile compares metadata first, then hashes the entire file if necessary.
// A file changed within quietPeriod of now is deferred: FileQuietPeriod for a
// library scan, ChangeQuietPeriod for a run of watcher-detected changes.
func InspectFile(ctx context.Context, path string, previous *FileFingerprint, now func() time.Time, quietPeriod time.Duration) (*FileInspection, error) {
	return inspectFile(ctx, path, previous, now, quietPeriod, true)
}

// InspectFileMetadata inspects and pins a file without reading its content.
// Changed filesystem metadata always requires processing, even for identical bytes.
func InspectFileMetadata(ctx context.Context, path string, previous *FileFingerprint, now func() time.Time, quietPeriod time.Duration) (*FileInspection, error) {
	return inspectFile(ctx, path, previous, now, quietPeriod, false)
}

func inspectFile(ctx context.Context, path string, previous *FileFingerprint, now func() time.Time, quietPeriod time.Duration, hashContent bool) (*FileInspection, error) {
	err := ctx.Err()
	if err != nil {
		return nil, err
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	regular := info.Mode().IsRegular()
	if !regular {
		return nil, fmt.Errorf("not a regular file: %s", path)
	}
	observed := fingerprintFromInfo(info)
	eligible := observed.ChangedAt().Add(quietPeriod)
	r := &FileInspection{path: path, Fingerprint: observed}
	observedAt := now()
	err = ctx.Err()
	if err != nil {
		return nil, err
	}
	tooRecent := observedAt.Before(eligible)
	if tooRecent {
		r.Outcome, r.Reason, r.EligibleAt = FileDeferred, FileTooRecent, eligible
		return r, nil
	}
	unchanged := previous != nil && sameFileMetadata(*previous, observed)
	if unchanged {
		r.Outcome, r.Fingerprint = FileUnchanged, *previous
		return r, nil
	}
	err = r.open(ctx)
	if err == nil && hashContent {
		err = r.hash(ctx)
	}
	if err != nil {
		closeErr := r.Close()
		var deferred *FileDeferral
		isDeferred := errors.As(err, &deferred)
		if isDeferred {
			r.Outcome, r.Reason, r.Fingerprint = FileDeferred, deferred.Reason, deferred.Fingerprint
			return r, closeErr
		}
		return nil, errors.Join(err, closeErr)
	}
	r.Outcome = FileNeedsProcessing
	if hashContent && previous != nil && previous.Size == observed.Size && previous.SHA256 == r.Fingerprint.SHA256 {
		r.Outcome = FileFingerprintOnly
	}
	return r, nil
}

var hashBuffers = sync.Pool{New: func() any { b := make([]byte, 256*1024); return &b }}

func (r *FileInspection) open(ctx context.Context) error {
	entry, err := os.Lstat(r.path)
	if err != nil {
		return inspectionPathError(err)
	}
	r.entry = fingerprintFromInfo(entry)
	r.resolvedPath, err = filepath.EvalSymlinks(r.path)
	if err != nil {
		return inspectionPathError(err)
	}
	// Nonblocking open prevents a raced replacement with a FIFO from hanging.
	r.file, err = os.OpenFile(r.path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return inspectionPathError(err)
	}
	return r.Validate(ctx)
}

func (r *FileInspection) hash(ctx context.Context) error {
	// Bound the read to the observed size so an active append cannot keep
	// hashing alive indefinitely. Validation below rejects a changed size.
	sum, err := hashFile(ctx, io.LimitReader(r.file, r.Fingerprint.Size))
	if err != nil {
		return err
	}
	err = r.Validate(ctx)
	if err != nil {
		return err
	}
	r.Fingerprint.SHA256 = sum
	return nil
}

func hashFile(ctx context.Context, reader io.Reader) ([sha256.Size]byte, error) {
	buffer := hashBuffers.Get().(*[]byte)
	defer hashBuffers.Put(buffer)
	h := sha256.New()
	for {
		err := ctx.Err()
		if err != nil {
			return [sha256.Size]byte{}, err
		}
		n, readErr := reader.Read(*buffer)
		_, err = h.Write((*buffer)[:n])
		if err != nil {
			return [sha256.Size]byte{}, err
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return [sha256.Size]byte{}, readErr
		}
	}
	err := ctx.Err()
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return [sha256.Size]byte(h.Sum(nil)), nil
}

func inspectionPathError(err error) error {
	missing := errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) || errors.Is(err, syscall.ELOOP)
	if missing {
		return &FileDeferral{Reason: FileChanged}
	}
	return err
}

func (r *FileInspection) Validate(ctx context.Context) error {
	err := ctx.Err()
	if err != nil {
		return err
	}
	info, err := r.file.Stat()
	if err != nil {
		return err
	}
	observed := fingerprintFromInfo(info)
	stable := info.Mode().IsRegular() && sameFileMetadata(r.Fingerprint, observed)
	if !stable {
		return &FileDeferral{Reason: FileChanged, Fingerprint: observed}
	}
	info, err = os.Stat(r.path)
	if err != nil {
		return inspectionPathError(err)
	}
	observed = fingerprintFromInfo(info)
	stable = info.Mode().IsRegular() && sameFileMetadata(r.Fingerprint, observed)
	if !stable {
		return &FileDeferral{Reason: FileChanged, Fingerprint: observed}
	}
	entry, err := os.Lstat(r.path)
	if err != nil {
		return inspectionPathError(err)
	}
	resolved, err := filepath.EvalSymlinks(r.path)
	if err != nil {
		return inspectionPathError(err)
	}
	stable = resolved == r.resolvedPath && sameFileMetadata(r.entry, fingerprintFromInfo(entry))
	if !stable {
		return &FileDeferral{Reason: FileChanged, Fingerprint: observed}
	}
	return ctx.Err()
}
