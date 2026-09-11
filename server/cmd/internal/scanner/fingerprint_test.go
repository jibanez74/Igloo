package scanner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func inspectTestFile(t testing.TB, path string, previous *FileFingerprint) *FileInspection {
	t.Helper()
	result, err := InspectFile(context.Background(), path, previous, func() time.Time { return time.Now().Add(time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		err := result.Close()
		if err != nil {
			t.Error(err)
		}
	})
	return result
}

func writeFingerprintFile(t testing.TB, path string, data []byte) {
	t.Helper()
	err := os.WriteFile(path, data, 0600)
	if err != nil {
		t.Fatal(err)
	}
}

func TestInspectSinglePathAndFullContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "track")
	content := bytes.Repeat([]byte("a"), 3*256*1024)
	writeFingerprintFile(t, path, content)
	first := inspectTestFile(t, path, nil)
	if first.Outcome != FileNeedsProcessing || first.Fingerprint.SHA256 != sha256.Sum256(content) {
		t.Fatalf("initial inspection: %+v", first)
	}
	// Matching metadata returns before opening the file or allocating a hasher.
	second := inspectTestFile(t, path, &first.Fingerprint)
	if second.Outcome != FileUnchanged || second.file != nil {
		t.Fatalf("unchanged: %+v", second)
	}
	for _, offset := range []int{0, len(content) / 2, len(content) - 1} {
		baseline := first.Fingerprint
		content[offset]++
		writeFingerprintFile(t, path, content)
		mtime := time.Unix(0, baseline.MtimeNS)
		err := os.Chtimes(path, mtime, mtime)
		if err != nil {
			t.Fatal(err)
		}
		first = inspectTestFile(t, path, &baseline)
		if first.Outcome != FileNeedsProcessing || first.Fingerprint.SHA256 != sha256.Sum256(content) {
			t.Fatalf("edit at %d not detected: %+v", offset, first)
		}
	}
}

func TestInspectIdenticalBytes(t *testing.T) {
	for _, change := range []string{"mtime", "permissions", "replacement"} {
		t.Run(change, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "movie")
			writeFingerprintFile(t, path, []byte("media"))
			first := inspectTestFile(t, path, nil)
			var err error
			switch change {
			case "mtime":
				stamp := time.Now().Add(-time.Hour)
				err = os.Chtimes(path, stamp, stamp)
			case "permissions":
				err = os.Chmod(path, 0640)
			case "replacement":
				writeFingerprintFile(t, path+".new", []byte("media"))
				err = os.Rename(path+".new", path)
			}
			if err != nil {
				t.Fatal(err)
			}
			next := inspectTestFile(t, path, &first.Fingerprint)
			if next.Outcome != FileFingerprintOnly {
				t.Fatalf("got %+v", next)
			}
			repeated := inspectTestFile(t, path, &next.Fingerprint)
			if repeated.Outcome != FileUnchanged {
				t.Fatalf("repeated: %+v", repeated)
			}
		})
	}
}

func TestInspectQuietPeriod(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	writeFingerprintFile(t, path, []byte("media"))
	baseline := inspectTestFile(t, path, nil).Fingerprint
	eligible := time.Unix(0, max(baseline.MtimeNS, baseline.CtimeNS)).Add(FileQuietPeriod)
	for _, delta := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond} {
		now := func() time.Time { return eligible.Add(delta) }
		r, err := InspectFile(context.Background(), path, nil, now)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		if delta < 0 {
			if r.Outcome != FileDeferred || r.Reason != FileTooRecent || r.EligibleAt != eligible || r.file != nil {
				t.Fatalf("quiet period: %+v", r)
			}
		} else if r.Outcome != FileNeedsProcessing {
			t.Fatalf("eligible: %+v", r)
		}
	}
	future := time.Now().Add(24 * time.Hour)
	err := os.Chtimes(path, future, future)
	if err != nil {
		t.Fatal(err)
	}
	r, err := InspectFile(context.Background(), path, &baseline, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if r.Outcome != FileDeferred || !r.EligibleAt.Equal(future.Add(FileQuietPeriod)) {
		t.Fatalf("future dated: %+v", r)
	}
	// Restoring mtime cannot bypass the later status-change timestamp.
	past := time.Now().Add(-time.Hour)
	err = os.Chtimes(path, past, past)
	if err != nil {
		t.Fatal(err)
	}
	r, err = InspectFile(context.Background(), path, &baseline, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if r.Outcome != FileDeferred || r.Fingerprint.CtimeNS <= r.Fingerprint.MtimeNS {
		t.Fatalf("ctime: %+v", r)
	}
}

func TestInspectionRejectsChangesBeforeCommit(t *testing.T) {
	for _, change := range []string{"write", "replace", "remove", "symlink", "same target symlink", "fifo", "directory symlink"} {
		t.Run(change, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "file")
			writeFingerprintFile(t, path, []byte("media"))
			if change == "symlink" || change == "same target symlink" {
				err := os.Symlink(path, path+".link")
				if err != nil {
					t.Fatal(err)
				}
				path += ".link"
			}
			if change == "directory symlink" {
				err := os.Symlink(dir, dir+".link")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { os.Remove(dir + ".link") })
				path = filepath.Join(dir+".link", "file")
			}
			r := inspectTestFile(t, path, nil)
			var err error
			switch change {
			case "write":
				writeFingerprintFile(t, path, []byte("edited"))
			case "replace":
				writeFingerprintFile(t, path+".new", []byte("other"))
				err = os.Rename(path+".new", path)
			case "remove", "fifo":
				err = os.Remove(path)
				if err == nil && change == "fifo" {
					err = syscall.Mkfifo(path, 0600)
				}
			case "symlink", "same target symlink":
				target := filepath.Join(dir, "other")
				writeFingerprintFile(t, target, []byte("other"))
				if change == "same target symlink" {
					target = filepath.Join(dir, "file")
				}
				err = os.Symlink(target, path+".new")
				if err == nil {
					err = os.Rename(path+".new", path)
				}
			case "directory symlink":
				other := t.TempDir()
				writeFingerprintFile(t, filepath.Join(other, "file"), []byte("other"))
				err = os.Remove(dir + ".link")
				if err == nil {
					err = os.Symlink(other, dir+".link")
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			err = r.Validate(context.Background())
			var deferred *FileDeferral
			if !errors.As(err, &deferred) || deferred.Reason != FileChanged {
				t.Fatalf("validation = %v", err)
			}
		})
	}
}

func TestInspectionRacedOpenAndFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	writeFingerprintFile(t, path, []byte("media"))
	// The clock is called between initial stat and open; replace with a FIFO at
	// that boundary to verify the open cannot hang and the race is deferred.
	clock := func() time.Time {
		err := os.Remove(path)
		if err != nil {
			t.Fatal(err)
		}
		err = syscall.Mkfifo(path, 0600)
		if err != nil {
			t.Fatal(err)
		}
		return time.Now().Add(time.Hour)
	}
	r, err := InspectFile(context.Background(), path, nil, clock)
	if err != nil || r.Outcome != FileDeferred {
		t.Fatalf("raced FIFO: %+v %v", r, err)
	}
	err = r.Close()
	if err != nil {
		t.Fatal(err)
	}
	_, err = InspectFile(context.Background(), path, nil, time.Now)
	if err == nil {
		t.Fatal("accepted special file")
	}
	_, err = InspectFile(context.Background(), path+".missing", nil, time.Now)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = InspectFile(ctx, path, nil, time.Now)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	err = os.Remove(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFingerprintFile(t, path, []byte("media"))
	err = os.Chmod(path, 0000)
	if err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() != 0 {
		_, err = InspectFile(context.Background(), path, nil, func() time.Time { return time.Now().Add(time.Hour) })
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("unreadable: %v", err)
		}
	}
}

type callbackReader struct{ read func([]byte) (int, error) }

func (r callbackReader) Read(b []byte) (int, error) { return r.read(b) }

func TestStreamingHashFailuresAndCancellation(t *testing.T) {
	failure := errors.New("read failed")
	_, err := hashFile(context.Background(), callbackReader{read: func([]byte) (int, error) { return 0, failure }})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err = hashFile(ctx, callbackReader{read: func(b []byte) (int, error) { calls++; cancel(); return len(b), nil }})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("err=%v reads=%d", err, calls)
	}
	// Validate observes a write interleaved with streaming reads on the held FD.
	path := filepath.Join(t.TempDir(), "file")
	writeFingerprintFile(t, path, bytes.Repeat([]byte("a"), 512*1024))
	r := inspectTestFile(t, path, nil)
	_, err = r.file.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}
	wrote := false
	_, err = hashFile(context.Background(), callbackReader{read: func(b []byte) (int, error) {
		n, readErr := r.file.Read(b)
		if !wrote {
			wrote = true
			writeFingerprintFile(t, path, []byte("changed"))
		}
		return n, readErr
	}})
	if err != nil {
		t.Fatal(err)
	}
	err = r.Validate(context.Background())
	var deferred *FileDeferral
	if !errors.As(err, &deferred) {
		t.Fatalf("concurrent write: %v", err)
	}
}

func BenchmarkInspectUnchanged(b *testing.B) {
	path := filepath.Join(b.TempDir(), "file")
	writeFingerprintFile(b, path, []byte("media"))
	baseline := inspectTestFile(b, path, nil).Fingerprint
	now := func() time.Time { return time.Now().Add(time.Hour) }
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		r, err := InspectFile(context.Background(), path, &baseline, now)
		if err != nil || r.Outcome != FileUnchanged {
			b.Fatalf("%+v %v", r, err)
		}
	}
}

func BenchmarkStreamingHash(b *testing.B) {
	path := filepath.Join(b.TempDir(), "file")
	writeFingerprintFile(b, path, bytes.Repeat([]byte("a"), 8*1024*1024))
	f, err := os.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	b.SetBytes(8 * 1024 * 1024)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, err = f.Seek(0, io.SeekStart)
		if err != nil {
			b.Fatal(err)
		}
		_, err = hashFile(context.Background(), f)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestMetadataInspectionReadsNoContent(t *testing.T) {
	for _, size := range []int64{1, 8 << 30, 1 << 40} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sparse.mkv")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			err = file.Truncate(size)
			file.Close()
			if err != nil {
				t.Fatal(err)
			}
			inspection, err := InspectFileMetadata(context.Background(), path, nil, func() time.Time { return time.Now().Add(time.Hour) })
			if err != nil {
				t.Fatal(err)
			}
			defer inspection.Close()
			// The descriptor's offset must remain at zero regardless of sparse size.
			offset, err := inspection.file.Seek(0, io.SeekCurrent)
			if err != nil || offset != 0 || inspection.Fingerprint.SHA256 != [32]byte{} || inspection.Fingerprint.Size != size {
				t.Fatalf("inspection read content: offset=%d fingerprint=%+v err=%v", offset, inspection.Fingerprint, err)
			}
			err = inspection.Validate(context.Background())
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
