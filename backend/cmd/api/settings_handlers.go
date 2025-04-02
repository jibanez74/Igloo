package main

import (
	"igloo/cmd/internal/database"

	"github.com/gofiber/fiber/v2"
)

func (app *application) getSettings(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"settings": fiber.Map{
			"movies_dir_list":    app.settings.MoviesDirList,
			"movies_img_dir":     app.settings.MoviesImgDir,
			"music_dir_list":     app.settings.MusicDirList,
			"tvshows_dir_list":   app.settings.TvshowsDirList,
			"transcode_dir":      app.settings.TranscodeDir,
			"studios_img_dir":    app.settings.StudiosImgDir,
			"artists_img_dir":    app.settings.ArtistsImgDir,
			"static_dir":         app.settings.StaticDir,
			"avatar_img_dir":     app.settings.AvatarImgDir,
			"download_images":    app.settings.DownloadImages,
			"tmdb_api_key":       app.settings.TmdbApiKey,
			"ffmpeg_path":        app.settings.FfmpegPath,
			"ffprobe_path":       app.settings.FfprobePath,
			"enable_transcoding": app.settings.EnableHardwareAcceleration,
			"hardware_encoder":   app.settings.HardwareEncoder,
			"jellyfin_token":     app.settings.JellyfinToken,
		},
	})
}

func (app *application) updateSettings(c *fiber.Ctx) error {
	var req database.UpdateSettingsParams

	err := c.BodyParser(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unable to parse request for updating settings",
		})
	}

	settings, err := app.queries.UpdateSettings(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to update settings",
		})
	}

	app.settings = &settings

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"settings": fiber.Map{
			"movies_dir_list":    app.settings.MoviesDirList,
			"movies_img_dir":     app.settings.MoviesImgDir,
			"music_dir_list":     app.settings.MusicDirList,
			"tvshows_dir_list":   app.settings.TvshowsDirList,
			"transcode_dir":      app.settings.TranscodeDir,
			"studios_img_dir":    app.settings.StudiosImgDir,
			"artists_img_dir":    app.settings.ArtistsImgDir,
			"static_dir":         app.settings.StaticDir,
			"avatar_img_dir":     app.settings.AvatarImgDir,
			"download_images":    app.settings.DownloadImages,
			"tmdb_api_key":       app.settings.TmdbApiKey,
			"ffmpeg_path":        app.settings.FfmpegPath,
			"ffprobe_path":       app.settings.FfprobePath,
			"enable_transcoding": app.settings.EnableHardwareAcceleration,
			"hardware_encoder":   app.settings.HardwareEncoder,
			"jellyfin_token":     app.settings.JellyfinToken,
		},
	})
}
