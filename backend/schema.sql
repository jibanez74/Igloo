--
-- Drop existing tables (in correct order for foreign key constraints)
--
DROP TABLE IF EXISTS movie_genres CASCADE;
DROP TABLE IF EXISTS movie_studios CASCADE;
DROP TABLE IF EXISTS user_movies CASCADE;
DROP TABLE IF EXISTS video_streams CASCADE;
DROP TABLE IF EXISTS audio_streams CASCADE;
DROP TABLE IF EXISTS subtitles CASCADE;
DROP TABLE IF EXISTS chapters CASCADE;
DROP TABLE IF EXISTS movie_extras CASCADE;
DROP TABLE IF EXISTS crew_list CASCADE;
DROP TABLE IF EXISTS cast_list CASCADE;
DROP TABLE IF EXISTS movies CASCADE;
DROP TABLE IF EXISTS artists CASCADE;
DROP TABLE IF EXISTS genres CASCADE;
DROP TABLE IF EXISTS studios CASCADE;
DROP TABLE IF EXISTS global_settings;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS device_codes CASCADE;

--
-- Core tables
--
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    name VARCHAR(60) NOT NULL,
    email VARCHAR(100) NOT NULL UNIQUE,
    username VARCHAR(20) NOT NULL UNIQUE,
    password VARCHAR(128) NOT NULL,
    is_active BOOLEAN NOT NULL,
    is_admin BOOLEAN NOT NULL,
    avatar VARCHAR(255) NOT NULL
);

CREATE TABLE global_settings (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    port INTEGER NOT NULL,
    debug BOOLEAN NOT NULL,
    base_url VARCHAR(255) NOT NULL,
    movies_dir_list VARCHAR(255) NOT NULL,
    movies_img_dir VARCHAR(255) NOT NULL,
    music_dir_list VARCHAR(255) NOT NULL,
    tvshows_dir_list VARCHAR(255) NOT NULL,
    transcode_dir VARCHAR(255) NOT NULL,
    studios_img_dir VARCHAR(255) NOT NULL,
    artists_img_dir VARCHAR(255) NOT NULL,
    static_dir VARCHAR(255) NOT NULL,
    download_images BOOLEAN NOT NULL,
    tmdb_api_key VARCHAR(255) NOT NULL,
    ffmpeg_path VARCHAR(255) NOT NULL,
    ffprobe_path VARCHAR(255) NOT NULL,
    hardware_acceleration VARCHAR(255) NOT NULL,
    jellyfin_token VARCHAR(255) NOT NULL,
        issuer VARCHAR(255) NOT NULL,
    audience      VARCHAR(255) NOT NULL,
    secret        VARCHAR(255) NOT NULL,
    cookie_domain  VARCHAR(255) NOT NULL,
    cookie_path    VARCHAR(255) NOT NULL
);

CREATE TABLE device_codes (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    device_code VARCHAR(255) NOT NULL UNIQUE,
    user_code VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    is_verified BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE movies (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    title VARCHAR(100) NOT NULL,
    file_path VARCHAR(255) NOT NULL UNIQUE,
    file_name VARCHAR(255) NOT NULL,
    container VARCHAR(10) NOT NULL,
    size BIGINT NOT NULL,
    content_type VARCHAR(50) NOT NULL,
    run_time INTEGER NOT NULL,
    adult BOOLEAN NOT NULL,
    tag_line VARCHAR(255) NOT NULL,
    summary TEXT NOT NULL,
    art TEXT NOT NULL,
    thumb TEXT NOT NULL,
    tmdb_id VARCHAR(255) NOT NULL,
    imdb_id VARCHAR(255) NOT NULL,
    year INTEGER NOT NULL,
    release_date DATE NOT NULL,
    budget BIGINT NOT NULL,
    revenue BIGINT NOT NULL,
    content_rating VARCHAR(20) NOT NULL,
    audience_rating REAL NOT NULL,
    critic_rating REAL NOT NULL,
    spoken_languages VARCHAR(255) NOT NULL
);

CREATE TABLE artists (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    name VARCHAR(255) NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    thumb TEXT NOT NULL,
    tmdb_id INTEGER UNIQUE NOT NULL
);

CREATE TABLE genres (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    tag VARCHAR(50) NOT NULL,
    genre_type VARCHAR(10) NOT NULL
);

CREATE TABLE studios (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    name VARCHAR(255) NOT NULL,
    country VARCHAR(2) NOT NULL,
    logo TEXT NOT NULL,
    tmdb_id INTEGER UNIQUE NOT NULL
);

--
-- Movie-related tables
--
CREATE TABLE cast_list (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    artist_id INTEGER REFERENCES artists(id) ON DELETE CASCADE,
    movie_id INTEGER REFERENCES movies(id) ON DELETE CASCADE,
    character TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);

CREATE TABLE crew_list (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    artist_id INTEGER REFERENCES artists(id) ON DELETE CASCADE,
    movie_id INTEGER REFERENCES movies(id) ON DELETE CASCADE,
    job VARCHAR(255) NOT NULL,
    department VARCHAR(255) NOT NULL
);

CREATE TABLE video_streams (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    title VARCHAR(255) NOT NULL,
    index INTEGER NOT NULL,
    profile VARCHAR(20) NOT NULL,
    aspect_ratio VARCHAR(50) NOT NULL,
    bit_rate VARCHAR(60) NOT NULL,
    bit_depth VARCHAR(60) NOT NULL,
    codec VARCHAR(30) NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    coded_width INTEGER NOT NULL,
    coded_height INTEGER NOT NULL,
    color_transfer VARCHAR(100) NOT NULL,
    color_primaries VARCHAR(100) NOT NULL,
    color_space VARCHAR(100) NOT NULL,
    color_range VARCHAR(100) NOT NULL,
    frame_rate VARCHAR(100) NOT NULL,
    avg_frame_rate VARCHAR(100) NOT NULL,
    level INTEGER NOT NULL,
    movie_id INTEGER REFERENCES movies(id) ON DELETE CASCADE
);

CREATE TABLE audio_streams (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    title VARCHAR(255) NOT NULL,
    index INTEGER NOT NULL,
    codec VARCHAR(30) NOT NULL,
    channels INTEGER NOT NULL,
    channel_layout VARCHAR(50) NOT NULL,
    language VARCHAR(60) NOT NULL,
    movie_id INTEGER REFERENCES movies(id) ON DELETE CASCADE
);

CREATE TABLE subtitles (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    title VARCHAR(255) NOT NULL,
    index INTEGER NOT NULL,
    codec VARCHAR(30) NOT NULL,
    language VARCHAR(100) NOT NULL,
    movie_id INTEGER REFERENCES movies(id) ON DELETE CASCADE
);

CREATE TABLE chapters (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    title VARCHAR(255) NOT NULL,
    start_time_ms INTEGER NOT NULL,
    thumb VARCHAR(255) NOT NULL,
    movie_id INTEGER REFERENCES movies(id) ON DELETE CASCADE
);

CREATE TABLE movie_extras (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    title VARCHAR(255) NOT NULL,
    url VARCHAR(255) NOT NULL,
    kind VARCHAR(100) NOT NULL,
    movie_id INTEGER REFERENCES movies(id) ON DELETE CASCADE
);

--
-- Junction tables for many-to-many relationships
--
CREATE TABLE movie_genres (
    movie_id INTEGER REFERENCES movies(id) ON DELETE CASCADE,
    genre_id INTEGER REFERENCES genres(id) ON DELETE CASCADE,
    PRIMARY KEY (movie_id, genre_id)
);

CREATE TABLE movie_studios (
    movie_id INTEGER REFERENCES movies(id) ON DELETE CASCADE,
    studio_id INTEGER REFERENCES studios(id) ON DELETE CASCADE,
    PRIMARY KEY (movie_id, studio_id)
);

CREATE TABLE user_movies (
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    movie_id INTEGER REFERENCES movies(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, movie_id)
);

--
-- Indexes
--
-- Core table indexes
CREATE INDEX idx_movies_title ON movies(title);
CREATE INDEX idx_artists_name ON artists(name);
CREATE INDEX idx_genres_tag ON genres(tag);
CREATE INDEX idx_studios_name ON studios(name);

-- Foreign key indexes
CREATE INDEX idx_cast_list_movie_id ON cast_list(movie_id);
CREATE INDEX idx_cast_list_artist_id ON cast_list(artist_id);
CREATE INDEX idx_crew_list_movie_id ON crew_list(movie_id);
CREATE INDEX idx_crew_list_artist_id ON crew_list(artist_id);
CREATE INDEX idx_movie_extras_movie_id ON movie_extras(movie_id);
CREATE INDEX idx_chapters_movie_id ON chapters(movie_id);
CREATE INDEX idx_video_streams_movie_id ON video_streams(movie_id);
CREATE INDEX idx_audio_streams_movie_id ON audio_streams(movie_id);
CREATE INDEX idx_subtitles_movie_id ON subtitles(movie_id);

-- Many-to-many relationship indexes
CREATE INDEX idx_movie_genres_movie_id ON movie_genres(movie_id);
CREATE INDEX idx_movie_genres_genre_id ON movie_genres(genre_id);
CREATE INDEX idx_movie_studios_movie_id ON movie_studios(movie_id);
CREATE INDEX idx_movie_studios_studio_id ON movie_studios(studio_id);
CREATE INDEX idx_user_movies_user_id ON user_movies(user_id);
CREATE INDEX idx_user_movies_movie_id ON user_movies(movie_id);

CREATE INDEX idx_device_codes_device_code ON device_codes(device_code);
CREATE INDEX idx_device_codes_user_code ON device_codes(user_code);
CREATE INDEX idx_device_codes_expires_at ON device_codes(expires_at);
