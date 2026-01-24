package helpers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetOrCreateDir_CreatesNewDirectory(t *testing.T) {
	tempDir := t.TempDir()
	newDir := filepath.Join(tempDir, "newdir")

	created, err := GetOrCreateDir(newDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !created {
		t.Error("expected created to be true for new directory")
	}

	info, err := os.Stat(newDir)
	if err != nil {
		t.Fatalf("expected directory to exist, got error: %v", err)
	}

	if !info.IsDir() {
		t.Error("expected path to be a directory")
	}
}

func TestGetOrCreateDir_CreatesNestedDirectories(t *testing.T) {
	tempDir := t.TempDir()
	nestedDir := filepath.Join(tempDir, "a", "b", "c")

	created, err := GetOrCreateDir(nestedDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !created {
		t.Error("expected created to be true for new nested directory")
	}

	info, err := os.Stat(nestedDir)
	if err != nil {
		t.Fatalf("expected directory to exist, got error: %v", err)
	}

	if !info.IsDir() {
		t.Error("expected path to be a directory")
	}
}

func TestGetOrCreateDir_ExistingDirectory(t *testing.T) {
	tempDir := t.TempDir()
	existingDir := filepath.Join(tempDir, "existing")

	err := os.Mkdir(existingDir, 0o755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	created, err := GetOrCreateDir(existingDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if created {
		t.Error("expected created to be false for existing directory")
	}
}

func TestGetOrCreateDir_PathIsFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "file.txt")

	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	f.Close()

	created, err := GetOrCreateDir(filePath)
	if err == nil {
		t.Fatal("expected error for file path, got nil")
	}

	if created {
		t.Error("expected created to be false when path is a file")
	}
}

func TestGetOrCreateDir_EmptyPath(t *testing.T) {
	created, err := GetOrCreateDir("")
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}

	if created {
		t.Error("expected created to be false for empty path")
	}
}

func TestGetOrCreateDir_TempDirItself(t *testing.T) {
	tempDir := t.TempDir()

	created, err := GetOrCreateDir(tempDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if created {
		t.Error("expected created to be false for existing temp directory")
	}
}

