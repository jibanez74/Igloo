package helpers

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

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

func TestGetOrCreateDir_EmptyPath(t *testing.T) {
	created, err := GetOrCreateDir("")
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}

	if created {
		t.Error("expected created to be false for empty path")
	}
}

// The process umask clears bits from whatever mode is requested, so under the
// usual 022 a request for 0o777 would also come out as 0o755. Clearing the
// umask makes the mode Igloo asks for observable.
func TestGetOrCreateDir_CreatesWithDirMode(t *testing.T) {
	previousUmask := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(previousUmask) })

	tempDir := t.TempDir()
	newDir := filepath.Join(tempDir, "modes")

	_, err := GetOrCreateDir(newDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	info, err := os.Stat(newDir)
	if err != nil {
		t.Fatalf("expected directory to exist, got error: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0o755 {
		t.Errorf("created directory mode = %#o, want %#o", mode, 0o755)
	}
}

func TestGetOrCreateDir_ReportsCreationFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}

	tempDir := t.TempDir()
	readOnlyParent := filepath.Join(tempDir, "readonly")

	err := os.Mkdir(readOnlyParent, 0o500)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	t.Cleanup(func() { os.Chmod(readOnlyParent, 0o700) })

	created, err := GetOrCreateDir(filepath.Join(readOnlyParent, "child"))
	if err == nil {
		t.Fatal("expected an error creating under a read-only parent, got nil")
	}
	if created {
		t.Error("expected created to be false when creation fails")
	}
	if !strings.Contains(err.Error(), "failed to create directory") {
		t.Errorf("GetOrCreateDir error = %q, want it to report the creation failure", err)
	}
}

func TestValidateDir_AcceptsReadableDirectoryAndRejectsFileAndMissingPath(t *testing.T) {
	tempDir := t.TempDir()

	err := ValidateDir(tempDir)
	if err != nil {
		t.Fatalf("expected no error for a readable directory, got %v", err)
	}

	filePath := filepath.Join(tempDir, "file.txt")
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	f.Close()

	err = ValidateDir(filePath)
	if err == nil {
		t.Error("expected an error for a regular file, got nil")
	}

	err = ValidateDir(filepath.Join(tempDir, "missing"))
	if err == nil {
		t.Error("expected an error for a missing path, got nil")
	}
}

// An operator-fixable permission failure must be reported as one, not folded
// into the generic stat or open failure.
func TestValidateDir_ReportsPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}

	tempDir := t.TempDir()
	unreadable := filepath.Join(tempDir, "unreadable")

	err := os.Mkdir(unreadable, 0o000)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	t.Cleanup(func() { os.Chmod(unreadable, 0o700) })

	err = ValidateDir(unreadable)
	if err == nil {
		t.Fatal("expected an error for an unreadable directory, got nil")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("ValidateDir error = %q, want it to report permission denied", err)
	}

	_, err = GetOrCreateDir(unreadable)
	if err == nil {
		t.Fatal("expected an error from GetOrCreateDir for an unreadable directory, got nil")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("GetOrCreateDir error = %q, want it to report permission denied", err)
	}
}
