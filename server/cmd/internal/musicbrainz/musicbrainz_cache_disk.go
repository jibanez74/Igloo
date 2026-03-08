package musicbrainz

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const diskCacheTTL = 7 * 24 * time.Hour

type diskCache struct {
	db *sql.DB
}

func openDiskCache(cacheDir string) (*diskCache, error) {
	if cacheDir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(cacheDir, "cache.db")
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	schema := `
	CREATE TABLE IF NOT EXISTS artist_cache (key TEXT PRIMARY KEY, value BLOB, expires_at INTEGER);
	CREATE TABLE IF NOT EXISTS album_cache (key TEXT PRIMARY KEY, value BLOB, expires_at INTEGER);
	CREATE TABLE IF NOT EXISTS image_cache (key TEXT PRIMARY KEY, value TEXT, expires_at INTEGER);
	`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &diskCache{db: db}, nil
}

func (d *diskCache) getArtist(key string) (*ArtistResult, bool) {
	if d == nil || d.db == nil {
		return nil, false
	}
	var blob []byte
	err := d.db.QueryRow("SELECT value FROM artist_cache WHERE key = ? AND expires_at > ?", key, time.Now().Unix()).Scan(&blob)
	if err == sql.ErrNoRows || err != nil {
		return nil, false
	}
	var r ArtistResult
	if json.Unmarshal(blob, &r) != nil {
		return nil, false
	}
	return &r, true
}

func (d *diskCache) setArtist(key string, r *ArtistResult) {
	if d == nil || d.db == nil || r == nil {
		return
	}
	blob, err := json.Marshal(r)
	if err != nil {
		return
	}
	expiresAt := time.Now().Add(diskCacheTTL).Unix()
	_, _ = d.db.Exec("INSERT OR REPLACE INTO artist_cache (key, value, expires_at) VALUES (?, ?, ?)", key, blob, expiresAt)
}

func (d *diskCache) getAlbum(key string) (*AlbumResult, bool) {
	if d == nil || d.db == nil {
		return nil, false
	}
	var blob []byte
	err := d.db.QueryRow("SELECT value FROM album_cache WHERE key = ? AND expires_at > ?", key, time.Now().Unix()).Scan(&blob)
	if err == sql.ErrNoRows || err != nil {
		return nil, false
	}
	var r AlbumResult
	if json.Unmarshal(blob, &r) != nil {
		return nil, false
	}
	return &r, true
}

func (d *diskCache) setAlbum(key string, r *AlbumResult) {
	if d == nil || d.db == nil || r == nil {
		return
	}
	blob, err := json.Marshal(r)
	if err != nil {
		return
	}
	expiresAt := time.Now().Add(diskCacheTTL).Unix()
	_, _ = d.db.Exec("INSERT OR REPLACE INTO album_cache (key, value, expires_at) VALUES (?, ?, ?)", key, blob, expiresAt)
}

func (d *diskCache) getImage(key string) (string, bool) {
	if d == nil || d.db == nil {
		return "", false
	}
	var val string
	err := d.db.QueryRow("SELECT value FROM image_cache WHERE key = ? AND expires_at > ?", key, time.Now().Unix()).Scan(&val)
	if err == sql.ErrNoRows || err != nil {
		return "", false
	}
	return val, true
}

func (d *diskCache) setImage(key, url string) {
	if d == nil || d.db == nil {
		return
	}
	expiresAt := time.Now().Add(diskCacheTTL).Unix()
	_, _ = d.db.Exec("INSERT OR REPLACE INTO image_cache (key, value, expires_at) VALUES (?, ?, ?)", key, url, expiresAt)
}

func (d *diskCache) close() error {
	if d == nil || d.db == nil {
		return nil
	}
	return d.db.Close()
}
