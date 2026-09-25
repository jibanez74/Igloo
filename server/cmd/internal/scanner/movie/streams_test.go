package movie

import (
	"context"
	"database/sql"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"

	_ "github.com/mattn/go-sqlite3"
)

func TestProcessMovieStreamsPersistsDispositions(t *testing.T) {
	testScanner := setupMovieScanner(t)
	ctx := context.Background()

	movieID, err := testScanner.queries.UpsertMovie(ctx, database.UpsertMovieParams{
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
	movie, err := testScanner.queries.GetMovieByID(ctx, movieID)
	if err != nil {
		t.Fatal(err)
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

	_, err = processMovieStreams(ctx, testScanner.queries, movie.ID, fixture.Streams)
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
			ctx := context.Background()

			movieID, err := testScanner.queries.UpsertMovie(ctx, database.UpsertMovieParams{
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
			movie, err := testScanner.queries.GetMovieByID(ctx, movieID)
			if err != nil {
				t.Fatal(err)
			}

			fixture := movieScannerMetadataFixture("120")
			fixture.Streams[0].FieldOrder = tt.fieldOrder
			fixture.Streams[0].SideDataList = tt.sideData

			_, err = processMovieStreams(ctx, testScanner.queries, movie.ID, fixture.Streams)
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
