package isobmff_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"testing"

	"igloo/cmd/internal/ffmpeg/fmp4testutil"
	"igloo/cmd/internal/isobmff"
)

func TestReadMoovFindsMoovAtEitherEnd(t *testing.T) {
	moov := fmp4testutil.Box("moov", fmp4testutil.Box("udta"))
	ftyp := fmp4testutil.Box("ftyp", []byte("M4A "))
	mdat := fmp4testutil.Box("mdat", make([]byte, 64))
	layouts := map[string][]byte{
		"front": bytes.Join([][]byte{ftyp, moov, mdat}, nil),
		"back":  bytes.Join([][]byte{ftyp, mdat, moov}, nil),
	}
	for name, file := range layouts {
		t.Run(name, func(t *testing.T) {
			payload, err := isobmff.ReadMoov(context.Background(), bytes.NewReader(file), int64(len(file)))
			if err != nil {
				t.Fatalf("ReadMoov returned error: %v", err)
			}
			if !bytes.Equal(payload, moov[8:]) {
				t.Fatalf("payload = %x, want %x", payload, moov[8:])
			}
		})
	}
}

func TestReadMoovRejectsMissingAndOversizedMoov(t *testing.T) {
	file := bytes.Join([][]byte{fmp4testutil.Box("ftyp", []byte("M4A ")), fmp4testutil.Box("mdat", make([]byte, 8))}, nil)
	_, err := isobmff.ReadMoov(context.Background(), bytes.NewReader(file), int64(len(file)))
	if err == nil {
		t.Fatal("a file without moov did not error")
	}

	// The declared moov fits the declared file size, so only the payload cap
	// can reject it, and it must do so before reading the payload.
	header := make([]byte, 16)
	binary.BigEndian.PutUint32(header[0:4], isobmff.MaxMoovPayloadBytes+16)
	copy(header[4:8], "moov")
	_, err = isobmff.ReadMoov(context.Background(), bytes.NewReader(header), isobmff.MaxMoovPayloadBytes+16)
	if !errors.Is(err, isobmff.ErrMoovTooLarge) {
		t.Fatalf("error = %v, want ErrMoovTooLarge", err)
	}
}

func TestFindChildWalksOnlyDirectChildren(t *testing.T) {
	data := bytes.Join([][]byte{
		fmp4testutil.Box("udta", fmp4testutil.Box("ilst")),
		fmp4testutil.Box("ilst", []byte{1, 2}),
	}, nil)
	box, found, err := isobmff.FindChild(data, 0, len(data), "ilst")
	if err != nil || !found {
		t.Fatalf("FindChild = found %v, error %v", found, err)
	}
	got := data[box.PayloadStart:box.End]
	if !bytes.Equal(got, []byte{1, 2}) {
		t.Fatalf("payload = %v, want the top-level ilst's", got)
	}

	binary.BigEndian.PutUint32(data[0:4], uint32(len(data)+1))
	_, err = isobmff.ChildBoxes(data, 0, len(data))
	if err == nil {
		t.Fatal("a box overrunning its parent did not error")
	}
}
