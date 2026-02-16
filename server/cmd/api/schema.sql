-- users
CREATE TABLE
  IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    is_admin BOOLEAN NOT NULL DEFAULT false,
    avatar TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
  );

CREATE INDEX IF NOT EXISTS idx_user_name ON users (name);

-- settings
CREATE TABLE
  IF NOT EXISTS settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tmdb_key TEXT,
    jellyfin_token TEXT,
    spotify_client_id TEXT,
    spotify_client_secret TEXT,
    hardware_acceleration_device TEXT CHECK (
      hardware_acceleration_device IN ('cpu', 'apple', 'nvidia', 'intel')
    ),
    enable_logger BOOLEAN NOT NULL DEFAULT false,
    enable_watcher BOOLEAN NOT NULL DEFAULT false,
    download_images BOOLEAN NOT NULL DEFAULT false,
    movies_dir TEXT,
    shows_dir TEXT,
    music_dir TEXT,
    static_dir TEXT NOT NULL DEFAULT 'static',
    logs_dir TEXT NOT NULL DEFAULT 'logs',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
  );

-- musicians
CREATE TABLE
  IF NOT EXISTS musicians (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    sort_name TEXT NOT NULL,
    summary TEXT,
    spotify_popularity REAL,
    spotify_followers INTEGER,
    spotify_id TEXT UNIQUE,
    thumb TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
  );

CREATE INDEX IF NOT EXISTS idx_musician_name ON musicians (name);

-- albums
CREATE TABLE
  IF NOT EXISTS albums (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    sort_title TEXT NOT NULL,
    musician TEXT,
    spotify_id TEXT UNIQUE,
    spotify_popularity REAL,
    release_date TEXT,
    year INTEGER,
    total_tracks INTEGER,
    cover TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (title, musician)
  );

CREATE INDEX IF NOT EXISTS idx_album_title ON albums (title);

-- tracks
CREATE TABLE
  IF NOT EXISTS tracks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    sort_title TEXT NOT NULL,
    file_path TEXT NOT NULL UNIQUE,
    file_name TEXT NOT NULL,
    container TEXT NOT NULL CHECK (container IN ('mp3', 'flac', 'm4a')),
    mime_type TEXT NOT NULL CHECK (
      mime_type IN ('audio/mpeg', 'audio/flac', 'audio/mp4')
    ),
    codec TEXT NOT NULL,
    size INTEGER NOT NULL,
    track_index INTEGER NOT NULL,
    duration INTEGER NOT NULL,
    disc INTEGER NOT NULL,
    channels TEXT NOT NULL,
    channel_layout TEXT NOT NULL,
    bit_rate INTEGER NOT NULL,
    profile TEXT NOT NULL,
    release_date TEXT,
    year INTEGER,
    composer TEXT,
    copyright TEXT,
    language TEXT,
    album_id INTEGER,
    musician_id INTEGER,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (album_id) REFERENCES albums (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (musician_id) REFERENCES musicians (id) ON DELETE SET NULL ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_track_title ON tracks (title);

CREATE INDEX IF NOT EXISTS idx_track_album ON tracks (album_id);

CREATE INDEX IF NOT EXISTS idx_track_musician ON tracks (musician_id);

-- movies
CREATE TABLE
  IF NOT EXISTS movies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    file_path TEXT NOT NULL UNIQUE,
    file_name TEXT NOT NULL,
    size INTEGER NOT NULL,
    container TEXT NOT NULL CHECK (container IN ('mkv', 'mp4', 'avi', 'webm')),
    mime_type TEXT NOT NULL,
    adult BOOLEAN NOT NULL,
    tmdb_id INTEGER,
    imdb_id TEXT,
    poster_path TEXT,
    language TEXT,
    year INTEGER,
    release_date TEXT,
    overview TEXT,
    tag_line TEXT,
    certification TEXT,
    critic_rating REAL,
    audience_rating REAL,
    revenue REAL,
    budget REAL,
    run_time INTEGER,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
  );

CREATE INDEX IF NOT EXISTS idx_movie_title ON movies (title);

CREATE INDEX IF NOT EXISTS idx_movies_tmdb_id ON movies (tmdb_id);

CREATE INDEX IF NOT EXISTS idx_movies_imdb_id ON movies (imdb_id);

-- production_companies
CREATE TABLE
  IF NOT EXISTS production_companies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    tmdb_id INTEGER NOT NULL UNIQUE,
    logo TEXT,
    country TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
  );

CREATE INDEX IF NOT EXISTS idx_production_company_name ON production_companies (name);

-- artist
CREATE TABLE
  IF NOT EXISTS artist (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    tmdb_id INTEGER NOT NULL UNIQUE,
    profile TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
  );

-- genres
CREATE TABLE
  IF NOT EXISTS genres (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tag TEXT NOT NULL,
    genre_type TEXT NOT NULL CHECK (genre_type IN ('movie', 'show', 'music')),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tag, genre_type)
  );

-- tables for extras for movies and tv shows
-- this include trailers, special features and others
CREATE TABLE
  IF NOT EXISTS extra_videos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    external_id TEXT UNIQUE,
    key TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('trailer', 'special_feature', 'other')),
    site TEXT NOT NULL CHECK (site IN ('youtube', 'vimeo', 'other')),
    official BOOLEAN NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
  );

CREATE INDEX IF NOT EXISTS idx_genre_tag ON genres (tag);

-- video_streams
CREATE TABLE
  IF NOT EXISTS video_streams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    movie_id INTEGER NOT NULL,
    stream_index INTEGER NOT NULL,
    codec TEXT NOT NULL,
    codec_profile TEXT,
    codec_level INTEGER,
    bit_rate INTEGER NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    coded_width INTEGER,
    coded_height INTEGER,
    aspect_ratio TEXT,
    frame_rate REAL NOT NULL,
    avg_frame_rate TEXT,
    bit_depth INTEGER,
    color_range TEXT,
    color_space TEXT,
    color_primaries TEXT,
    color_transfer TEXT,
    language TEXT,
    title TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_video_streams_movie ON video_streams (movie_id);

CREATE INDEX IF NOT EXISTS idx_video_streams_index ON video_streams (movie_id, stream_index);

-- audio_streams
CREATE TABLE
  IF NOT EXISTS audio_streams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    movie_id INTEGER NOT NULL,
    stream_index INTEGER NOT NULL,
    codec TEXT NOT NULL,
    codec_profile TEXT,
    bit_rate INTEGER NOT NULL,
    sample_rate INTEGER,
    channels INTEGER NOT NULL,
    channel_layout TEXT,
    language TEXT,
    title TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_audio_streams_movie ON audio_streams (movie_id);

CREATE INDEX IF NOT EXISTS idx_audio_streams_index ON audio_streams (movie_id, stream_index);

CREATE INDEX IF NOT EXISTS idx_audio_streams_language ON audio_streams (movie_id, language);

-- subtitles
CREATE TABLE
  IF NOT EXISTS subtitles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    movie_id INTEGER NOT NULL,
    stream_index INTEGER NOT NULL,
    codec TEXT NOT NULL,
    language TEXT,
    title TEXT,
    is_forced BOOLEAN NOT NULL DEFAULT false,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_subtitles_movie ON subtitles (movie_id);

CREATE INDEX IF NOT EXISTS idx_subtitles_index ON subtitles (movie_id, stream_index);

CREATE INDEX IF NOT EXISTS idx_subtitles_language ON subtitles (movie_id, language);

-- chapters
CREATE TABLE
  IF NOT EXISTS chapters (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    start_time INTEGER NOT NULL,
    thumb TEXT,
    movie_id INTEGER,
    FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

-- cast
CREATE TABLE
  IF NOT EXISTS cast(
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    movie_id INTEGER NOT NULL,
    artist_id INTEGER NOT NULL,
    character TEXT NOT NULL,
    cast_order INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (artist_id) REFERENCES artist (id) ON DELETE CASCADE ON UPDATE CASCADE,
    UNIQUE (movie_id, artist_id, cast_order)
  );

CREATE INDEX IF NOT EXISTS idx_cast_movie ON cast(movie_id);

CREATE INDEX IF NOT EXISTS idx_cast_artist ON cast(artist_id);

CREATE INDEX IF NOT EXISTS idx_cast_order ON cast(movie_id, cast_order);

-- crew
CREATE TABLE
  IF NOT EXISTS crew (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    movie_id INTEGER NOT NULL,
    artist_id INTEGER NOT NULL,
    job TEXT NOT NULL,
    department TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (artist_id) REFERENCES artist (id) ON DELETE CASCADE ON UPDATE CASCADE,
    UNIQUE (movie_id, artist_id, job, department)
  );

CREATE INDEX IF NOT EXISTS idx_crew_movie ON crew (movie_id);

CREATE INDEX IF NOT EXISTS idx_crew_artist ON crew (artist_id);

CREATE INDEX IF NOT EXISTS idx_crew_department ON crew (movie_id, department);

-- movie_production_companies
CREATE TABLE
  IF NOT EXISTS movie_production_companies (
    movie_id INTEGER NOT NULL,
    production_company_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (movie_id, production_company_id),
    FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (production_company_id) REFERENCES production_companies (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_movie_production_companies_movie ON movie_production_companies (movie_id);

CREATE INDEX IF NOT EXISTS idx_movie_production_companies_company ON movie_production_companies (production_company_id);

-- movie_genres
CREATE TABLE
  IF NOT EXISTS movie_genres (
    movie_id INTEGER NOT NULL,
    genre_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (movie_id, genre_id),
    FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (genre_id) REFERENCES genres (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_movie_genres_movie ON movie_genres (movie_id);

CREATE INDEX IF NOT EXISTS idx_movie_genres_genre ON movie_genres (genre_id);

-- movie_extra_videos: many-to-many between movies and extra_videos (trailers, special features).
-- One extra_video row is shared across movie rows that represent the same film (e.g. same tmdb_id).
CREATE TABLE
  IF NOT EXISTS movie_extra_videos (
    movie_id INTEGER NOT NULL,
    extra_video_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (movie_id, extra_video_id),
    FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (extra_video_id) REFERENCES extra_videos (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_movie_extra_videos_movie ON movie_extra_videos (movie_id);

CREATE INDEX IF NOT EXISTS idx_movie_extra_videos_extra ON movie_extra_videos (extra_video_id);

-- musician_genres
CREATE TABLE
  IF NOT EXISTS musician_genres (
    musician_id INTEGER NOT NULL,
    genre_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (musician_id, genre_id),
    FOREIGN KEY (musician_id) REFERENCES musicians (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (genre_id) REFERENCES genres (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_musician_genres_musician ON musician_genres (musician_id);

CREATE INDEX IF NOT EXISTS idx_musician_genres_genre ON musician_genres (genre_id);

-- musician_albums
CREATE TABLE
  IF NOT EXISTS musician_albums (
    musician_id INTEGER NOT NULL,
    album_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (musician_id, album_id),
    FOREIGN KEY (musician_id) REFERENCES musicians (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (album_id) REFERENCES albums (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_musician_albums_musician ON musician_albums (musician_id);

CREATE INDEX IF NOT EXISTS idx_musician_albums_album ON musician_albums (album_id);

-- track_genres
CREATE TABLE
  IF NOT EXISTS track_genres (
    track_id INTEGER NOT NULL,
    genre_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (track_id, genre_id),
    FOREIGN KEY (track_id) REFERENCES tracks (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (genre_id) REFERENCES genres (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_track_genres_track ON track_genres (track_id);

CREATE INDEX IF NOT EXISTS idx_track_genres_genre ON track_genres (genre_id);

-- album_genres
CREATE TABLE
  IF NOT EXISTS album_genres (
    album_id INTEGER NOT NULL,
    genre_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (album_id, genre_id),
    FOREIGN KEY (album_id) REFERENCES albums (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (genre_id) REFERENCES genres (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_album_genres_album ON album_genres (album_id);

CREATE INDEX IF NOT EXISTS idx_album_genres_genre ON album_genres (genre_id);

-- user_liked_tracks
CREATE TABLE
  IF NOT EXISTS user_liked_tracks (
    user_id INTEGER NOT NULL,
    track_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, track_id),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (track_id) REFERENCES tracks (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_user_liked_tracks_user ON user_liked_tracks (user_id);

CREATE INDEX IF NOT EXISTS idx_user_liked_tracks_track ON user_liked_tracks (track_id);

-- sessions
CREATE TABLE
  IF NOT EXISTS sessions (
    token TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    expiry REAL NOT NULL
  );

CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions (expiry);

-- playlists
CREATE TABLE
  IF NOT EXISTS playlists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    cover_image TEXT,
    is_public BOOLEAN NOT NULL DEFAULT false,
    folder_id INTEGER,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_playlist_user ON playlists (user_id);

CREATE INDEX IF NOT EXISTS idx_playlist_folder ON playlists (folder_id);

-- playlist_tracks
CREATE TABLE
  IF NOT EXISTS playlist_tracks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    playlist_id INTEGER NOT NULL,
    track_id INTEGER NOT NULL,
    position INTEGER NOT NULL,
    added_by INTEGER,
    added_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (playlist_id, track_id),
    FOREIGN KEY (playlist_id) REFERENCES playlists (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (track_id) REFERENCES tracks (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (added_by) REFERENCES users (id) ON DELETE SET NULL ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_playlist_tracks_playlist ON playlist_tracks (playlist_id);

CREATE INDEX IF NOT EXISTS idx_playlist_tracks_position ON playlist_tracks (playlist_id, position);

-- playlist_collaborators
CREATE TABLE
  IF NOT EXISTS playlist_collaborators (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    playlist_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    can_edit BOOLEAN NOT NULL DEFAULT true,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (playlist_id, user_id),
    FOREIGN KEY (playlist_id) REFERENCES playlists (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_playlist_collaborators_playlist ON playlist_collaborators (playlist_id);

CREATE INDEX IF NOT EXISTS idx_playlist_collaborators_user ON playlist_collaborators (user_id);

-- user_play_history
CREATE TABLE
  IF NOT EXISTS user_play_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    track_id INTEGER NOT NULL,
    played_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    duration_played INTEGER NOT NULL DEFAULT 0,
    completed BOOLEAN NOT NULL DEFAULT false,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (track_id) REFERENCES tracks (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_user_play_history_user ON user_play_history (user_id);

CREATE INDEX IF NOT EXISTS idx_user_play_history_track ON user_play_history (track_id);

CREATE INDEX IF NOT EXISTS idx_user_play_history_played_at ON user_play_history (user_id, played_at DESC);

-- user_track_stats
CREATE TABLE
  IF NOT EXISTS user_track_stats (
    user_id INTEGER NOT NULL,
    track_id INTEGER NOT NULL,
    play_count INTEGER NOT NULL DEFAULT 0,
    total_time_played INTEGER NOT NULL DEFAULT 0,
    last_played_at TEXT,
    first_played_at TEXT,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, track_id),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (track_id) REFERENCES tracks (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_user_track_stats_user ON user_track_stats (user_id);

CREATE INDEX IF NOT EXISTS idx_user_track_stats_play_count ON user_track_stats (user_id, play_count DESC);

CREATE INDEX IF NOT EXISTS idx_user_track_stats_last_played ON user_track_stats (user_id, last_played_at DESC);