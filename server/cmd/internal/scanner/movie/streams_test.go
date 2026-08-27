package movie

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"

	_ "github.com/mattn/go-sqlite3"
)

func TestProcessMovieStreamsPersistsDispositions(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()
	ctx := context.Background()

	movie, err := testScanner.queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Disposition Movie",
		FilePath:  "/movies/Disposition.Movie.2024.mp4",
		FileName:  "Disposition.Movie.2024.mp4",
		Size:      1024,
		Container: "mp4",
		MimeType:  helpers.VideoMimeTypes["mp4"],
	})
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	fixture := movieScannerMetadataFixture("120")
	fixture.Streams = append(fixture.Streams,
		ffprobe.Stream{
			Index:       4,
			CodecName:   "aac",
			CodecType:   "audio",
			Channels:    2,
			Tags:        ffprobe.StreamTags{Language: "eng", Title: "Main"},
			Disposition: ffprobe.StreamDisposition{Default: 1},
		},
		ffprobe.Stream{
			Index:       5,
			CodecName:   "subrip",
			CodecType:   "subtitle",
			Tags:        ffprobe.StreamTags{Language: "eng", Title: "Signs"},
			Disposition: ffprobe.StreamDisposition{Forced: 1, Default: 1},
		},
	)

	_, err = testScanner.scanner.processMovieStreams(ctx, testScanner.queries, movie.ID, fixture.Streams)
	if err != nil {
		t.Fatalf("process movie streams: %v", err)
	}

	audioStreams, err := testScanner.queries.GetAudioStreamsByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get audio streams: %v", err)
	}
	if len(audioStreams) != 2 {
		t.Fatalf("audio stream count = %d, want 2", len(audioStreams))
	}
	if audioStreams[0].IsDefault {
		t.Error("first audio stream (no disposition) persisted is_default=true, want false")
	}
	if !audioStreams[1].IsDefault {
		t.Error("default-flagged audio stream persisted is_default=false, want true")
	}

	subtitles, err := testScanner.queries.GetSubtitlesByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get subtitles: %v", err)
	}
	if len(subtitles) != 2 {
		t.Fatalf("subtitle count = %d, want 2", len(subtitles))
	}
	if subtitles[0].IsForced || subtitles[0].IsDefault {
		t.Error("plain subtitle persisted disposition flags, want none")
	}
	if !subtitles[1].IsForced || !subtitles[1].IsDefault {
		t.Error("forced+default subtitle lost its flags")
	}
}

// field_order and the display-matrix rotation feed the deinterlace and remux
// decisions, so the scanner must persist them faithfully — including the
// difference between an explicit 0-degree matrix and no matrix at all.
func TestProcessMovieStreamsPersistsFieldOrderAndRotation(t *testing.T) {
	tests := []struct {
		name         string
		fieldOrder   string
		sideData     []ffprobe.StreamSideData
		wantOrder    sql.NullString
		wantRotation sql.NullInt64
	}{
		{
			name:         "interlaced with rotation",
			fieldOrder:   "tt",
			sideData:     []ffprobe.StreamSideData{{SideDataType: "Display Matrix", Rotation: -90}},
			wantOrder:    sql.NullString{String: "tt", Valid: true},
			wantRotation: sql.NullInt64{Int64: -90, Valid: true},
		},
		{
			name:         "explicit zero-degree matrix stays distinguishable",
			fieldOrder:   "progressive",
			sideData:     []ffprobe.StreamSideData{{SideDataType: "Display Matrix", Rotation: 0}},
			wantOrder:    sql.NullString{String: "progressive", Valid: true},
			wantRotation: sql.NullInt64{Int64: 0, Valid: true},
		},
		{
			name: "absent metadata persists as NULL",
		},
		{
			name:     "non-matrix side data carries no rotation",
			sideData: []ffprobe.StreamSideData{{SideDataType: "H.26[45] User Data Unregistered SEI message"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testScanner := setupMovieScanner(t)
			defer testScanner.db.Close()
			ctx := context.Background()

			movie, err := testScanner.queries.UpsertMovie(ctx, database.UpsertMovieParams{
				Title:     "Field Order Movie",
				FilePath:  "/movies/Field.Order.Movie.2024.mp4",
				FileName:  "Field.Order.Movie.2024.mp4",
				Size:      1024,
				Container: "mp4",
				MimeType:  helpers.VideoMimeTypes["mp4"],
			})
			if err != nil {
				t.Fatalf("insert movie: %v", err)
			}

			fixture := movieScannerMetadataFixture("120")
			fixture.Streams[0].FieldOrder = tt.fieldOrder
			fixture.Streams[0].SideDataList = tt.sideData

			_, err = testScanner.scanner.processMovieStreams(ctx, testScanner.queries, movie.ID, fixture.Streams)
			if err != nil {
				t.Fatalf("process movie streams: %v", err)
			}

			videoStreams, err := testScanner.queries.GetVideoStreamsByMovieID(ctx, movie.ID)
			if err != nil {
				t.Fatalf("get video streams: %v", err)
			}
			if len(videoStreams) == 0 {
				t.Fatal("no video streams persisted")
			}
			if videoStreams[0].FieldOrder != tt.wantOrder {
				t.Errorf("field_order = %+v, want %+v", videoStreams[0].FieldOrder, tt.wantOrder)
			}
			if videoStreams[0].Rotation != tt.wantRotation {
				t.Errorf("rotation = %+v, want %+v", videoStreams[0].Rotation, tt.wantRotation)
			}
		})
	}
}

func TestStreamIndexUniquePerMovie(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	result, err := testScanner.db.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult, duration)
		VALUES ('Unique Index Movie', '/tmp/unique-index.mkv', 'unique-index.mkv', 1, 'mkv', 'video/x-matroska', 0, 3600.0)
	`)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	movieID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("movie id: %v", err)
	}

	tables := []struct {
		name   string
		insert string
	}{
		{"video_streams", `INSERT INTO video_streams (movie_id, stream_index, codec, bit_rate, width, height, frame_rate) VALUES (?, 0, 'h264', 5000000, 1920, 1080, 23.976)`},
		{"audio_streams", `INSERT INTO audio_streams (movie_id, stream_index, codec, bit_rate, channels) VALUES (?, 1, 'aac', 192000, 2)`},
		{"subtitles", `INSERT INTO subtitles (movie_id, stream_index, codec) VALUES (?, 2, 'subrip')`},
	}
	for _, table := range tables {
		_, err = testScanner.db.Exec(table.insert, movieID)
		if err != nil {
			t.Fatalf("first %s insert: %v", table.name, err)
		}
		_, err = testScanner.db.Exec(table.insert, movieID)
		if err == nil {
			t.Fatalf("expected duplicate (movie_id, stream_index) insert into %s to fail", table.name)
		}
		if !strings.Contains(err.Error(), "UNIQUE") {
			t.Fatalf("expected UNIQUE constraint error for %s, got %v", table.name, err)
		}
	}
}
