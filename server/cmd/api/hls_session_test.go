package main

import (
	"context"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
)

func TestCreateHLSSession_ErrorsWhenMovieHasNoDuration(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	ctx := context.Background()
	_, err := app.DB.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult)
		VALUES ('No Duration', '/tmp/nodur.mkv', 'nodur.mkv', 1, 'mkv', 'video/x-matroska', 0)
	`)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	var id int64
	err = app.DB.QueryRowContext(ctx, `SELECT id FROM movies WHERE file_path = '/tmp/nodur.mkv'`).Scan(&id)
	if err != nil {
		t.Fatalf("select id: %v", err)
	}

	_, err = app.createHLSSession(ctx, id, "720p_3mbps", 0, 0)
	if err == nil {
		t.Fatal("expected error when duration missing")
	}
	if !strings.Contains(err.Error(), "no valid duration") {
		t.Errorf("error = %v, want mention of no valid duration", err)
	}
}

func TestCreateHLSSession_ErrorsWhenNoVideoStream(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	ctx := context.Background()
	_, err := app.DB.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult, duration)
		VALUES ('No Video', '/tmp/novid.mkv', 'novid.mkv', 1, 'mkv', 'video/x-matroska', 0, 3600.0)
	`)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	var id int64
	err = app.DB.QueryRowContext(ctx, `SELECT id FROM movies WHERE file_path = '/tmp/novid.mkv'`).Scan(&id)
	if err != nil {
		t.Fatalf("select id: %v", err)
	}

	_, err = app.createHLSSession(ctx, id, "720p_3mbps", 0, 0)
	if err == nil {
		t.Fatal("expected error when no video stream rows")
	}
	if !strings.Contains(err.Error(), "no playable video track") {
		t.Errorf("error = %v, want mention of no playable video", err)
	}
}

func TestCreateHLSSession_ErrorsWhenAudioTrackOutOfRange(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	ctx := context.Background()
	res, err := app.DB.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult, duration)
		VALUES ('One Audio', '/tmp/oneaud.mkv', 'oneaud.mkv', 1, 'mkv', 'video/x-matroska', 0, 100.0)
	`)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	movieID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}

	_, err = app.DB.Exec(`
		INSERT INTO video_streams (movie_id, stream_index, codec, bit_rate, width, height, frame_rate)
		VALUES (?, 0, 'h264', 5000000, 1920, 1080, 23.976)
	`, movieID)
	if err != nil {
		t.Fatalf("insert video stream: %v", err)
	}
	_, err = app.DB.Exec(`
		INSERT INTO audio_streams (movie_id, stream_index, codec, bit_rate, channels)
		VALUES (?, 1, 'aac', 192000, 2)
	`, movieID)
	if err != nil {
		t.Fatalf("insert audio stream: %v", err)
	}

	_, err = app.createHLSSession(ctx, movieID, "720p_3mbps", 1, 0)
	if err == nil {
		t.Fatal("expected error when audio track index out of range")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("error = %v, want out of range", err)
	}
}
