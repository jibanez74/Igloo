-- users table
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

-- settings table
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

-- musicians table
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

-- albums table
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

-- tracks table
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
    FOREIGN KEY (album_id) REFERENCES albums (id) ON DELETE SET NULL ON UPDATE CASCADE,
    FOREIGN KEY (musician_id) REFERENCES musicians (id) ON DELETE SET NULL ON UPDATE CASCADE
  );

CREATE INDEX IF NOT EXISTS idx_track_title ON tracks (title);

CREATE INDEX IF NOT EXISTS idx_track_album ON tracks (album_id);

CREATE INDEX IF NOT EXISTS idx_track_musician ON tracks (musician_id);

-- table for genres
CREATE TABLE
  IF NOT EXISTS genres (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tag TEXT NOT NULL,
    genre_type TEXT NOT NULL CHECK (genre_type IN ('movie', 'show', 'music')),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tag, genre_type)
  );

CREATE INDEX IF NOT EXISTS idx_genre_tag ON genres (tag);

-- many to many tables
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

-- many-to-many: musicians <-> albums
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

-- many-to-many: tracks <-> genres
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

-- many-to-many: albums <-> genres
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

-- sessions table for scs session management
CREATE TABLE
  IF NOT EXISTS sessions (
    token TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    expiry REAL NOT NULL
  );

CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions (expiry);