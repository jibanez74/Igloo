-- users
CREATE TABLE
  IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    is_admin BOOLEAN NOT NULL DEFAULT false,
    avatar TEXT,
    preferred_hls_profile TEXT,
    download_mbps REAL,
    preferred_audio_language TEXT,
    preferred_subtitle_language TEXT,
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
    server_upload_mbps REAL,
    static_dir TEXT NOT NULL DEFAULT 'static',
    logs_dir TEXT NOT NULL DEFAULT 'logs',
    transcode_dir TEXT NOT NULL DEFAULT 'transcode',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
  );

CREATE UNIQUE INDEX IF NOT EXISTS idx_settings_singleton ON settings ((1));

CREATE TABLE
  IF NOT EXISTS app_metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
  );

-- musicians
CREATE TABLE
  IF NOT EXISTS musicians (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    sort_name TEXT NOT NULL,
    summary TEXT,
    spotify_id TEXT UNIQUE,
    spotify_popularity REAL,
    spotify_followers INTEGER,
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
    spotify_id TEXT UNIQUE,
    spotify_popularity REAL,
    musician TEXT,
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

-- track_musicians
CREATE TABLE
  IF NOT EXISTS track_musicians (
    track_id INTEGER NOT NULL,
    musician_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (track_id, musician_id),
    FOREIGN KEY (track_id) REFERENCES tracks (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (musician_id) REFERENCES musicians (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_track_musicians_track ON track_musicians (track_id);

CREATE INDEX IF NOT EXISTS idx_track_musicians_musician ON track_musicians (musician_id);

-- movies
CREATE TABLE
  IF NOT EXISTS movies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    file_path TEXT NOT NULL UNIQUE,
    file_name TEXT NOT NULL,
    size INTEGER NOT NULL,
    container TEXT NOT NULL CHECK (container IN ('mkv', 'mp4', 'avi', 'mov', 'm4v', 'webm')),
    mime_type TEXT NOT NULL,
    adult BOOLEAN NOT NULL,
    tmdb_id INTEGER,
    imdb_id TEXT,
    poster_path TEXT,
    backdrop_path TEXT,
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
    duration REAL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
  );

CREATE INDEX IF NOT EXISTS idx_movie_title ON movies (title);

CREATE INDEX IF NOT EXISTS idx_movies_tmdb_id ON movies (tmdb_id);

CREATE INDEX IF NOT EXISTS idx_movies_imdb_id ON movies (imdb_id);

-- movie_watch_progress
CREATE TABLE
  IF NOT EXISTS movie_watch_progress (
    user_id INTEGER NOT NULL,
    movie_id INTEGER NOT NULL,
    progress_sec REAL NOT NULL DEFAULT 0,
    duration_sec REAL NOT NULL DEFAULT 0,
    watched BOOLEAN NOT NULL DEFAULT false,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, movie_id),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_movie_watch_progress_user_updated_at
ON movie_watch_progress (user_id, updated_at DESC);

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
    pixel_format TEXT,
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

-- music_spotify_matches
CREATE TABLE
  IF NOT EXISTS music_spotify_matches (
    entity_type TEXT NOT NULL CHECK (entity_type IN ('album', 'musician')),
    entity_id INTEGER NOT NULL,
    spotify_id TEXT,
    status TEXT NOT NULL CHECK (status IN ('matched', 'failed', 'unmatched')),
    reason TEXT,
    score INTEGER,
    threshold_value INTEGER,
    candidate_name TEXT,
    candidate_artist TEXT,
    search_query TEXT,
    strategy TEXT,
    error TEXT,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (entity_type, entity_id)
  );

CREATE INDEX IF NOT EXISTS idx_music_spotify_matches_status
ON music_spotify_matches (entity_type, status);

CREATE TRIGGER IF NOT EXISTS music_spotify_matches_album_ad AFTER DELETE ON albums BEGIN
  DELETE FROM music_spotify_matches WHERE entity_type = 'album' AND entity_id = old.id;
END;

CREATE TRIGGER IF NOT EXISTS music_spotify_matches_musician_ad AFTER DELETE ON musicians BEGIN
  DELETE FROM music_spotify_matches WHERE entity_type = 'musician' AND entity_id = old.id;
END;

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

-- user_liked_movies (movie likes; mirrors user_liked_tracks — Phase 0 / movies page)
CREATE TABLE
  IF NOT EXISTS user_liked_movies (
    user_id INTEGER NOT NULL,
    movie_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, movie_id),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_user_liked_movies_user ON user_liked_movies (user_id);

CREATE INDEX IF NOT EXISTS idx_user_liked_movies_movie ON user_liked_movies (movie_id);

-- watch_rooms
CREATE TABLE
  IF NOT EXISTS watch_rooms (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_user_id INTEGER NOT NULL,
    movie_id INTEGER NOT NULL,
    playback_mode TEXT NOT NULL CHECK (
      playback_mode IN ('direct', 'remux', '2160p_16mbps', '1080p_8mbps', '1080p_6mbps', '1080p_4mbps', '720p_3mbps')
    ),
    audio_track INTEGER NOT NULL DEFAULT 0 CHECK (audio_track >= 0),
    subtitle_track INTEGER CHECK (subtitle_track >= 0),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (owner_user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_watch_rooms_owner ON watch_rooms (owner_user_id);

CREATE INDEX IF NOT EXISTS idx_watch_rooms_movie ON watch_rooms (movie_id);

-- watch_room_members
CREATE TABLE
  IF NOT EXISTS watch_room_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    room_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (room_id, user_id),
    FOREIGN KEY (room_id) REFERENCES watch_rooms (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_watch_room_members_room ON watch_room_members (room_id);

CREATE INDEX IF NOT EXISTS idx_watch_room_members_user ON watch_room_members (user_id);

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
    movie_id INTEGER,
    content_type TEXT NOT NULL DEFAULT 'track',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (content_type IN ('movie', 'track')),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE SET NULL ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_playlist_user ON playlists (user_id);

CREATE INDEX IF NOT EXISTS idx_playlist_content_type ON playlists (content_type);

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

-- playlist_movies (items for movie playlists; use with playlists.content_type = 'movie')
CREATE TABLE
  IF NOT EXISTS playlist_movies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    playlist_id INTEGER NOT NULL,
    movie_id INTEGER NOT NULL,
    position INTEGER NOT NULL,
    added_by INTEGER,
    added_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (playlist_id, movie_id),
    FOREIGN KEY (playlist_id) REFERENCES playlists (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (added_by) REFERENCES users (id) ON DELETE SET NULL ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_playlist_movies_playlist ON playlist_movies (playlist_id);

CREATE INDEX IF NOT EXISTS idx_playlist_movies_position ON playlist_movies (playlist_id, position);

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

-- FTS5 virtual tables for library search.
-- External-content tables: FTS stores the index and joins back to the source
-- table by rowid (= primary key id). Triggers below keep them in sync.
CREATE VIRTUAL TABLE IF NOT EXISTS movies_fts USING fts5 (
  title,
  overview,
  tag_line,
  content = 'movies',
  content_rowid = 'id',
  tokenize = 'unicode61 remove_diacritics 2'
);

CREATE TRIGGER IF NOT EXISTS movies_ai AFTER INSERT ON movies BEGIN
  INSERT INTO movies_fts (rowid, title, overview, tag_line)
  VALUES (new.id, new.title, new.overview, new.tag_line);
END;

CREATE TRIGGER IF NOT EXISTS movies_ad AFTER DELETE ON movies BEGIN
  INSERT INTO movies_fts (movies_fts, rowid, title, overview, tag_line)
  VALUES ('delete', old.id, old.title, old.overview, old.tag_line);
END;

CREATE TRIGGER IF NOT EXISTS movies_au AFTER UPDATE ON movies BEGIN
  INSERT INTO movies_fts (movies_fts, rowid, title, overview, tag_line)
  VALUES ('delete', old.id, old.title, old.overview, old.tag_line);
  INSERT INTO movies_fts (rowid, title, overview, tag_line)
  VALUES (new.id, new.title, new.overview, new.tag_line);
END;

CREATE VIRTUAL TABLE IF NOT EXISTS albums_fts USING fts5 (
  title,
  musician,
  content = 'albums',
  content_rowid = 'id',
  tokenize = 'unicode61 remove_diacritics 2'
);

CREATE TRIGGER IF NOT EXISTS albums_ai AFTER INSERT ON albums BEGIN
  INSERT INTO albums_fts (rowid, title, musician)
  VALUES (new.id, new.title, new.musician);
END;

CREATE TRIGGER IF NOT EXISTS albums_ad AFTER DELETE ON albums BEGIN
  INSERT INTO albums_fts (albums_fts, rowid, title, musician)
  VALUES ('delete', old.id, old.title, old.musician);
END;

CREATE TRIGGER IF NOT EXISTS albums_au AFTER UPDATE ON albums BEGIN
  INSERT INTO albums_fts (albums_fts, rowid, title, musician)
  VALUES ('delete', old.id, old.title, old.musician);
  INSERT INTO albums_fts (rowid, title, musician)
  VALUES (new.id, new.title, new.musician);
END;

CREATE VIRTUAL TABLE IF NOT EXISTS musicians_fts USING fts5 (
  name,
  sort_name,
  content = 'musicians',
  content_rowid = 'id',
  tokenize = 'unicode61 remove_diacritics 2'
);

CREATE TRIGGER IF NOT EXISTS musicians_ai AFTER INSERT ON musicians BEGIN
  INSERT INTO musicians_fts (rowid, name, sort_name)
  VALUES (new.id, new.name, new.sort_name);
END;

CREATE TRIGGER IF NOT EXISTS musicians_ad AFTER DELETE ON musicians BEGIN
  INSERT INTO musicians_fts (musicians_fts, rowid, name, sort_name)
  VALUES ('delete', old.id, old.name, old.sort_name);
END;

CREATE TRIGGER IF NOT EXISTS musicians_au AFTER UPDATE ON musicians BEGIN
  INSERT INTO musicians_fts (musicians_fts, rowid, name, sort_name)
  VALUES ('delete', old.id, old.name, old.sort_name);
  INSERT INTO musicians_fts (rowid, name, sort_name)
  VALUES (new.id, new.name, new.sort_name);
END;

CREATE VIRTUAL TABLE IF NOT EXISTS tracks_fts USING fts5 (
  title,
  content = 'tracks',
  content_rowid = 'id',
  tokenize = 'unicode61 remove_diacritics 2'
);

CREATE TRIGGER IF NOT EXISTS tracks_ai AFTER INSERT ON tracks BEGIN
  INSERT INTO tracks_fts (rowid, title)
  VALUES (new.id, new.title);
END;

CREATE TRIGGER IF NOT EXISTS tracks_ad AFTER DELETE ON tracks BEGIN
  INSERT INTO tracks_fts (tracks_fts, rowid, title)
  VALUES ('delete', old.id, old.title);
END;

CREATE TRIGGER IF NOT EXISTS tracks_au AFTER UPDATE ON tracks BEGIN
  INSERT INTO tracks_fts (tracks_fts, rowid, title)
  VALUES ('delete', old.id, old.title);
  INSERT INTO tracks_fts (rowid, title)
  VALUES (new.id, new.title);
END;

CREATE VIRTUAL TABLE IF NOT EXISTS tracks_search_fts USING fts5 (
  title,
  album_title,
  musician_name,
  tokenize = 'unicode61 remove_diacritics 2'
);

CREATE TRIGGER IF NOT EXISTS tracks_search_ai AFTER INSERT ON tracks BEGIN
  INSERT INTO tracks_search_fts (rowid, title, album_title, musician_name)
  SELECT
    new.id,
    new.title,
    a.title,
    TRIM(COALESCE(m.name, '') || ' ' || COALESCE(a.musician, ''))
  FROM (SELECT 1) AS seed
  LEFT JOIN albums AS a ON a.id = new.album_id
  LEFT JOIN musicians AS m ON m.id = new.musician_id;
END;

CREATE TRIGGER IF NOT EXISTS tracks_search_ad AFTER DELETE ON tracks BEGIN
  DELETE FROM tracks_search_fts WHERE rowid = old.id;
END;

CREATE TRIGGER IF NOT EXISTS tracks_search_au AFTER UPDATE ON tracks BEGIN
  DELETE FROM tracks_search_fts WHERE rowid = old.id;
  INSERT INTO tracks_search_fts (rowid, title, album_title, musician_name)
  SELECT
    new.id,
    new.title,
    a.title,
    TRIM(COALESCE(m.name, '') || ' ' || COALESCE(a.musician, ''))
  FROM (SELECT 1) AS seed
  LEFT JOIN albums AS a ON a.id = new.album_id
  LEFT JOIN musicians AS m ON m.id = new.musician_id;
END;

CREATE TRIGGER IF NOT EXISTS tracks_search_album_au AFTER UPDATE OF title, musician ON albums BEGIN
  DELETE FROM tracks_search_fts WHERE rowid IN (
    SELECT id FROM tracks WHERE album_id = new.id
  );
  INSERT INTO tracks_search_fts (rowid, title, album_title, musician_name)
  SELECT
    t.id,
    t.title,
    a.title,
    TRIM(COALESCE(m.name, '') || ' ' || COALESCE(a.musician, ''))
  FROM tracks AS t
  LEFT JOIN albums AS a ON a.id = t.album_id
  LEFT JOIN musicians AS m ON m.id = t.musician_id
  WHERE t.album_id = new.id;
END;

CREATE TRIGGER IF NOT EXISTS tracks_search_musician_au AFTER UPDATE OF name ON musicians BEGIN
  DELETE FROM tracks_search_fts WHERE rowid IN (
    SELECT id FROM tracks WHERE musician_id = new.id
  );
  INSERT INTO tracks_search_fts (rowid, title, album_title, musician_name)
  SELECT
    t.id,
    t.title,
    a.title,
    TRIM(COALESCE(m.name, '') || ' ' || COALESCE(a.musician, ''))
  FROM tracks AS t
  LEFT JOIN albums AS a ON a.id = t.album_id
  LEFT JOIN musicians AS m ON m.id = t.musician_id
  WHERE t.musician_id = new.id;
END;
