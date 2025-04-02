package main

import (
	"igloo/cmd/internal/database"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitDirs(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Save original home directory
	originalHome := os.Getenv("HOME")

	// Set temporary home directory for testing
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	tests := []struct {
		name     string
		settings *database.CreateSettingsParams
		wantErr  bool
	}{
		{
			name: "successful directory creation",
			settings: &database.CreateSettingsParams{
				MoviesDirList:  "/test/movies",
				MusicDirList:   "/test/music",
				TvshowsDirList: "/test/tvshows",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{}
			err := app.initDirs(tt.settings)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify all required directories were created
			expectedDirs := []string{
				filepath.Join(tmpDir, ".local", "share", "igloo"),
				filepath.Join(tmpDir, ".local", "share", "igloo", "transcode"),
				filepath.Join(tmpDir, ".local", "share", "igloo", "static"),
				filepath.Join(tmpDir, ".local", "share", "igloo", "static", "images", "movies"),
				filepath.Join(tmpDir, ".local", "share", "igloo", "static", "images", "studios"),
				filepath.Join(tmpDir, ".local", "share", "igloo", "static", "images", "artists"),
				filepath.Join(tmpDir, ".local", "share", "igloo", "static", "images", "avatar"),
			}

			for _, dir := range expectedDirs {
				_, err := os.Stat(dir)
				assert.NoError(t, err, "Directory %s should exist", dir)
			}

			// Verify settings were updated correctly
			assert.Equal(t, filepath.Join(tmpDir, ".local", "share", "igloo", "transcode"), tt.settings.TranscodeDir)
			assert.Equal(t, filepath.Join(tmpDir, ".local", "share", "igloo", "static"), tt.settings.StaticDir)
			assert.Equal(t, "/test/movies", tt.settings.MoviesDirList)
			assert.Equal(t, filepath.Join(tmpDir, ".local", "share", "igloo", "static", "images", "movies"), tt.settings.MoviesImgDir)
			assert.Equal(t, "/test/music", tt.settings.MusicDirList)
			assert.Equal(t, "/test/tvshows", tt.settings.TvshowsDirList)
			assert.Equal(t, filepath.Join(tmpDir, ".local", "share", "igloo", "static", "images", "studios"), tt.settings.StudiosImgDir)
			assert.Equal(t, filepath.Join(tmpDir, ".local", "share", "igloo", "static", "images", "artists"), tt.settings.ArtistsImgDir)
			assert.Equal(t, filepath.Join(tmpDir, ".local", "share", "igloo", "static", "images", "avatar"), tt.settings.AvatarImgDir)
		})
	}
}

func TestInitDirs_NoHomeDir(t *testing.T) {
	// Temporarily unset HOME environment variable
	originalHome := os.Getenv("HOME")
	os.Unsetenv("HOME")
	defer os.Setenv("HOME", originalHome)

	app := &application{}
	settings := &database.CreateSettingsParams{}
	err := app.initDirs(settings)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get users home directory")
}
