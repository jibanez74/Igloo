// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0

package database

import (
	"github.com/jackc/pgx/v5/pgtype"
)

type AudioStream struct {
	ID            int32              `json:"id"`
	CreatedAt     pgtype.Timestamptz `json:"created_at"`
	UpdatedAt     pgtype.Timestamptz `json:"updated_at"`
	Title         string             `json:"title"`
	Index         int32              `json:"index"`
	Codec         string             `json:"codec"`
	Channels      int32              `json:"channels"`
	ChannelLayout string             `json:"channel_layout"`
	Language      string             `json:"language"`
	MovieID       pgtype.Int4        `json:"movie_id"`
}

type CastList struct {
	ID        int32              `json:"id"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
	ArtistID  pgtype.Int4        `json:"artist_id"`
	MovieID   pgtype.Int4        `json:"movie_id"`
	Character string             `json:"character"`
	SortOrder int32              `json:"sort_order"`
}

type Chapter struct {
	ID          int32              `json:"id"`
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
	UpdatedAt   pgtype.Timestamptz `json:"updated_at"`
	Title       string             `json:"title"`
	StartTimeMs int32              `json:"start_time_ms"`
	Thumb       string             `json:"thumb"`
	MovieID     pgtype.Int4        `json:"movie_id"`
}

type CrewList struct {
	ID         int32              `json:"id"`
	CreatedAt  pgtype.Timestamptz `json:"created_at"`
	UpdatedAt  pgtype.Timestamptz `json:"updated_at"`
	ArtistID   pgtype.Int4        `json:"artist_id"`
	MovieID    pgtype.Int4        `json:"movie_id"`
	Job        string             `json:"job"`
	Department string             `json:"department"`
}

type DeviceCode struct {
	ID         int32              `json:"id"`
	CreatedAt  pgtype.Timestamptz `json:"created_at"`
	DeviceCode string             `json:"device_code"`
	UserCode   string             `json:"user_code"`
	ExpiresAt  pgtype.Timestamptz `json:"expires_at"`
	UserID     pgtype.Int4        `json:"user_id"`
	IsVerified bool               `json:"is_verified"`
}

type GlobalSetting struct {
	ID                         int32              `json:"id"`
	CreatedAt                  pgtype.Timestamptz `json:"created_at"`
	UpdatedAt                  pgtype.Timestamptz `json:"updated_at"`
	Port                       int32              `json:"port"`
	Debug                      bool               `json:"debug"`
	BaseUrl                    string             `json:"base_url"`
	MoviesDirList              string             `json:"movies_dir_list"`
	MoviesImgDir               string             `json:"movies_img_dir"`
	MusicDirList               string             `json:"music_dir_list"`
	TvshowsDirList             string             `json:"tvshows_dir_list"`
	TranscodeDir               string             `json:"transcode_dir"`
	StudiosImgDir              string             `json:"studios_img_dir"`
	ArtistsImgDir              string             `json:"artists_img_dir"`
	AvatarImgDir               string             `json:"avatar_img_dir"`
	StaticDir                  string             `json:"static_dir"`
	DownloadImages             bool               `json:"download_images"`
	TmdbApiKey                 string             `json:"tmdb_api_key"`
	FfmpegPath                 string             `json:"ffmpeg_path"`
	FfprobePath                string             `json:"ffprobe_path"`
	EnableHardwareAcceleration bool               `json:"enable_hardware_acceleration"`
	HardwareEncoder            string             `json:"hardware_encoder"`
	JellyfinToken              string             `json:"jellyfin_token"`
	Issuer                     string             `json:"issuer"`
	Audience                   string             `json:"audience"`
	Secret                     string             `json:"secret"`
	CookieDomain               string             `json:"cookie_domain"`
	CookiePath                 string             `json:"cookie_path"`
}

type Movie struct {
	ID              int32              `json:"id"`
	CreatedAt       pgtype.Timestamptz `json:"created_at"`
	UpdatedAt       pgtype.Timestamptz `json:"updated_at"`
	Title           string             `json:"title"`
	FilePath        string             `json:"file_path"`
	FileName        string             `json:"file_name"`
	Container       string             `json:"container"`
	Size            int64              `json:"size"`
	ContentType     string             `json:"content_type"`
	RunTime         int32              `json:"run_time"`
	Adult           bool               `json:"adult"`
	TagLine         string             `json:"tag_line"`
	Summary         string             `json:"summary"`
	Art             string             `json:"art"`
	Thumb           string             `json:"thumb"`
	TmdbID          string             `json:"tmdb_id"`
	ImdbID          string             `json:"imdb_id"`
	Year            int32              `json:"year"`
	ReleaseDate     pgtype.Date        `json:"release_date"`
	Budget          int64              `json:"budget"`
	Revenue         int64              `json:"revenue"`
	ContentRating   string             `json:"content_rating"`
	AudienceRating  float32            `json:"audience_rating"`
	CriticRating    float32            `json:"critic_rating"`
	SpokenLanguages string             `json:"spoken_languages"`
}

type MovieExtra struct {
	ID        int32              `json:"id"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
	Title     string             `json:"title"`
	Url       string             `json:"url"`
	Kind      string             `json:"kind"`
	MovieID   pgtype.Int4        `json:"movie_id"`
}

type Studio struct {
	ID        int32              `json:"id"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
	Name      string             `json:"name"`
	Country   string             `json:"country"`
	Logo      string             `json:"logo"`
	TmdbID    int32              `json:"tmdb_id"`
}

type Subtitle struct {
	ID        int32              `json:"id"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
	Title     string             `json:"title"`
	Index     int32              `json:"index"`
	Codec     string             `json:"codec"`
	Language  string             `json:"language"`
	MovieID   pgtype.Int4        `json:"movie_id"`
}

type User struct {
	ID        int32              `json:"id"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
	Name      string             `json:"name"`
	Email     string             `json:"email"`
	Username  string             `json:"username"`
	Password  string             `json:"password"`
	IsActive  bool               `json:"is_active"`
	IsAdmin   bool               `json:"is_admin"`
	Avatar    string             `json:"avatar"`
}

type VideoStream struct {
	ID             int32              `json:"id"`
	CreatedAt      pgtype.Timestamptz `json:"created_at"`
	UpdatedAt      pgtype.Timestamptz `json:"updated_at"`
	Title          string             `json:"title"`
	Index          int32              `json:"index"`
	Duration       string             `json:"duration"`
	Profile        string             `json:"profile"`
	AspectRatio    string             `json:"aspect_ratio"`
	BitRate        string             `json:"bit_rate"`
	BitDepth       string             `json:"bit_depth"`
	Codec          string             `json:"codec"`
	Width          int32              `json:"width"`
	Height         int32              `json:"height"`
	CodedWidth     int32              `json:"coded_width"`
	CodedHeight    int32              `json:"coded_height"`
	ColorTransfer  string             `json:"color_transfer"`
	ColorPrimaries string             `json:"color_primaries"`
	ColorSpace     string             `json:"color_space"`
	ColorRange     string             `json:"color_range"`
	FrameRate      string             `json:"frame_rate"`
	AvgFrameRate   string             `json:"avg_frame_rate"`
	Level          int32              `json:"level"`
	MovieID        pgtype.Int4        `json:"movie_id"`
}
