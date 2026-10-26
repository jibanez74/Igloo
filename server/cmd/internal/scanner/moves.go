package scanner

import (
	"context"
	"os"
	"path/filepath"

	"igloo/cmd/internal/database"
)

// moveKey is what a rename, or a move within one filesystem, keeps of a file:
// its inode, size and modification time. The device is left out, since
// device numbers change across reboots.
type moveKey struct {
	inode string
	size  int64
	mtime int64
}

func moveKeyOf(fingerprint FileFingerprint) (moveKey, bool) {
	known := fingerprint.Inode != ""
	return moveKey{inode: fingerprint.Inode, size: fingerprint.Size, mtime: fingerprint.MtimeNS}, known
}

// Move is a cataloged file found again under a new path.
type Move struct {
	From CatalogFile
	To   string
}

// MovedFrom pairs a file the catalog does not hold at path with the cataloged
// file of this run it is, found again after a rename or a move within the
// library: the one with the same inode, size and modification time, among
// those the walk has not seen at their own paths and not already moved. A
// file that cannot be stat'ed, and a key two such cataloged files share,
// pair with nothing. MoveConfirmed checks that the file left its old path.
func (r *Reconciliation) MovedFrom(path string) (Move, bool) {
	if r.moves == nil {
		r.moves = make(map[moveKey][]int)
		for i, file := range r.candidates {
			key, known := moveKeyOf(file.Fingerprint)
			if known {
				r.moves[key] = append(r.moves[key], i)
			}
		}
	}
	if len(r.moves) == 0 {
		return Move{}, false
	}
	info, err := os.Stat(path)
	if err != nil {
		return Move{}, false
	}
	key, known := moveKeyOf(FingerprintOf(info))
	if !known {
		return Move{}, false
	}
	// Seen and moved candidates are filtered here rather than when the index
	// is built: music pairs each batch while the walk is still seeing files.
	var pair []CatalogFile
	for _, index := range r.moves[key] {
		file := r.candidates[index]
		seen := r.saw(file)
		if !seen && !r.moved[file.ID] {
			pair = append(pair, file)
		}
	}
	if len(pair) != 1 {
		return Move{}, false
	}
	same := filepath.Clean(pair[0].Path) == filepath.Clean(path)
	if same {
		return Move{}, false
	}
	return Move{From: pair[0], To: path}, true
}

// MoveConfirmed re-keys a cataloged file to the path MovedFrom found it under,
// through rekey inside a transaction, once its old path is confirmed missing,
// and confirms that again immediately before commit, as DeleteConfirmed does:
// a file still at its old path was copied or linked, not moved. A committed
// move takes the file out of the cleanup and counts toward the report's
// catalog changes.
func (r *Reconciliation) MoveConfirmed(
	ctx context.Context,
	tx TxRunner,
	move Move,
	report *Report,
	rekey func(*database.Queries) (bool, error),
	committed func(),
) (bool, error) {
	moved, err := r.whileMissing(ctx, tx, move.From, rekey, committed)
	if err != nil || !moved {
		return false, err
	}
	r.moved[move.From.ID] = true
	report.moves++
	return true, nil
}
