package itunes_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	"igloo/cmd/internal/ffmpeg/fmp4testutil"
	"igloo/cmd/internal/itunes"
	"igloo/cmd/internal/itunes/itunestest"
)

func readCatalogIDs(t *testing.T, file []byte) (itunes.CatalogIDs, error) {
	t.Helper()
	return itunes.ReadCatalogIDs(context.Background(), bytes.NewReader(file), int64(len(file)))
}

func TestReadCatalogIDsReadsPurchaseAtoms(t *testing.T) {
	want := itunes.CatalogIDs{Track: 1432044890, Album: 1432044881}
	for name, moovAtEnd := range map[string]bool{"moov first": false, "moov last": true} {
		t.Run(name, func(t *testing.T) {
			got, err := readCatalogIDs(t, itunestest.M4A(want, moovAtEnd))
			if err != nil {
				t.Fatalf("ReadCatalogIDs returned error: %v", err)
			}
			if got != want {
				t.Fatalf("ids = %+v, want %+v", got, want)
			}
		})
	}
}

func TestReadCatalogIDsWithoutAtomsHasNoIDs(t *testing.T) {
	cases := map[string][]byte{
		"no ids": itunestest.M4A(itunes.CatalogIDs{}, false),
		"no udta": bytes.Join([][]byte{
			fmp4testutil.Box("ftyp", []byte("M4A ")),
			fmp4testutil.Box("moov", fmp4testutil.Box("mvhd", make([]byte, 100))),
		}, nil),
		"QuickTime meta without ilst": fmp4testutil.Box("moov", fmp4testutil.Box("udta", fmp4testutil.Box("meta",
			fmp4testutil.Box("hdlr", make([]byte, 24)),
		))),
	}
	for name, file := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := readCatalogIDs(t, file)
			if err != nil {
				t.Fatalf("ReadCatalogIDs returned error: %v", err)
			}
			if got != (itunes.CatalogIDs{}) {
				t.Fatalf("ids = %+v, want none", got)
			}
		})
	}
}

func TestReadCatalogIDsReadsUnsignedFourByteIDs(t *testing.T) {
	for _, atom := range []string{"cnID", "plID"} {
		t.Run(atom, func(t *testing.T) {
			for name, value := range map[string]uint32{
				"max signed":   1<<31 - 1,
				"above signed": 1 << 31,
				"max unsigned": 1<<32 - 1,
			} {
				t.Run(name, func(t *testing.T) {
					data := fmp4testutil.Box("data", fmp4testutil.U32(21), fmp4testutil.U32(0), fmp4testutil.U32(value))
					meta := fmp4testutil.Box("meta", fmp4testutil.FullBoxPayload(0,
						fmp4testutil.Box("ilst", fmp4testutil.Box(atom, data)),
					))
					file := fmp4testutil.Box("moov", fmp4testutil.Box("udta", meta))
					var want itunes.CatalogIDs
					if atom == "cnID" {
						want.Track = int64(value)
					} else {
						want.Album = int64(value)
					}
					got, err := readCatalogIDs(t, file)
					if err != nil {
						t.Fatalf("ReadCatalogIDs returned error: %v", err)
					}
					if got != want {
						t.Fatalf("ids = %+v, want %+v", got, want)
					}
				})
			}
		})
	}
}

func TestReadCatalogIDsReadsQuickTimeMeta(t *testing.T) {
	data := fmp4testutil.Box("data", fmp4testutil.U32(21), fmp4testutil.U32(0), fmp4testutil.U32(7))
	file := fmp4testutil.Box("moov", fmp4testutil.Box("udta", fmp4testutil.Box("meta",
		fmp4testutil.Box("hdlr", make([]byte, 24)),
		fmp4testutil.Box("ilst", fmp4testutil.Box("plID", data)),
	)))
	got, err := readCatalogIDs(t, file)
	if err != nil {
		t.Fatalf("ReadCatalogIDs returned error: %v", err)
	}
	if got != (itunes.CatalogIDs{Album: 7}) {
		t.Fatalf("ids = %+v, want album 7", got)
	}
}

func TestReadCatalogIDsRejectsBrokenFiles(t *testing.T) {
	file := itunestest.M4A(itunes.CatalogIDs{Track: 1, Album: 2}, false)
	cases := map[string][]byte{
		"not an MP4": []byte("ID3\x04\x00\x00\x00\x00\x00\x00 mp3 bytes"),
		"truncated":  file[:len(file)/2],
	}
	// An ilst item claiming more bytes than its parent holds.
	overrun := bytes.Clone(file)
	at := bytes.Index(overrun, []byte("cnID")) - 4
	binary.BigEndian.PutUint32(overrun[at:at+4], 1<<20)
	cases["item overruns ilst"] = overrun
	for name, broken := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := readCatalogIDs(t, broken)
			if err == nil {
				t.Fatal("ReadCatalogIDs did not error")
			}
		})
	}
}
