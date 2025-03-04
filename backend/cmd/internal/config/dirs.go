package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func (c *config) createDirs(sharePath string) error {
	transcodeDir := filepath.Join(sharePath, "transcode")

	_, err := os.Stat(transcodeDir)
	if err != nil {
		err = os.MkdirAll(filepath.Join(sharePath, "transcode"), 0755)
		if err != nil {
			return fmt.Errorf("failed to create transcode dir: %w", err)
		}
	}

	staticDir := filepath.Join(sharePath, "static")

	_, err = os.Stat(staticDir)
	if err != nil {
		err = os.MkdirAll(filepath.Join(sharePath, "static"), 0755)
		if err != nil {
			return fmt.Errorf("failed to create static dir: %w", err)
		}
	}

	c.Settings.TranscodeDir = transcodeDir
	c.Settings.StaticDir = staticDir

	imgDir := filepath.Join(staticDir, "images")

	c.Settings.MoviesDirList = os.Getenv("MOVIES_DIR_LIST")
	c.Settings.MoviesImgDir = filepath.Join(imgDir, "movies")
	c.Settings.MusicDirList = os.Getenv("MUSIC_DIR_LIST")
	c.Settings.TvshowsDirList = os.Getenv("TVSHOWS_DIR_LIST")
	c.Settings.StudiosImgDir = filepath.Join(imgDir, "studios")

	return nil
}
