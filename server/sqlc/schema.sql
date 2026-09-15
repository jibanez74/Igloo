-- Startup reapplies this schema; object creation and seed inserts must remain idempotent.

-- Accounts, settings, and authentication

-- Playback preferences are device-specific and stored in the browser.
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  email TEXT NOT NULL UNIQUE,
  password TEXT NOT NULL,
  is_admin BOOLEAN NOT NULL DEFAULT false,
  avatar TEXT,
  pin TEXT CHECK (pin GLOB '[0-9][0-9][0-9][0-9]'),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_name ON users (name);

CREATE TABLE IF NOT EXISTS settings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tmdb_key TEXT,
  immich_base_url TEXT,
  immich_api_key TEXT,
  jellyfin_base_url TEXT,
  jellyfin_api_key TEXT,
  spotify_client_id TEXT,
  spotify_client_secret TEXT,
  hardware_acceleration_device TEXT CHECK (
    hardware_acceleration_device IN ('cpu', 'apple', 'nvidia', 'intel')
  ),
  enable_watcher BOOLEAN NOT NULL DEFAULT false,
  download_images BOOLEAN NOT NULL DEFAULT false,
  movies_dir TEXT,
  shows_dir TEXT,
  music_dir TEXT,
  server_upload_mbps REAL,
  static_dir TEXT NOT NULL DEFAULT 'static',
  transcode_dir TEXT NOT NULL DEFAULT 'transcode'
);

-- The constant expression limits application settings to a single row.
CREATE UNIQUE INDEX IF NOT EXISTS idx_settings_singleton ON settings ((1));

CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY,
  data BLOB NOT NULL,
  expiry REAL NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions (expiry);

-- Long-lived bearer tokens are stored as hashes.
CREATE TABLE IF NOT EXISTS devices (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  platform TEXT NOT NULL DEFAULT '',
  app_version TEXT,
  token_hash TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_used_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_devices_user_id ON devices (user_id);

CREATE INDEX IF NOT EXISTS idx_devices_last_used_at ON devices (last_used_at);

-- Shared catalog metadata

CREATE TABLE IF NOT EXISTS production_companies (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  tmdb_id INTEGER NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS artist (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  tmdb_id INTEGER NOT NULL UNIQUE,
  profile TEXT
);

CREATE TABLE IF NOT EXISTS genres (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tag TEXT NOT NULL,
  genre_type TEXT NOT NULL CHECK (genre_type IN ('movie', 'show', 'music')),
  UNIQUE (tag, genre_type)
);

CREATE TABLE IF NOT EXISTS extra_videos (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  external_id TEXT UNIQUE,
  key TEXT NOT NULL,
  type TEXT NOT NULL CHECK (type IN ('trailer', 'special_feature', 'other')),
  site TEXT NOT NULL CHECK (site IN ('youtube', 'vimeo', 'other'))
);

-- Music catalog and scanner metadata

CREATE TABLE IF NOT EXISTS musicians (
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

-- Keep this expression aligned with GetMusiciansAlphabetical in queries/musicians.sql.
CREATE INDEX IF NOT EXISTS idx_musicians_alpha ON musicians (
  CASE
    WHEN UPPER(SUBSTR(sort_name, 1, 1)) BETWEEN 'A' AND 'Z' THEN UPPER(SUBSTR(sort_name, 1, 1))
    ELSE '#'
  END,
  sort_name
);

CREATE TABLE IF NOT EXISTS albums (
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
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- NULL and empty album artists share one identity. Keep UpsertAlbum's conflict
-- target in queries/albums.sql aligned with this expression.
CREATE UNIQUE INDEX IF NOT EXISTS idx_albums_title_musician ON albums (title, COALESCE(musician, ''));

-- Keep this expression aligned with GetAlbumsAlphabetical in queries/albums.sql.
CREATE INDEX IF NOT EXISTS idx_albums_alpha ON albums (
  CASE
    WHEN UPPER(SUBSTR(title, 1, 1)) BETWEEN 'A' AND 'Z' THEN UPPER(SUBSTR(title, 1, 1))
    ELSE '#'
  END,
  UPPER(title)
);

CREATE INDEX IF NOT EXISTS idx_albums_created_at ON albums (created_at DESC);

CREATE TABLE IF NOT EXISTS tracks (
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

CREATE INDEX IF NOT EXISTS idx_track_album ON tracks (album_id, disc, track_index);

CREATE INDEX IF NOT EXISTS idx_track_musician ON tracks (musician_id);

-- Keep this expression aligned with GetTracksAlphabetical in queries/tracks.sql.
CREATE INDEX IF NOT EXISTS idx_track_alpha ON tracks (
  CASE
    WHEN UPPER(SUBSTR(title, 1, 1)) BETWEEN 'A' AND 'Z' THEN UPPER(SUBSTR(title, 1, 1))
    ELSE '#'
  END,
  UPPER(title)
);

-- Supports reconciliation of album dates from current track contributions.
CREATE INDEX IF NOT EXISTS music_track_dates ON tracks (album_id, release_date);

CREATE TABLE IF NOT EXISTS track_musicians (
  track_id INTEGER NOT NULL,
  musician_id INTEGER NOT NULL,
  PRIMARY KEY (track_id, musician_id),
  FOREIGN KEY (track_id) REFERENCES tracks (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (musician_id) REFERENCES musicians (id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- track_id second so MusicArtistTrackMetadata pages an artist's tracks in
-- track_id order without a temporary sort.
CREATE INDEX IF NOT EXISTS idx_track_musicians_musician_track ON track_musicians (musician_id, track_id);

-- Local and Spotify genre contributions coexist; reconciliation removes only
-- the affected source. The same provenance rule applies to album_genres.
CREATE TABLE IF NOT EXISTS musician_genres (
  musician_id INTEGER NOT NULL,
  genre_id INTEGER NOT NULL,
  source TEXT NOT NULL DEFAULT 'local' CHECK (source IN ('local', 'spotify')),
  PRIMARY KEY (musician_id, genre_id, source),
  FOREIGN KEY (musician_id) REFERENCES musicians (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (genre_id) REFERENCES genres (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE IF NOT EXISTS musician_albums (
  musician_id INTEGER NOT NULL,
  album_id INTEGER NOT NULL,
  PRIMARY KEY (musician_id, album_id),
  FOREIGN KEY (musician_id) REFERENCES musicians (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (album_id) REFERENCES albums (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_musician_albums_album ON musician_albums (album_id);

CREATE TABLE IF NOT EXISTS track_genres (
  track_id INTEGER NOT NULL,
  genre_id INTEGER NOT NULL,
  PRIMARY KEY (track_id, genre_id),
  FOREIGN KEY (track_id) REFERENCES tracks (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (genre_id) REFERENCES genres (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE IF NOT EXISTS album_genres (
  album_id INTEGER NOT NULL,
  genre_id INTEGER NOT NULL,
  source TEXT NOT NULL DEFAULT 'local' CHECK (source IN ('local', 'spotify')),
  PRIMARY KEY (album_id, genre_id, source),
  FOREIGN KEY (album_id) REFERENCES albums (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (genre_id) REFERENCES genres (id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- Scanner aliases use Go's Unicode trim/lower normalization; display spelling
-- and explicit sort tags do not define identity. Album and genre aliases use
-- the same normalization.
CREATE TABLE IF NOT EXISTS music_artist_identity (
  identity_key TEXT PRIMARY KEY,
  musician_id INTEGER NOT NULL REFERENCES musicians (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS music_artist_identity_owner ON music_artist_identity (musician_id);

CREATE TABLE IF NOT EXISTS music_album_identity (
  title_key TEXT NOT NULL,
  artist_key TEXT NOT NULL,
  album_id INTEGER NOT NULL REFERENCES albums (id) ON DELETE CASCADE,
  PRIMARY KEY (title_key, artist_key)
);

CREATE INDEX IF NOT EXISTS music_album_identity_owner ON music_album_identity (album_id);

CREATE TABLE IF NOT EXISTS music_genre_identity (
  identity_key TEXT PRIMARY KEY,
  genre_id INTEGER NOT NULL REFERENCES genres (id) ON DELETE CASCADE
);

-- Original tags support enrichment retries and credit reconciliation
-- without probing files again.
CREATE TABLE IF NOT EXISTS music_track_metadata (
  track_id INTEGER PRIMARY KEY REFERENCES tracks (id) ON DELETE CASCADE,
  artist_tag TEXT NOT NULL,
  artist_key TEXT NOT NULL,
  artist_sort TEXT NOT NULL,
  album_sort TEXT NOT NULL
);

-- Explicit credit sort values contribute at most one vote per track and artist.
CREATE TABLE IF NOT EXISTS music_credit_metadata (
  track_id INTEGER NOT NULL REFERENCES tracks (id) ON DELETE CASCADE,
  musician_id INTEGER NOT NULL REFERENCES musicians (id) ON DELETE CASCADE,
  sort_name TEXT NOT NULL,
  FOREIGN KEY (track_id, musician_id) REFERENCES track_musicians (track_id, musician_id) ON DELETE CASCADE,
  PRIMARY KEY (track_id, musician_id, sort_name)
);

CREATE INDEX IF NOT EXISTS music_credit_metadata_votes ON music_credit_metadata (musician_id, sort_name, track_id);

-- Spotify dates are a fallback when no valid local track date remains.
CREATE TABLE IF NOT EXISTS music_album_metadata (
  album_id INTEGER PRIMARY KEY REFERENCES albums (id) ON DELETE CASCADE,
  spotify_date TEXT
);

-- Cached Spotify outcomes survive rescans. Entity type determines the owner;
-- maintenance triggers remove entries when that owner is deleted.
CREATE TABLE IF NOT EXISTS music_spotify_matches (
  entity_type TEXT NOT NULL CHECK (entity_type IN ('album', 'musician')),
  entity_id INTEGER NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('matched', 'failed', 'unmatched')),
  reason TEXT,
  PRIMARY KEY (entity_type, entity_id)
);

-- Successful scanner baselines; size remains on the catalog row. Filesystem
-- identifiers are unsigned decimal text and SHA-256 is a 32-byte blob.
CREATE TABLE IF NOT EXISTS track_file_fingerprints (
  track_id INTEGER PRIMARY KEY NOT NULL REFERENCES tracks (id) ON DELETE CASCADE,
  mtime_ns INTEGER NOT NULL,
  ctime_ns INTEGER NOT NULL,
  device TEXT NOT NULL,
  inode TEXT NOT NULL,
  sha256 BLOB NOT NULL CHECK (typeof(sha256) = 'blob' AND length(sha256) = 32)
);

-- Movies and playback metadata

CREATE TABLE IF NOT EXISTS movies (
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

CREATE INDEX IF NOT EXISTS idx_movies_tmdb_id ON movies (tmdb_id);

-- Keep this expression and tie-breaker aligned with GetMoviesLibraryAsc/Desc.
CREATE INDEX IF NOT EXISTS idx_movies_title ON movies (LOWER(title), id);

CREATE INDEX IF NOT EXISTS idx_movies_created_at ON movies (created_at DESC);

-- Stream indices are absolute ffprobe indices, including gaps from excluded streams.
CREATE TABLE IF NOT EXISTS video_streams (
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
  -- ffprobe tt/bb/tb/bt values indicate interlacing; NULL is treated as progressive.
  field_order TEXT,
  -- No display matrix is NULL; an explicit zero-degree matrix is 0.
  rotation INTEGER,
  language TEXT,
  title TEXT,
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_video_streams_movie_stream ON video_streams (movie_id, stream_index);

CREATE TABLE IF NOT EXISTS audio_streams (
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
  is_default BOOLEAN NOT NULL DEFAULT false,
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_audio_streams_movie_stream ON audio_streams (movie_id, stream_index);

CREATE TABLE IF NOT EXISTS subtitles (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  movie_id INTEGER NOT NULL,
  stream_index INTEGER NOT NULL,
  codec TEXT NOT NULL,
  language TEXT,
  title TEXT,
  is_forced BOOLEAN NOT NULL DEFAULT false,
  is_default BOOLEAN NOT NULL DEFAULT false,
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_subtitles_movie_stream ON subtitles (movie_id, stream_index);

-- Fingerprints cover file identity, stream properties, and the output producer.
-- Only definitive preflight verdicts are cached; transient failures are not.
CREATE TABLE IF NOT EXISTS remux_safety_verdicts (
  movie_id INTEGER NOT NULL,
  stream_index INTEGER NOT NULL,
  fingerprint TEXT NOT NULL,
  safe BOOLEAN NOT NULL,
  reason TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (movie_id, stream_index),
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- File-identity fingerprints invalidate cached container seek data. keyframes
-- is a JSON array of ascending presentation times in seconds.
CREATE TABLE IF NOT EXISTS keyframe_indexes (
  movie_id INTEGER NOT NULL,
  stream_index INTEGER NOT NULL,
  fingerprint TEXT NOT NULL,
  duration_sec REAL NOT NULL,
  keyframes TEXT NOT NULL,
  PRIMARY KEY (movie_id, stream_index),
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE IF NOT EXISTS chapters (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  start_time INTEGER NOT NULL,
  thumb TEXT,
  movie_id INTEGER NOT NULL,
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_chapters_movie ON chapters (movie_id, start_time);

CREATE TABLE IF NOT EXISTS cast (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  movie_id INTEGER NOT NULL,
  artist_id INTEGER NOT NULL,
  character TEXT NOT NULL,
  cast_order INTEGER NOT NULL,
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (artist_id) REFERENCES artist (id) ON DELETE CASCADE ON UPDATE CASCADE,
  UNIQUE (movie_id, artist_id, cast_order)
);

CREATE INDEX IF NOT EXISTS idx_cast_order ON cast (movie_id, cast_order);

CREATE TABLE IF NOT EXISTS crew (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  movie_id INTEGER NOT NULL,
  artist_id INTEGER NOT NULL,
  job TEXT NOT NULL,
  department TEXT NOT NULL,
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (artist_id) REFERENCES artist (id) ON DELETE CASCADE ON UPDATE CASCADE,
  UNIQUE (movie_id, artist_id, job, department)
);

CREATE INDEX IF NOT EXISTS idx_crew_movie_department_job ON crew (movie_id, department, job);

CREATE TABLE IF NOT EXISTS movie_production_companies (
  movie_id INTEGER NOT NULL,
  production_company_id INTEGER NOT NULL,
  PRIMARY KEY (movie_id, production_company_id),
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (production_company_id) REFERENCES production_companies (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE IF NOT EXISTS movie_genres (
  movie_id INTEGER NOT NULL,
  genre_id INTEGER NOT NULL,
  PRIMARY KEY (movie_id, genre_id),
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (genre_id) REFERENCES genres (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_movie_genres_genre ON movie_genres (genre_id);

-- An extra video may be shared by catalog rows representing the same film.
CREATE TABLE IF NOT EXISTS movie_extra_videos (
  movie_id INTEGER NOT NULL,
  extra_video_id INTEGER NOT NULL,
  PRIMARY KEY (movie_id, extra_video_id),
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (extra_video_id) REFERENCES extra_videos (id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- Successful scanner baselines; size remains on the catalog row. Filesystem
-- identifiers are unsigned decimal text and SHA-256 is a 32-byte blob.
CREATE TABLE IF NOT EXISTS movie_file_fingerprints (
  movie_id INTEGER PRIMARY KEY NOT NULL REFERENCES movies (id) ON DELETE CASCADE,
  mtime_ns INTEGER NOT NULL,
  ctime_ns INTEGER NOT NULL,
  device TEXT NOT NULL,
  inode TEXT NOT NULL
);

-- Pending descriptive enrichment does not invalidate usable technical media data.
-- attempts counts definitive TMDB misses; last_attempt_at (unix seconds) drives
-- the scanner's no-match backoff. A technical rescan resets both.
CREATE TABLE IF NOT EXISTS movie_tmdb_retries (
  movie_id INTEGER PRIMARY KEY NOT NULL REFERENCES movies (id) ON DELETE CASCADE,
  attempts INTEGER NOT NULL DEFAULT 0,
  last_attempt_at INTEGER
);

-- User activity

-- save_session_id and save_sequence order progress writes within a playback session.
CREATE TABLE IF NOT EXISTS movie_watch_progress (
  user_id INTEGER NOT NULL,
  movie_id INTEGER NOT NULL,
  progress_sec REAL NOT NULL DEFAULT 0,
  duration_sec REAL NOT NULL DEFAULT 0,
  watched BOOLEAN NOT NULL DEFAULT false,
  save_session_id TEXT NOT NULL DEFAULT '',
  save_sequence INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, movie_id),
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_movie_watch_progress_user_updated_at ON movie_watch_progress (user_id, updated_at DESC)
WHERE watched = false;

CREATE INDEX IF NOT EXISTS idx_movie_watch_progress_movie ON movie_watch_progress (movie_id);

CREATE TABLE IF NOT EXISTS user_liked_tracks (
  user_id INTEGER NOT NULL,
  track_id INTEGER NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, track_id),
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (track_id) REFERENCES tracks (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_liked_tracks_track ON user_liked_tracks (track_id);

CREATE INDEX IF NOT EXISTS idx_user_liked_tracks_user_created ON user_liked_tracks (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS user_liked_movies (
  user_id INTEGER NOT NULL,
  movie_id INTEGER NOT NULL,
  PRIMARY KEY (user_id, movie_id),
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_liked_movies_movie ON user_liked_movies (movie_id);

CREATE TABLE IF NOT EXISTS user_play_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  track_id INTEGER NOT NULL,
  played_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  duration_played INTEGER NOT NULL DEFAULT 0,
  completed BOOLEAN NOT NULL DEFAULT false,
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (track_id) REFERENCES tracks (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_play_history_track ON user_play_history (track_id);

CREATE INDEX IF NOT EXISTS idx_user_play_history_played_at ON user_play_history (user_id, played_at DESC);

CREATE TABLE IF NOT EXISTS user_track_stats (
  user_id INTEGER NOT NULL,
  track_id INTEGER NOT NULL,
  play_count INTEGER NOT NULL DEFAULT 0,
  total_time_played INTEGER NOT NULL DEFAULT 0,
  last_played_at TEXT,
  PRIMARY KEY (user_id, track_id),
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (track_id) REFERENCES tracks (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_track_stats_play_count ON user_track_stats (user_id, play_count DESC);

CREATE INDEX IF NOT EXISTS idx_user_track_stats_track ON user_track_stats (track_id);

-- Playlists

CREATE TABLE IF NOT EXISTS playlists (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  description TEXT,
  cover_image TEXT,
  is_public BOOLEAN NOT NULL DEFAULT false,
  movie_id INTEGER,
  content_type TEXT NOT NULL DEFAULT 'track',
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CHECK (content_type IN ('movie', 'track')),
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE SET NULL ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_playlist_user_content ON playlists (user_id, content_type);

CREATE INDEX IF NOT EXISTS idx_playlists_movie ON playlists (movie_id);

CREATE TABLE IF NOT EXISTS playlist_tracks (
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

CREATE INDEX IF NOT EXISTS idx_playlist_tracks_position ON playlist_tracks (playlist_id, position);

CREATE INDEX IF NOT EXISTS idx_playlist_tracks_track ON playlist_tracks (track_id);

CREATE INDEX IF NOT EXISTS idx_playlist_tracks_added_by ON playlist_tracks (added_by);

CREATE TABLE IF NOT EXISTS playlist_movies (
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

CREATE INDEX IF NOT EXISTS idx_playlist_movies_position ON playlist_movies (playlist_id, position);

CREATE INDEX IF NOT EXISTS idx_playlist_movies_movie ON playlist_movies (movie_id);

CREATE INDEX IF NOT EXISTS idx_playlist_movies_added_by ON playlist_movies (added_by);

CREATE TABLE IF NOT EXISTS playlist_collaborators (
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

CREATE INDEX IF NOT EXISTS idx_playlist_collaborators_user ON playlist_collaborators (user_id);

-- Watch rooms

CREATE TABLE IF NOT EXISTS watch_rooms (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  owner_user_id INTEGER NOT NULL,
  movie_id INTEGER NOT NULL,
  playback_mode TEXT NOT NULL CHECK (
    playback_mode IN ('direct', 'remux', '2160p_16mbps', '1080p_8mbps', '1080p_6mbps', '1080p_4mbps', '720p_3mbps')
  ),
  audio_track INTEGER NOT NULL DEFAULT 0 CHECK (audio_track >= 0),
  subtitle_track INTEGER CHECK (subtitle_track >= 0),
  -- Pin selected ordinals to stream identity so rescans detect reordered tracks.
  -- NULL means no stream identity was pinned.
  audio_stream_index INTEGER,
  audio_language TEXT,
  subtitle_stream_index INTEGER,
  subtitle_language TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (owner_user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_watch_rooms_owner ON watch_rooms (owner_user_id);

CREATE INDEX IF NOT EXISTS idx_watch_rooms_movie ON watch_rooms (movie_id);

CREATE TABLE IF NOT EXISTS watch_room_members (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  room_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (room_id, user_id),
  FOREIGN KEY (room_id) REFERENCES watch_rooms (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_watch_room_members_user ON watch_room_members (user_id);

-- Notifications

-- Request-queue visibility is admin-only and enforced by handlers, not per-user targeting.
CREATE TABLE IF NOT EXISTS notifications (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_by_user_id INTEGER NOT NULL,
  title TEXT NOT NULL CHECK (title IN ('movie_request', 'album_request', 'track_request', 'other')),
  message TEXT NOT NULL,
  is_admin BOOLEAN NOT NULL DEFAULT false,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (created_by_user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_notifications_created_by_user ON notifications (created_by_user_id);

CREATE INDEX IF NOT EXISTS idx_notifications_admin_created_at ON notifications (is_admin, created_at DESC);

CREATE TABLE IF NOT EXISTS notification_reads (
  notification_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  PRIMARY KEY (notification_id, user_id),
  FOREIGN KEY (notification_id) REFERENCES notifications (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_notification_reads_user ON notification_reads (user_id);

-- Search infrastructure

-- Generations invalidate the bounded in-memory typo indexes. Seed inserts must
-- preserve existing generations when startup reapplies this schema.
CREATE TABLE IF NOT EXISTS search_vocab_generations (
  vocab_table TEXT PRIMARY KEY,
  generation INTEGER NOT NULL DEFAULT 0
);

INSERT OR IGNORE INTO search_vocab_generations (vocab_table) VALUES
  ('movies_fts_vocab'),
  ('albums_fts_vocab'),
  ('musicians_fts_vocab'),
  ('tracks_search_fts_vocab');

-- External-content FTS indexes join source rows by primary key; maintenance
-- triggers synchronize searchable fields.
CREATE VIRTUAL TABLE IF NOT EXISTS movies_fts USING fts5 (
  title,
  overview,
  tag_line,
  content = 'movies',
  content_rowid = 'id',
  tokenize = 'unicode61 remove_diacritics 2'
);

CREATE VIRTUAL TABLE IF NOT EXISTS albums_fts USING fts5 (
  title,
  musician,
  content = 'albums',
  content_rowid = 'id',
  tokenize = 'unicode61 remove_diacritics 2'
);

CREATE VIRTUAL TABLE IF NOT EXISTS musicians_fts USING fts5 (
  name,
  sort_name,
  content = 'musicians',
  content_rowid = 'id',
  tokenize = 'unicode61 remove_diacritics 2'
);

-- Track search stores terms from tracks, albums, and musicians in one index.
CREATE VIRTUAL TABLE IF NOT EXISTS tracks_search_fts USING fts5 (
  title,
  album_title,
  musician_name,
  tokenize = 'unicode61 remove_diacritics 2'
);

-- Read-only term vocabularies support typo correction.
CREATE VIRTUAL TABLE IF NOT EXISTS movies_fts_vocab USING fts5vocab (movies_fts, 'row');

CREATE VIRTUAL TABLE IF NOT EXISTS albums_fts_vocab USING fts5vocab (albums_fts, 'row');

CREATE VIRTUAL TABLE IF NOT EXISTS musicians_fts_vocab USING fts5vocab (musicians_fts, 'row');

CREATE VIRTUAL TABLE IF NOT EXISTS tracks_search_fts_vocab USING fts5vocab (tracks_search_fts, 'row');

-- Maintenance triggers

-- Match-cache cleanup

CREATE TRIGGER IF NOT EXISTS music_spotify_matches_album_ad
AFTER DELETE ON albums
BEGIN
  DELETE FROM music_spotify_matches WHERE entity_type = 'album' AND entity_id = old.id;
END;

CREATE TRIGGER IF NOT EXISTS music_spotify_matches_musician_ad
AFTER DELETE ON musicians
BEGIN
  DELETE FROM music_spotify_matches WHERE entity_type = 'musician' AND entity_id = old.id;
END;

-- Search indexing
-- UPDATE triggers name only indexed inputs; album and musician changes also
-- refresh denormalized track terms.

CREATE TRIGGER IF NOT EXISTS movies_ai
AFTER INSERT ON movies
BEGIN
  INSERT INTO movies_fts (rowid, title, overview, tag_line)
  VALUES (new.id, new.title, new.overview, new.tag_line);
END;

CREATE TRIGGER IF NOT EXISTS movies_ad
AFTER DELETE ON movies
BEGIN
  INSERT INTO movies_fts (movies_fts, rowid, title, overview, tag_line)
  VALUES ('delete', old.id, old.title, old.overview, old.tag_line);
END;

CREATE TRIGGER IF NOT EXISTS movies_au
AFTER UPDATE OF title, overview, tag_line ON movies
BEGIN
  INSERT INTO movies_fts (movies_fts, rowid, title, overview, tag_line)
  VALUES ('delete', old.id, old.title, old.overview, old.tag_line);
  INSERT INTO movies_fts (rowid, title, overview, tag_line)
  VALUES (new.id, new.title, new.overview, new.tag_line);
END;

CREATE TRIGGER IF NOT EXISTS albums_ai
AFTER INSERT ON albums
BEGIN
  INSERT INTO albums_fts (rowid, title, musician)
  VALUES (new.id, new.title, new.musician);
END;

CREATE TRIGGER IF NOT EXISTS albums_ad
AFTER DELETE ON albums
BEGIN
  INSERT INTO albums_fts (albums_fts, rowid, title, musician)
  VALUES ('delete', old.id, old.title, old.musician);
END;

CREATE TRIGGER IF NOT EXISTS albums_au
AFTER UPDATE OF title, musician ON albums
BEGIN
  INSERT INTO albums_fts (albums_fts, rowid, title, musician)
  VALUES ('delete', old.id, old.title, old.musician);
  INSERT INTO albums_fts (rowid, title, musician)
  VALUES (new.id, new.title, new.musician);
END;

CREATE TRIGGER IF NOT EXISTS musicians_ai
AFTER INSERT ON musicians
BEGIN
  INSERT INTO musicians_fts (rowid, name, sort_name)
  VALUES (new.id, new.name, new.sort_name);
END;

CREATE TRIGGER IF NOT EXISTS musicians_ad
AFTER DELETE ON musicians
BEGIN
  INSERT INTO musicians_fts (musicians_fts, rowid, name, sort_name)
  VALUES ('delete', old.id, old.name, old.sort_name);
END;

CREATE TRIGGER IF NOT EXISTS musicians_au
AFTER UPDATE OF name, sort_name ON musicians
BEGIN
  INSERT INTO musicians_fts (musicians_fts, rowid, name, sort_name)
  VALUES ('delete', old.id, old.name, old.sort_name);
  INSERT INTO musicians_fts (rowid, name, sort_name)
  VALUES (new.id, new.name, new.sort_name);
END;

CREATE TRIGGER IF NOT EXISTS tracks_search_ai
AFTER INSERT ON tracks
BEGIN
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

CREATE TRIGGER IF NOT EXISTS tracks_search_ad
AFTER DELETE ON tracks
BEGIN
  DELETE FROM tracks_search_fts WHERE rowid = old.id;
END;

CREATE TRIGGER IF NOT EXISTS tracks_search_au
AFTER UPDATE OF title, album_id, musician_id ON tracks
BEGIN
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

CREATE TRIGGER IF NOT EXISTS tracks_search_album_au
AFTER UPDATE OF title, musician ON albums
BEGIN
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

CREATE TRIGGER IF NOT EXISTS tracks_search_musician_au
AFTER UPDATE OF name ON musicians
BEGIN
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

-- Search vocabulary invalidation
-- FTS and generation changes commit atomically. Only vocabularies affected
-- by searchable field changes are invalidated.

CREATE TRIGGER IF NOT EXISTS search_vocab_movies_ai
AFTER INSERT ON movies
BEGIN
  UPDATE search_vocab_generations
  SET generation = generation + 1
  WHERE vocab_table = 'movies_fts_vocab';
END;

CREATE TRIGGER IF NOT EXISTS search_vocab_movies_ad
AFTER DELETE ON movies
BEGIN
  UPDATE search_vocab_generations
  SET generation = generation + 1
  WHERE vocab_table = 'movies_fts_vocab';
END;

CREATE TRIGGER IF NOT EXISTS search_vocab_movies_au
AFTER UPDATE OF title, overview, tag_line ON movies
BEGIN
  UPDATE search_vocab_generations
  SET generation = generation + 1
  WHERE vocab_table = 'movies_fts_vocab';
END;

CREATE TRIGGER IF NOT EXISTS search_vocab_albums_ai
AFTER INSERT ON albums
BEGIN
  UPDATE search_vocab_generations
  SET generation = generation + 1
  WHERE vocab_table = 'albums_fts_vocab';
END;

CREATE TRIGGER IF NOT EXISTS search_vocab_albums_ad
AFTER DELETE ON albums
BEGIN
  UPDATE search_vocab_generations
  SET generation = generation + 1
  WHERE vocab_table = 'albums_fts_vocab';
END;

CREATE TRIGGER IF NOT EXISTS search_vocab_albums_au
AFTER UPDATE OF title, musician ON albums
BEGIN
  UPDATE search_vocab_generations
  SET generation = generation + 1
  WHERE vocab_table IN ('albums_fts_vocab', 'tracks_search_fts_vocab');
END;

CREATE TRIGGER IF NOT EXISTS search_vocab_musicians_ai
AFTER INSERT ON musicians
BEGIN
  UPDATE search_vocab_generations
  SET generation = generation + 1
  WHERE vocab_table = 'musicians_fts_vocab';
END;

CREATE TRIGGER IF NOT EXISTS search_vocab_musicians_ad
AFTER DELETE ON musicians
BEGIN
  UPDATE search_vocab_generations
  SET generation = generation + 1
  WHERE vocab_table = 'musicians_fts_vocab';
END;

CREATE TRIGGER IF NOT EXISTS search_vocab_musicians_au
AFTER UPDATE OF name, sort_name ON musicians
BEGIN
  UPDATE search_vocab_generations
  SET generation = generation + 1
  WHERE vocab_table IN ('musicians_fts_vocab', 'tracks_search_fts_vocab');
END;

CREATE TRIGGER IF NOT EXISTS search_vocab_tracks_ai
AFTER INSERT ON tracks
BEGIN
  UPDATE search_vocab_generations
  SET generation = generation + 1
  WHERE vocab_table = 'tracks_search_fts_vocab';
END;

CREATE TRIGGER IF NOT EXISTS search_vocab_tracks_ad
AFTER DELETE ON tracks
BEGIN
  UPDATE search_vocab_generations
  SET generation = generation + 1
  WHERE vocab_table = 'tracks_search_fts_vocab';
END;

CREATE TRIGGER IF NOT EXISTS search_vocab_tracks_au
AFTER UPDATE OF title, album_id, musician_id ON tracks
BEGIN
  UPDATE search_vocab_generations
  SET generation = generation + 1
  WHERE vocab_table = 'tracks_search_fts_vocab';
END;

-- Music relationship reconciliation
-- Local relationships reflect current track contributions, including cascades.
-- Explicit UPSERT clauses remain idempotent under an outer track UPSERT conflict policy.

CREATE TRIGGER IF NOT EXISTS music_track_musicians_insert_relationships
AFTER INSERT ON track_musicians
BEGIN
  INSERT INTO musician_albums (musician_id, album_id)
  SELECT NEW.musician_id, album_id FROM tracks
  WHERE id = NEW.track_id AND album_id IS NOT NULL ON CONFLICT DO NOTHING;
  INSERT INTO musician_genres (musician_id, genre_id, source)
  SELECT NEW.musician_id, genre_id, 'local' FROM track_genres
  WHERE track_id = NEW.track_id ON CONFLICT DO NOTHING;
END;

CREATE TRIGGER IF NOT EXISTS music_track_musicians_delete_relationships
AFTER DELETE ON track_musicians
BEGIN
  DELETE FROM musician_albums WHERE musician_id = OLD.musician_id AND NOT EXISTS (
    SELECT 1 FROM tracks t JOIN track_musicians tm ON tm.track_id = t.id
    WHERE tm.musician_id = musician_albums.musician_id AND t.album_id = musician_albums.album_id
  );
  DELETE FROM musician_genres WHERE source = 'local' AND musician_id = OLD.musician_id AND NOT EXISTS (
    SELECT 1 FROM track_musicians tm JOIN track_genres tg ON tg.track_id = tm.track_id
    WHERE tm.musician_id = musician_genres.musician_id AND tg.genre_id = musician_genres.genre_id
  );
END;

CREATE TRIGGER IF NOT EXISTS music_track_genres_insert_relationships
AFTER INSERT ON track_genres
BEGIN
  INSERT INTO musician_genres (musician_id, genre_id, source)
  SELECT musician_id, NEW.genre_id, 'local' FROM track_musicians
  WHERE track_id = NEW.track_id ON CONFLICT DO NOTHING;
  INSERT INTO album_genres (album_id, genre_id, source)
  SELECT album_id, NEW.genre_id, 'local' FROM tracks
  WHERE id = NEW.track_id AND album_id IS NOT NULL ON CONFLICT DO NOTHING;
END;

CREATE TRIGGER IF NOT EXISTS music_track_genres_delete_relationships
AFTER DELETE ON track_genres
BEGIN
  DELETE FROM musician_genres WHERE source = 'local' AND genre_id = OLD.genre_id
  AND musician_id IN (SELECT musician_id FROM track_musicians WHERE track_id = OLD.track_id)
  AND NOT EXISTS (
    SELECT 1 FROM track_musicians tm JOIN track_genres tg ON tg.track_id = tm.track_id
    WHERE tm.musician_id = musician_genres.musician_id AND tg.genre_id = musician_genres.genre_id
  );
  DELETE FROM album_genres WHERE source = 'local' AND genre_id = OLD.genre_id
  AND album_id = (SELECT album_id FROM tracks WHERE id = OLD.track_id)
  AND NOT EXISTS (
    SELECT 1 FROM tracks t JOIN track_genres tg ON tg.track_id = t.id
    WHERE t.album_id = album_genres.album_id AND tg.genre_id = album_genres.genre_id
  );
END;

CREATE TRIGGER IF NOT EXISTS music_tracks_update_relationships
AFTER UPDATE OF album_id ON tracks WHEN OLD.album_id IS NOT NEW.album_id
BEGIN
  DELETE FROM musician_albums WHERE album_id = OLD.album_id
  AND musician_id IN (SELECT musician_id FROM track_musicians WHERE track_id = OLD.id)
  AND NOT EXISTS (
    SELECT 1 FROM tracks t JOIN track_musicians tm ON tm.track_id = t.id
    WHERE tm.musician_id = musician_albums.musician_id AND t.album_id = musician_albums.album_id
  );
  INSERT INTO musician_albums (musician_id, album_id)
  SELECT musician_id, NEW.album_id FROM track_musicians
  WHERE track_id = NEW.id AND NEW.album_id IS NOT NULL ON CONFLICT DO NOTHING;
  DELETE FROM album_genres WHERE source = 'local' AND album_id = OLD.album_id AND NOT EXISTS (
    SELECT 1 FROM tracks t JOIN track_genres tg ON tg.track_id = t.id
    WHERE t.album_id = album_genres.album_id AND tg.genre_id = album_genres.genre_id
  );
  INSERT INTO album_genres (album_id, genre_id, source)
  SELECT NEW.album_id, genre_id, 'local' FROM track_genres
  WHERE track_id = NEW.id AND NEW.album_id IS NOT NULL ON CONFLICT DO NOTHING;
END;

CREATE TRIGGER IF NOT EXISTS music_tracks_delete_album_genres
AFTER DELETE ON tracks
BEGIN
  DELETE FROM album_genres WHERE source = 'local' AND album_id = OLD.album_id AND NOT EXISTS (
    SELECT 1 FROM tracks t JOIN track_genres tg ON tg.track_id = t.id
    WHERE t.album_id = album_genres.album_id AND tg.genre_id = album_genres.genre_id
  );
END;

-- Album deletion cascades away track contributions. Select surviving sort
-- votes first so only artists credited on the deleted album are updated.
CREATE TRIGGER IF NOT EXISTS music_album_delete_artist_sorts
BEFORE DELETE ON albums
BEGIN
  UPDATE musicians SET sort_name = COALESCE((
    SELECT vote FROM (
      SELECT MIN(m.sort_name COLLATE BINARY) AS vote
      FROM music_credit_metadata m JOIN tracks t ON t.id = m.track_id
      WHERE m.musician_id = musicians.id AND m.sort_name <> '' AND t.album_id IS NOT OLD.id
      GROUP BY m.track_id
    ) GROUP BY vote ORDER BY COUNT(*) DESC, vote COLLATE BINARY LIMIT 1
  ), name)
  WHERE id IN (SELECT tm.musician_id FROM track_musicians tm JOIN tracks t ON t.id = tm.track_id WHERE t.album_id = OLD.id)
  AND sort_name IS NOT COALESCE((
    SELECT vote FROM (
      SELECT MIN(m.sort_name COLLATE BINARY) AS vote
      FROM music_credit_metadata m JOIN tracks t ON t.id = m.track_id
      WHERE m.musician_id = musicians.id AND m.sort_name <> '' AND t.album_id IS NOT OLD.id
      GROUP BY m.track_id
    ) GROUP BY vote ORDER BY COUNT(*) DESC, vote COLLATE BINARY LIMIT 1
  ), name);
END;

-- TV catalog: directories own shows; physical files own technical metadata.
CREATE TABLE IF NOT EXISTS shows (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 directory_path TEXT NOT NULL UNIQUE,
 local_name TEXT NOT NULL,
 premiere_year INTEGER,
 name TEXT NOT NULL,
 tmdb_id INTEGER,
 imdb_id TEXT,
 original_name TEXT,
 overview TEXT,
 tagline TEXT,
 language TEXT,
 origin_countries TEXT,
 first_air_date TEXT,
 last_air_date TEXT,
 status TEXT,
 type TEXT,
 adult BOOLEAN NOT NULL DEFAULT false,
 poster_path TEXT,
 backdrop_path TEXT,
 homepage TEXT,
 vote_average REAL,
 vote_count INTEGER,
 popularity REAL,
 certification TEXT,
 tmdb_season_count INTEGER,
 tmdb_episode_count INTEGER,
 created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- Keep this expression and tie-breaker aligned with GetShowsLibraryAsc/Desc.
CREATE INDEX IF NOT EXISTS idx_shows_name ON shows (LOWER(name), id);
-- attempts counts definitive TMDB misses; last_attempt_at (unix seconds) drives
-- the miss backoff shared with movies.
CREATE TABLE IF NOT EXISTS show_tmdb_retries (
 show_id INTEGER PRIMARY KEY NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
 attempts INTEGER NOT NULL DEFAULT 0,
 last_attempt_at INTEGER
);
CREATE TABLE IF NOT EXISTS show_seasons (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 show_id INTEGER NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
 season_number INTEGER NOT NULL CHECK (season_number >= 0),
 name TEXT NOT NULL,
 tmdb_id INTEGER,
 overview TEXT,
 air_date TEXT,
 poster_path TEXT,
 vote_average REAL,
 tmdb_episode_count INTEGER,
 created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE (show_id, season_number)
);
CREATE TABLE IF NOT EXISTS show_season_tmdb_retries (season_id INTEGER PRIMARY KEY NOT NULL REFERENCES show_seasons(id) ON DELETE CASCADE);
CREATE TABLE IF NOT EXISTS show_episodes (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 season_id INTEGER NOT NULL REFERENCES show_seasons(id) ON DELETE CASCADE,
 episode_number INTEGER NOT NULL CHECK (episode_number > 0),
 name TEXT NOT NULL,
 tmdb_id INTEGER,
 overview TEXT,
 air_date TEXT,
 still_path TEXT,
 production_code TEXT,
 tmdb_runtime INTEGER,
 vote_average REAL,
 vote_count INTEGER,
 created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE (season_id, episode_number),
 UNIQUE (id, season_id)
);
CREATE TABLE IF NOT EXISTS show_episode_tmdb_retries (episode_id INTEGER PRIMARY KEY NOT NULL REFERENCES show_episodes(id) ON DELETE CASCADE);
CREATE TABLE IF NOT EXISTS show_files (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 season_id INTEGER NOT NULL REFERENCES show_seasons(id) ON DELETE CASCADE,
 file_path TEXT NOT NULL UNIQUE,
 file_name TEXT NOT NULL,
 size INTEGER NOT NULL,
 container TEXT NOT NULL CHECK (container IN ('mkv', 'mp4', 'avi', 'mov', 'm4v', 'webm')),
 mime_type TEXT NOT NULL,
 duration REAL,
 created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE (id, season_id)
);
CREATE INDEX IF NOT EXISTS idx_show_files_season ON show_files(season_id);
CREATE TABLE IF NOT EXISTS show_episode_files (
 episode_id INTEGER NOT NULL,
 file_id INTEGER NOT NULL,
 season_id INTEGER NOT NULL,
 episode_order INTEGER NOT NULL CHECK (episode_order >= 0),
 PRIMARY KEY (episode_id, file_id),
 UNIQUE (file_id, episode_order),
 FOREIGN KEY (episode_id, season_id) REFERENCES show_episodes(id, season_id) ON DELETE CASCADE,
 FOREIGN KEY (file_id, season_id) REFERENCES show_files(id, season_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_show_episode_files_file_season ON show_episode_files(file_id, season_id);
CREATE INDEX IF NOT EXISTS idx_show_episode_files_episode_season ON show_episode_files(episode_id, season_id);
CREATE TABLE IF NOT EXISTS show_file_fingerprints (
 file_id INTEGER PRIMARY KEY NOT NULL REFERENCES show_files(id) ON DELETE CASCADE,
 mtime_ns INTEGER NOT NULL,
 ctime_ns INTEGER NOT NULL,
 device TEXT NOT NULL,
 inode TEXT NOT NULL
);
CREATE TABLE
  IF NOT EXISTS show_video_streams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER NOT NULL,
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
    field_order TEXT,
    -- Display-matrix rotation in degrees; NULL when the stream has no
    -- display matrix (an explicit 0-degree matrix persists as 0).
    rotation INTEGER,
    language TEXT,
    title TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (file_id) REFERENCES show_files (id) ON DELETE CASCADE ON UPDATE CASCADE
  );
CREATE UNIQUE INDEX IF NOT EXISTS ux_show_video_streams_file_stream ON show_video_streams(file_id, stream_index);
CREATE TABLE
  IF NOT EXISTS show_audio_streams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER NOT NULL,
    stream_index INTEGER NOT NULL,
    codec TEXT NOT NULL,
    codec_profile TEXT,
    bit_rate INTEGER NOT NULL,
    sample_rate INTEGER,
    channels INTEGER NOT NULL,
    channel_layout TEXT,
    language TEXT,
    title TEXT,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (file_id) REFERENCES show_files (id) ON DELETE CASCADE ON UPDATE CASCADE
  );
CREATE UNIQUE INDEX IF NOT EXISTS ux_show_audio_streams_file_stream ON show_audio_streams(file_id, stream_index);
CREATE TABLE
  IF NOT EXISTS show_subtitles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER NOT NULL,
    stream_index INTEGER NOT NULL,
    codec TEXT NOT NULL,
    language TEXT,
    title TEXT,
    is_forced BOOLEAN NOT NULL DEFAULT false,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (file_id) REFERENCES show_files (id) ON DELETE CASCADE ON UPDATE CASCADE
  );
CREATE UNIQUE INDEX IF NOT EXISTS ux_show_subtitles_file_stream ON show_subtitles(file_id, stream_index);
CREATE TABLE
  IF NOT EXISTS show_chapters (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    start_time INTEGER NOT NULL,
    thumb TEXT,
    file_id INTEGER NOT NULL,
    FOREIGN KEY (file_id) REFERENCES show_files (id) ON DELETE CASCADE ON UPDATE CASCADE
  );
CREATE INDEX IF NOT EXISTS idx_show_chapters_file ON show_chapters(file_id, start_time);
-- Per-file playback caches for TV. One show_files row can back several
-- episodes, so these key on the physical file, exactly as the movie twins key
-- on the movie row. Same fingerprint rules as remux_safety_verdicts and
-- keyframe_indexes above.
CREATE TABLE IF NOT EXISTS show_remux_safety_verdicts (
  file_id INTEGER NOT NULL,
  stream_index INTEGER NOT NULL,
  fingerprint TEXT NOT NULL,
  safe BOOLEAN NOT NULL,
  reason TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (file_id, stream_index),
  FOREIGN KEY (file_id) REFERENCES show_files (id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE TABLE IF NOT EXISTS show_keyframe_indexes (
  file_id INTEGER NOT NULL,
  stream_index INTEGER NOT NULL,
  fingerprint TEXT NOT NULL,
  duration_sec REAL NOT NULL,
  keyframes TEXT NOT NULL,
  PRIMARY KEY (file_id, stream_index),
  FOREIGN KEY (file_id) REFERENCES show_files (id) ON DELETE CASCADE ON UPDATE CASCADE
);
-- Watch progress is per logical episode, not per file: two episodes in one
-- combined file keep separate positions. Same write-ordering columns as
-- movie_watch_progress.
CREATE TABLE IF NOT EXISTS show_episode_watch_progress (
  user_id INTEGER NOT NULL,
  episode_id INTEGER NOT NULL,
  progress_sec REAL NOT NULL DEFAULT 0,
  duration_sec REAL NOT NULL DEFAULT 0,
  watched BOOLEAN NOT NULL DEFAULT false,
  save_session_id TEXT NOT NULL DEFAULT '',
  save_sequence INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, episode_id),
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (episode_id) REFERENCES show_episodes (id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_show_episode_watch_progress_user_updated_at ON show_episode_watch_progress (user_id, updated_at DESC)
WHERE watched = false;
CREATE INDEX IF NOT EXISTS idx_show_episode_watch_progress_episode ON show_episode_watch_progress (episode_id);
CREATE TABLE IF NOT EXISTS networks (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 tmdb_id INTEGER NOT NULL UNIQUE,
 name TEXT NOT NULL,
 logo TEXT,
 country TEXT
);
CREATE TABLE IF NOT EXISTS show_cast (
 show_id INTEGER NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
 artist_id INTEGER NOT NULL REFERENCES artist(id),
 character TEXT NOT NULL,
 cast_order INTEGER NOT NULL,
 credit_id TEXT NOT NULL,
 episode_count INTEGER NOT NULL,
 PRIMARY KEY (show_id, artist_id, character, credit_id)
);
CREATE INDEX IF NOT EXISTS idx_show_cast_artist_id ON show_cast(artist_id);
CREATE TABLE IF NOT EXISTS show_crew (
 show_id INTEGER NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
 artist_id INTEGER NOT NULL REFERENCES artist(id),
 department TEXT NOT NULL,
 job TEXT NOT NULL,
 credit_id TEXT NOT NULL,
 episode_count INTEGER NOT NULL,
 PRIMARY KEY (show_id, artist_id, department, job, credit_id)
);
CREATE INDEX IF NOT EXISTS idx_show_crew_artist_id ON show_crew(artist_id);
CREATE TABLE IF NOT EXISTS show_genres (
 show_id INTEGER NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
 genre_id INTEGER NOT NULL REFERENCES genres(id),
 PRIMARY KEY (show_id, genre_id)
);
CREATE INDEX IF NOT EXISTS idx_show_genres_genre_id ON show_genres(genre_id);
CREATE TABLE IF NOT EXISTS show_production_companies (
 show_id INTEGER NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
 production_company_id INTEGER NOT NULL REFERENCES production_companies(id),
 PRIMARY KEY (show_id, production_company_id)
);
CREATE INDEX IF NOT EXISTS idx_show_production_companies_production_company_id ON show_production_companies(production_company_id);
CREATE TABLE IF NOT EXISTS show_networks (
 show_id INTEGER NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
 network_id INTEGER NOT NULL REFERENCES networks(id),
 PRIMARY KEY (show_id, network_id)
);
CREATE INDEX IF NOT EXISTS idx_show_networks_network_id ON show_networks(network_id);
CREATE TABLE IF NOT EXISTS show_creators (
 show_id INTEGER NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
 artist_id INTEGER NOT NULL REFERENCES artist(id),
 PRIMARY KEY (show_id, artist_id)
);
CREATE INDEX IF NOT EXISTS idx_show_creators_artist_id ON show_creators(artist_id);
CREATE TABLE IF NOT EXISTS show_extra_videos (
 show_id INTEGER NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
 extra_video_id INTEGER NOT NULL REFERENCES extra_videos(id),
 PRIMARY KEY (show_id, extra_video_id)
);
CREATE INDEX IF NOT EXISTS idx_show_extra_videos_extra_video_id ON show_extra_videos(extra_video_id);
CREATE TABLE IF NOT EXISTS show_season_cast (
 season_id INTEGER NOT NULL REFERENCES show_seasons(id) ON DELETE CASCADE,
 artist_id INTEGER NOT NULL REFERENCES artist(id),
 character TEXT NOT NULL,
 cast_order INTEGER NOT NULL,
 credit_id TEXT NOT NULL,
 episode_count INTEGER NOT NULL,
 PRIMARY KEY (season_id, artist_id, character, credit_id)
);
CREATE INDEX IF NOT EXISTS idx_show_season_cast_artist_id ON show_season_cast(artist_id);
CREATE TABLE IF NOT EXISTS show_season_crew (
 season_id INTEGER NOT NULL REFERENCES show_seasons(id) ON DELETE CASCADE,
 artist_id INTEGER NOT NULL REFERENCES artist(id),
 department TEXT NOT NULL,
 job TEXT NOT NULL,
 credit_id TEXT NOT NULL,
 episode_count INTEGER NOT NULL,
 PRIMARY KEY (season_id, artist_id, department, job, credit_id)
);
CREATE INDEX IF NOT EXISTS idx_show_season_crew_artist_id ON show_season_crew(artist_id);
CREATE TABLE IF NOT EXISTS show_season_extra_videos (
 season_id INTEGER NOT NULL REFERENCES show_seasons(id) ON DELETE CASCADE,
 extra_video_id INTEGER NOT NULL REFERENCES extra_videos(id),
 PRIMARY KEY (season_id, extra_video_id)
);
CREATE INDEX IF NOT EXISTS idx_show_season_extra_videos_extra_video_id ON show_season_extra_videos(extra_video_id);
CREATE TABLE IF NOT EXISTS show_episode_crew (
 episode_id INTEGER NOT NULL REFERENCES show_episodes(id) ON DELETE CASCADE,
 artist_id INTEGER NOT NULL REFERENCES artist(id),
 department TEXT NOT NULL,
 job TEXT NOT NULL,
 credit_id TEXT NOT NULL,
 episode_count INTEGER NOT NULL,
 PRIMARY KEY (episode_id, artist_id, department, job, credit_id)
);
CREATE INDEX IF NOT EXISTS idx_show_episode_crew_artist_id ON show_episode_crew(artist_id);
CREATE TABLE IF NOT EXISTS show_episode_guest_cast (
 episode_id INTEGER NOT NULL REFERENCES show_episodes(id) ON DELETE CASCADE,
 artist_id INTEGER NOT NULL REFERENCES artist(id),
 character TEXT NOT NULL,
 cast_order INTEGER NOT NULL,
 credit_id TEXT NOT NULL,
 episode_count INTEGER NOT NULL,
 PRIMARY KEY (episode_id, artist_id, character, credit_id)
);
CREATE INDEX IF NOT EXISTS idx_show_episode_guest_cast_artist_id ON show_episode_guest_cast(artist_id);
