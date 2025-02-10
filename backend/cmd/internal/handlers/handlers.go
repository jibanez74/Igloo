package handlers

import (
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/tmdb"
)

type HandlersConfig struct {
	Ffmpeg  ffmpeg.FFmpeg
	Ffprobe ffprobe.Ffprobe
	Tmdb    tmdb.Tmdb
	Queries *database.Queries
}

type handlers struct {
	ffmpegBin  ffmpeg.FFmpeg
	ffprobeBin ffprobe.Ffprobe
	tmdbClient tmdb.Tmdb
	queries    *database.Queries
}

func New(h *HandlersConfig) *handlers {
	return &handlers{
		ffmpegBin:  h.Ffmpeg,
		ffprobeBin: h.Ffprobe,
		tmdbClient: h.Tmdb,
		queries:    h.Queries,
	}
}
