// Package itunestest builds MP4 files carrying iTunes catalog atoms, laid out
// like an iTunes Store purchase, for tests of code that reads them.
package itunestest

import (
	"bytes"

	"igloo/cmd/internal/ffmpeg/fmp4testutil"
	"igloo/cmd/internal/itunes"
)

// M4A returns an MP4 file whose ilst carries the given ids: cnID as the 4-byte
// value and plID as the 8-byte value iTunes writes. A zero id is left out.
// moovAtEnd places moov after mdat, the layout of a file that was not
// optimized for streaming.
func M4A(ids itunes.CatalogIDs, moovAtEnd bool) []byte {
	var items [][]byte
	if ids.Track != 0 {
		items = append(items, item("cnID", fmp4testutil.U32(uint32(ids.Track))))
	}
	if ids.Album != 0 {
		items = append(items, item("plID", fmp4testutil.U64(uint64(ids.Album))))
	}
	hdlr := fmp4testutil.Box("hdlr", fmp4testutil.FullBoxPayload(0, fmp4testutil.U32(0), []byte("mdir"), []byte("appl"), make([]byte, 9)))
	meta := fmp4testutil.Box("meta", fmp4testutil.FullBoxPayload(0, hdlr, fmp4testutil.Box("ilst", items...)))
	moov := fmp4testutil.Box("moov", fmp4testutil.Box("udta", meta))
	ftyp := fmp4testutil.Box("ftyp", []byte("M4A "), fmp4testutil.U32(0), []byte("M4A mp42isom"))
	mdat := fmp4testutil.Box("mdat", make([]byte, 32))
	if moovAtEnd {
		return bytes.Join([][]byte{ftyp, mdat, moov}, nil)
	}
	return bytes.Join([][]byte{ftyp, moov, mdat}, nil)
}

// item is an ilst integer item: a data box whose type word marks a signed
// big-endian integer (21), then a zero locale, then the value.
func item(atom string, value []byte) []byte {
	return fmp4testutil.Box(atom, fmp4testutil.Box("data", fmp4testutil.U32(21), fmp4testutil.U32(0), value))
}
