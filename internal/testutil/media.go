package testutil

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

// isoBox returns one ISO base media box with a 32-bit size.
func isoBox(typ string, payload ...[]byte) []byte {
	var body []byte
	for _, p := range payload {
		body = append(body, p...)
	}
	out := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(out, uint32(8+len(body)))
	copy(out[4:], typ)
	return append(out, body...)
}

// fullBoxHeader is the version and flags of a full box.
func fullBoxHeader(version byte) []byte { return []byte{version, 0, 0, 0} }

func be16(v uint16) []byte { b := make([]byte, 2); binary.BigEndian.PutUint16(b, v); return b }
func be32(v uint32) []byte { b := make([]byte, 4); binary.BigEndian.PutUint32(b, v); return b }
func be64(v uint64) []byte { b := make([]byte, 8); binary.BigEndian.PutUint64(b, v); return b }

// infeV2 returns an item info entry (version 2) for the given item.
func infeV2(id uint16, typ string) []byte {
	return isoBox("infe", fullBoxHeader(2), be16(id), be16(0), []byte(typ), []byte{0})
}

// WriteHEIC writes a minimal HEIF file (ftyp, meta with iinf/iloc, mdat)
// to dir/name. When dt is not zero, the file has an Exif item whose
// DateTimeOriginal is dt. With inIdat, the Exif item is stored in the meta
// box's idat (iloc construction method 1) instead of the mdat. There is no
// decodable image in it.
func WriteHEIC(t testing.TB, dir, name string, dt time.Time, inIdat bool) string {
	t.Helper()
	ftyp := isoBox("ftyp", []byte("heic"), be32(0), []byte("mif1heic"))
	hdlr := isoBox("hdlr", fullBoxHeader(0), be32(0), []byte("pict"), make([]byte, 12), []byte{0})

	if dt.IsZero() {
		iinf := isoBox("iinf", fullBoxHeader(0), be16(1), infeV2(1, "hvc1"))
		meta := isoBox("meta", fullBoxHeader(0), hdlr, iinf)
		return write(t, dir, name, concat(ftyp, meta, isoBox("mdat", []byte("image data"))))
	}

	// The Exif item: offset to the TIFF header, then "Exif\0\0" + TIFF.
	exifBlock := exifAPP1(dt)
	item := concat(be32(6), exifBlock)

	iinf := isoBox("iinf", fullBoxHeader(0), be16(2), infeV2(1, "hvc1"), infeV2(2, "Exif"))
	iloc := func(method uint16, offset uint32) []byte {
		return isoBox("iloc", fullBoxHeader(1),
			[]byte{0x44, 0x00}, // offset and length 4 bytes; no base offset or index
			be16(1),            // one item
			be16(2),            // item ID 2 (Exif)
			be16(method),       // construction method
			be16(0),            // data reference index
			be16(1),            // one extent
			be32(offset), be32(uint32(len(item))),
		)
	}

	if inIdat {
		meta := isoBox("meta", fullBoxHeader(0), hdlr, iinf, iloc(1, 0), isoBox("idat", item))
		return write(t, dir, name, concat(ftyp, meta, isoBox("mdat", []byte("image data"))))
	}
	// The item's file offset depends on the size of meta, which does not
	// depend on the offset value; build once to measure.
	metaLen := len(isoBox("meta", fullBoxHeader(0), hdlr, iinf, iloc(0, 0)))
	offset := uint32(len(ftyp) + metaLen + 8) // + mdat header
	meta := isoBox("meta", fullBoxHeader(0), hdlr, iinf, iloc(0, offset))
	return write(t, dir, name, concat(ftyp, meta, isoBox("mdat", item)))
}

// WriteMP4 writes a minimal MP4 file whose movie header (mvhd) has the
// creation time created, in mvhd version 0 or 1. A zero created writes 0,
// as some cameras do. With moovLast, the moov box follows the mdat box, as
// in most camera files.
func WriteMP4(t testing.TB, dir, name string, created time.Time, version byte, moovLast bool) string {
	t.Helper()
	var secs uint64
	if !created.IsZero() {
		secs = uint64(created.Sub(time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)) / time.Second)
	}
	var times []byte
	if version == 1 {
		times = concat(be64(secs), be64(secs), be32(1000), be64(0))
	} else {
		times = concat(be32(uint32(secs)), be32(uint32(secs)), be32(1000), be32(0))
	}
	mvhd := isoBox("mvhd", fullBoxHeader(version), times, make([]byte, 80))
	moov := isoBox("moov", mvhd)
	ftyp := isoBox("ftyp", []byte("isom"), be32(0x200), []byte("isomiso2mp41"))
	mdat := isoBox("mdat", bytes.Repeat([]byte{0xAB}, 64))
	if moovLast {
		return write(t, dir, name, concat(ftyp, mdat, moov))
	}
	return write(t, dir, name, concat(ftyp, moov, mdat))
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
