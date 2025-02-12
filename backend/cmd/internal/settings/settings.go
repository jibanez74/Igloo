package settings

import (
	"fmt"
	"os"
	"path/filepath"
)

type Settings interface {
	DeleteConfigFile() error
	GetDebug() bool
	GetPort() string
	GetDownloadImages() bool
	GetTmdbKey() string
	GetJellyfinToken() string
	GetFfmpegPath() string
	GetFfprobePath() string
	GetStaticDir() string
	GetMoviesImgDir() string
	GetStudiosImgDir() string
	GetArtistsImgDir() string
	GetPostgresHost() string
	GetPostgresPort() string
	GetPostgresUser() string
	GetPostgresPass() string
	GetPostgresDB() string
	GetPostgresSslMode() string
	GetPostgresMaxConns() int
	GetRedisAddress() string
}

type settings struct {
	Port             string `json:"port"`
	Debug            bool   `json:"debug"`
	DownloadImages   bool   `json:"download_images"`
	TmdbKey          string `json:"tmdb_key"`
	JellyfinToken    string `json:"jellyfin_token"`
	FfmpegPath       string `json:"ffmpeg_path"`
	FfprobePath      string `json:"ffprobe_path"`
	StaticDir        string `json:"static_dir"`
	MoviesImgDir     string `json:"movies_img_dir"`
	StudiosImgDir    string `json:"studios_img_dir"`
	ArtistsImgDir    string `json:"artists_img_dir"`
	PostgresHost     string `json:"postgres_host"`
	PostgresPort     string `json:"postgres_port"`
	PostgresUser     string `json:"postgres_user"`
	PostgresPass     string `json:"postgres_pass"`
	PostgresDB       string `json:"postgres_db"`
	PostgresSslMode  string `json:"postgres_ssl_mode"`
	PostgresMaxConns int    `json:"postgres_max_conns"`
	RedisHost        string `json:"redis_host"`
	RedisPort        int    `json:"redis_port"`
}

func (s *settings) GetDebug() bool             { return s.Debug }
func (s *settings) GetPort() string            { return s.Port }
func (s *settings) GetDownloadImages() bool    { return s.DownloadImages }
func (s *settings) GetTmdbKey() string         { return s.TmdbKey }
func (s *settings) GetJellyfinToken() string   { return s.JellyfinToken }
func (s *settings) GetFfmpegPath() string      { return s.FfmpegPath }
func (s *settings) GetFfprobePath() string     { return s.FfprobePath }
func (s *settings) GetStaticDir() string       { return s.StaticDir }
func (s *settings) GetMoviesImgDir() string    { return s.MoviesImgDir }
func (s *settings) GetStudiosImgDir() string   { return s.StudiosImgDir }
func (s *settings) GetArtistsImgDir() string   { return s.ArtistsImgDir }
func (s *settings) GetPostgresHost() string    { return s.PostgresHost }
func (s *settings) GetPostgresPort() string    { return s.PostgresPort }
func (s *settings) GetPostgresUser() string    { return s.PostgresUser }
func (s *settings) GetPostgresPass() string    { return s.PostgresPass }
func (s *settings) GetPostgresDB() string      { return s.PostgresDB }
func (s *settings) GetPostgresSslMode() string { return s.PostgresSslMode }
func (s *settings) GetPostgresMaxConns() int   { return s.PostgresMaxConns }
func (s *settings) GetRedisAddress() string    { return fmt.Sprintf("%s:%d", s.RedisHost, s.RedisPort) }

func New() (Settings, error) {
	configDir := getConfigDir()
	configPath := filepath.Join(configDir, "config.json")

	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	_, err = os.Stat(configPath)
	if err == nil {
		return loadExistingConfig(configPath)
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to check config file: %w", err)
	}

	return createNewConfig(configPath)
}
