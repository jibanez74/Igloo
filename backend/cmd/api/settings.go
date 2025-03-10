package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func (app *application) initSettings() error {
	count, err := app.queries.GetSettingsCount(context.Background())
	if err != nil {
		return fmt.Errorf("unable to determine settings count: %w", err)
	}

	var settings database.GlobalSetting

	if count == 0 {
		var s database.CreateSettingsParams

		port, err := strconv.Atoi(os.Getenv("PORT"))
		if err != nil {
			port = 8080
		}
		s.Port = int32(port)

		debug, err := strconv.ParseBool(os.Getenv("DEBUG"))
		if err != nil {
			debug = true
		}
		s.Debug = debug

		s.BaseUrl = os.Getenv("BASE_URL")
		if s.BaseUrl == "" {
			s.BaseUrl = "localhost"
		}

		downloadImages, err := strconv.ParseBool(os.Getenv("DOWNLOAD_IMAGES"))
		if err != nil {
			downloadImages = false
		}
		s.DownloadImages = downloadImages

		s.Audience = os.Getenv("AUDIENCE")
		if s.Audience == "" {
			s.Audience = "igloo"
		}

		s.CookieDomain = os.Getenv("COOKIE_DOMAIN")
		if s.CookieDomain == "" {
			s.CookieDomain = "localhost"
		}

		s.CookiePath = os.Getenv("COOKIE_PATH")
		if s.CookiePath == "" {
			s.CookiePath = "/"
		}

		s.Issuer = os.Getenv("ISSUER")
		if s.Issuer == "" {
			s.Issuer = "igloo"
		}

		s.Secret = os.Getenv("SECRET")
		if s.Secret == "" {
			s.Secret = fmt.Sprintf("secret_igloo_%d", time.Now().UnixNano())
		}

		s.FfmpegPath = os.Getenv("FFMPEG_PATH")
		s.HardwareAcceleration = os.Getenv("HARDWARE_ACCELERATION")
		s.FfprobePath = os.Getenv("FFPROBE_PATH")
		s.TmdbApiKey = os.Getenv("TMDB_API_KEY")
		s.JellyfinToken = os.Getenv("JELLYFIN_TOKEN")

		settings, err = app.queries.CreateSettings(context.Background(), s)
		if err != nil {
			return fmt.Errorf("failed to create settings: %w", err)
		}
	} else {
		settings, err = app.queries.GetSettings(context.Background())
		if err != nil {
			return fmt.Errorf("failed to get settings: %w", err)
		}
	}

	app.settings = &settings

	err = app.createDirs()
	if err != nil {
		return fmt.Errorf("failed to create dirs: %w", err)
	}

	return nil
}

func (app *application) createDirs() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get users home directory: %w", err)
	}

	sharePath := filepath.Join(homeDir, ".local", "share")

	transcodeDir := filepath.Join(sharePath, "transcode")

	_, err = os.Stat(transcodeDir)
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

	app.settings.TranscodeDir = transcodeDir
	app.settings.StaticDir = staticDir

	imgDir := filepath.Join(staticDir, "images")

	app.settings.MoviesDirList = os.Getenv("MOVIES_DIR_LIST")
	app.settings.MoviesImgDir = filepath.Join(imgDir, "movies")
	app.settings.MusicDirList = os.Getenv("MUSIC_DIR_LIST")
	app.settings.TvshowsDirList = os.Getenv("TVSHOWS_DIR_LIST")
	app.settings.StudiosImgDir = filepath.Join(imgDir, "studios")
	app.settings.ArtistsImgDir = filepath.Join(imgDir, "artists")

	return nil

}
