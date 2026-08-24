package main

import (
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

// movieContentType resolves the single MIME type a movie is described by:
// served as Content-Type on the direct stream endpoints, reported to the web
// client by GetMovieTechnicalDetails, and compared against "video/mp4" when a
// watch room asks for direct playback. Every caller must use this function —
// the client's direct-play decision and the server's validation of it are only
// in agreement while they read the same string.
//
// The pinned container map wins so the answer stays correct for rows scanned
// before the map existed (audit D1); the stored mime_type is the fallback for
// containers the map does not cover.
func movieContentType(container string, storedMimeType string) string {
	contentType := helpers.VideoMimeTypes[container]
	if contentType == "" {
		return storedMimeType
	}
	return contentType
}

// coverArtVideoCodecs are the still-image codecs ffprobe reports as video
// streams for embedded poster/thumbnail attachments.
var coverArtVideoCodecs = map[string]bool{
	"mjpeg": true,
	"png":   true,
	"gif":   true,
	"bmp":   true,
}

// primaryVideoStream returns the first real video stream, skipping embedded
// cover art. Mirrors getPrimaryVideoStream in the web client
// (web/src/lib/playback.ts) so a file whose attached picture sorts ahead of the
// feature cannot make client and server judge different streams. Returns nil
// for an empty slice.
func primaryVideoStream(streams []database.VideoStream) *database.VideoStream {
	if len(streams) == 0 {
		return nil
	}

	for i := range streams {
		if !coverArtVideoCodecs[strings.ToLower(strings.TrimSpace(streams[i].Codec))] {
			return &streams[i]
		}
	}

	return &streams[0]
}
