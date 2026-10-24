package itunes

import (
	"context"
	"encoding/binary"
	"io"

	"igloo/cmd/internal/isobmff"
)

// CatalogIDs are the Apple catalog ids an iTunes Store purchase carries: the
// track's own id (the cnID atom) and its album's collection id (plID). Zero
// means the file does not carry that id.
type CatalogIDs struct {
	Track int64
	Album int64
}

// ReadCatalogIDs reads the catalog ids from an MP4 file's iTunes metadata
// (moov/udta/meta/ilst). A file without that metadata has no ids and no
// error; an error means the box structure itself is unreadable.
func ReadCatalogIDs(ctx context.Context, r io.ReaderAt, size int64) (CatalogIDs, error) {
	moov, err := isobmff.ReadMoov(ctx, r, size)
	if err != nil {
		return CatalogIDs{}, err
	}
	udta, found, err := isobmff.FindChild(moov, 0, len(moov), "udta")
	if err != nil || !found {
		return CatalogIDs{}, err
	}
	meta, found, err := isobmff.FindChild(moov, udta.PayloadStart, udta.End, "meta")
	if err != nil || !found {
		return CatalogIDs{}, err
	}
	// iTunes writes meta as a full box, with a version and flags word ahead of
	// its children; QuickTime-style meta starts with its hdlr child instead.
	childrenStart := meta.PayloadStart
	quickTime := meta.End-childrenStart >= 8 && string(moov[childrenStart+4:childrenStart+8]) == "hdlr"
	if !quickTime {
		childrenStart += 4
	}
	if childrenStart > meta.End {
		return CatalogIDs{}, nil
	}
	ilst, found, err := isobmff.FindChild(moov, childrenStart, meta.End, "ilst")
	if err != nil || !found {
		return CatalogIDs{}, err
	}
	items, err := isobmff.ChildBoxes(moov, ilst.PayloadStart, ilst.End)
	if err != nil {
		return CatalogIDs{}, err
	}

	var ids CatalogIDs
	for _, item := range items {
		var target *int64
		switch item.Type {
		case "cnID":
			target = &ids.Track
		case "plID":
			target = &ids.Album
		default:
			continue
		}
		value, err := integerItem(moov, item)
		if err != nil {
			return CatalogIDs{}, err
		}
		*target = value
	}
	return ids, nil
}

// integerItem reads an ilst item's data box as a big-endian integer. The data
// payload is a type word and a locale word ahead of the value; a value of an
// unexpected width, or not positive, reads as absent.
func integerItem(moov []byte, item isobmff.Box) (int64, error) {
	data, found, err := isobmff.FindChild(moov, item.PayloadStart, item.End, "data")
	if err != nil || !found {
		return 0, err
	}
	value := moov[data.PayloadStart:data.End]
	if len(value) < 8 {
		return 0, nil
	}
	value = value[8:]
	var id int64
	switch len(value) {
	case 4:
		id = int64(binary.BigEndian.Uint32(value))
	case 8:
		id = int64(binary.BigEndian.Uint64(value))
	default:
		return 0, nil
	}
	return max(id, 0), nil
}
