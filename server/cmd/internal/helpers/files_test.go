package helpers

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomic_ReplacesTheTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.txt")
	err := os.WriteFile(path, []byte("old"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	err = WriteFileAtomic(path, strings.NewReader("new"))
	if err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("target holds %q, want %q", got, "new")
	}
	names := dirEntries(t, dir)
	if len(names) != 1 {
		t.Fatalf("dir holds %v, want only target.txt", names)
	}
}

func TestWriteFileAtomic_FailedWriteKeepsTheTargetAndRemovesTheTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.txt")
	err := os.WriteFile(path, []byte("old"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	cutOff := errors.New("connection reset")

	err = WriteFileAtomic(path, &failingReader{data: []byte("partial"), err: cutOff})
	if !errors.Is(err, cutOff) {
		t.Fatalf("err = %v, want the read error", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old" {
		t.Fatalf("target holds %q after a failed write, want %q", got, "old")
	}
	names := dirEntries(t, dir)
	if len(names) != 1 {
		t.Fatalf("dir holds %v, want only target.txt", names)
	}
}

func TestWriteFileAtomic_MissingDirectoryFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "target.txt")

	err := WriteFileAtomic(path, strings.NewReader("data"))
	if err == nil {
		t.Fatal("WriteFileAtomic succeeded without a directory")
	}
}
