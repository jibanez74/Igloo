package scanner

import (
	"context"
	"os"
	"path/filepath"

	"igloo/cmd/internal/database"
)

// moveKey is what a rename, or a move within one filesystem, keeps of a file:
// its inode, size and modification time. The device is left out, since
// device numbers change across boots.
type moveKey struct {
	inode string
	size  int64
	mtime int64
}

func moveKeyOf(fingerprint FileFingerprint) (moveKey, bool) {
	known := fingerprint.Inode != ""
	return moveKey{inode: fingerprint.Inode, size: fingerprint.Size, mtime: fingerprint.MtimeNS}, known
}

// ambiguousMove marks a key two cataloged files share, which matches neither.
const ambiguousMove = -1

// MovedFrom returns the cataloged file of this run that a file the catalog
// does not hold at path is, found again after a rename or a move within the
// library: the one with the same inode, size and modification time. A file
// that cannot be stat'ed, and a key two cataloged files share, match nothing.
// MoveConfirmed checks that the file left its old path.
func (r *Reconciliation) MovedFrom(path string) (CatalogFile, bool) {
	if r.moves == nil {
		r.moves = make(map[moveKey]int)
		for i, file := range r.candidates {
			key, known := moveKeyOf(file.Fingerprint)
			if !known {
				continue
			}
			_, shared := r.moves[key]
			if shared {
				r.moves[key] = ambiguousMove
				continue
			}
			r.moves[key] = i
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return CatalogFile{}, false
	}
	key, known := moveKeyOf(FingerprintOf(info))
	index, found := r.moves[key]
	if !known || !found || index == ambiguousMove {
		return CatalogFile{}, false
	}
	file := r.candidates[index]
	same := filepath.Clean(file.Path) == filepath.Clean(path)
	if same || r.moved[file.ID] {
		return CatalogFile{}, false
	}
	return file, true
}

// MoveConfirmed re-keys a cataloged file to the path MovedFrom found it under,
// through rekey inside a transaction, once its old path is confirmed missing,
// and confirms that again immediately before commit, as DeleteConfirmed does:
// a file still at its old path was copied or linked, not moved. A committed
// move takes the file out of the cleanup.
func (r *Reconciliation) MoveConfirmed(
	ctx context.Context,
	tx TxRunner,
	file CatalogFile,
	rekey func(*database.Queries) (bool, error),
	committed func(),
) (bool, error) {
	moved, err := r.whileMissing(ctx, tx, file, rekey, committed)
	if err != nil || !moved {
		return false, err
	}
	r.moved[file.ID] = true
	return true, nil
}
