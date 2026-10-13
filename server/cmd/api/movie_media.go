package main

import (
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

// mp4ContentType is what an MP4 container resolves to. Direct playback is
// offered for MP4 alone, so a watch room compares videoContentType's answer
// against it.
const mp4ContentType = "video/mp4"

// videoContentType resolves the single MIME type a video file is described by,
// for the direct stream endpoints, the technical details and the watch-room
// direct-play check alike: the client's direct-play decision and the server's
// validation only agree while they read the same string. The container map
// wins; the stored mime_type covers containers it lacks.
func videoContentType(container string, storedMimeType string) string {
	contentType := helpers.VideoMimeTypes[container]
	if contentType == "" {
		return storedMimeType
	}
	return contentType
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
		if !helpers.IsPlaybackCoverArtVideoCodec(streams[i].Codec) {
			return &streams[i]
		}
	}

	return &streams[0]
}
