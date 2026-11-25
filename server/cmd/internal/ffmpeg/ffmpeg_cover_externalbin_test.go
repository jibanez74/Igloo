//go:build externalbin

package ffmpeg

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExternalExtractAttachedPictureCopiesTheEmbeddedBytes(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatal(err)
	}
	f := &ffmpeg{bin: candidate.path, capabilities: Capabilities{Probed: true}}
	dest := filepath.Join(t.TempDir(), "picture.tmp")

	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	err = f.ExtractAttachedPicture(ctx, filepath.Join("testdata", "embedded-cover.flac"), 1, dest)
	if err != nil {
		t.Fatalf("ExtractAttachedPicture: %v", err)
	}
	picture, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	isJPEG := bytes.HasPrefix(picture, []byte{0xff, 0xd8, 0xff}) && bytes.HasSuffix(picture, []byte{0xff, 0xd9})
	if !isJPEG {
		t.Fatalf("extracted %d bytes that are not one whole JPEG", len(picture))
	}

	err = f.ExtractAttachedPicture(ctx, filepath.Join("testdata", "embedded-cover.flac"), 5, dest)
	if err == nil {
		t.Fatal("extracting a stream the file does not have succeeded")
	}
}
