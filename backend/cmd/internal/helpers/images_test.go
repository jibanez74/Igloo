package helpers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveTmdbImage(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "tmdb_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name        string
		tmdbUrl     string
		output      string
		fileName    string
		wantErr     bool
		errContains string
	}{
		{
			name:        "missing tmdbUrl",
			tmdbUrl:     "",
			output:      tempDir,
			fileName:    "test.jpg",
			wantErr:     true,
			errContains: "tmdbUrl, output, and fileName are required",
		},
		{
			name:        "missing output",
			tmdbUrl:     "https://image.tmdb.org/t/p/original/test.jpg",
			output:      "",
			fileName:    "test.jpg",
			wantErr:     true,
			errContains: "tmdbUrl, output, and fileName are required",
		},
		{
			name:        "missing fileName",
			tmdbUrl:     "https://image.tmdb.org/t/p/original/test.jpg",
			output:      tempDir,
			fileName:    "",
			wantErr:     true,
			errContains: "tmdbUrl, output, and fileName are required",
		},
		{
			name:        "invalid tmdbUrl",
			tmdbUrl:     "https://invalid-url.com/image.jpg",
			output:      tempDir,
			fileName:    "test.jpg",
			wantErr:     true,
			errContains: "failed to download image",
		},
		{
			name:     "successful download",
			tmdbUrl:  "https://image.tmdb.org/t/p/original/wwemzKWzjKYJFfCeiB57q3r4Bcm.png", // TMDB logo
			output:   tempDir,
			fileName: "tmdb_logo.jpg",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SaveTmdbImage(tt.tmdbUrl, tt.output, tt.fileName)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error message %q does not contain %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				} else {
					// Verify file was created
					filePath := filepath.Join(tt.output, tt.fileName)
					if _, err := os.Stat(filePath); os.IsNotExist(err) {
						t.Errorf("file was not created at %s", filePath)
					}
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
