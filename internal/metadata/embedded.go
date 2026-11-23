package metadata

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// ErrNoEmbeddedDate is returned when a file has no readable embedded
// capture date.
var ErrNoEmbeddedDate = errors.New("no embedded capture date")

// Limits for the boxes read into memory, so a broken or hostile file
// cannot make Phorga allocate much.
const (
	maxIndexBox  = 4 << 20 // iinf, iloc
	maxExifBlock = 4 << 20 // the Exif item of a HEIC file
)

// quickTimeEpoch is the zero point of QuickTime and MP4 timestamps.
var quickTimeEpoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)

// ExtractEmbeddedDate reads only the capture date stored in the file itself
// and never uses the filename, path or mtime:
//
//   - JPEG and TIFF-based RAW: EXIF DateTimeOriginal (falling back to DateTime)
//   - HEIC/HEIF: the same EXIF fields, from the file's Exif item
//   - MP4/MOV: the creation time of the movie header (mvhd), converted to the
//     local time zone
//
// The duplicate checks use it as "real EXIF" (ORG-011, DUP-002). Errors
// opening or reading the file are returned as *fs.PathError.
func ExtractEmbeddedDate(path string) (time.Time, error) {
	t, _, err := extractEmbeddedDate(path)
	return t, err
}

// extractEmbeddedDate is ExtractEmbeddedDate and also returns the method
// for the log: "exif" or "quicktime" (ORG-002).
func extractEmbeddedDate(path string) (time.Time, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return time.Time{}, "", err
	}

	var head [12]byte
	if n, _ := f.ReadAt(head[:], 0); n == len(head) && string(head[4:8]) == "ftyp" {
		// ISO base media file: HEIC/HEIF, MP4 or MOV.
		bf := boxFile{r: f, size: info.Size()}
		if t, err := heicEXIFDate(bf); err == nil {
			return t, "exif", nil
		}
		if t, err := quickTimeDate(bf); err == nil {
			return t, "quicktime", nil
		}
		return time.Time{}, "", ErrNoEmbeddedDate
	}

	// JPEG or TIFF-based RAW. QuickTime files without an ftyp box (old MOV)
	// start with another box; try them too.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return time.Time{}, "", err
	}
	if t, err := exifDate(f); err == nil {
		return t, "exif", nil
	}
	if t, err := quickTimeDate(boxFile{r: f, size: info.Size()}); err == nil {
		return t, "quicktime", nil
	}
	return time.Time{}, "", ErrNoEmbeddedDate
}

// exifDate decodes EXIF from a JPEG, a TIFF or a raw EXIF block and returns
// DateTimeOriginal (falling back to DateTime).
func exifDate(r io.Reader) (time.Time, error) {
	x, err := exif.Decode(r)
	if err != nil {
		return time.Time{}, err
	}
	return x.DateTime()
}

// boxFile reads boxes of an ISO base media file (ISO/IEC 14496-12).
type boxFile struct {
	r    io.ReaderAt
	size int64
}

// box is one box: its type and where its payload starts and ends.
type box struct {
	typ        string
	start, end int64
}

// boxes returns the boxes between start and end, stopping at the first
// malformed one.
func (bf boxFile) boxes(start, end int64) []box {
	var out []box
	for off := start; off+8 <= end; {
		var h [16]byte
		if _, err := bf.r.ReadAt(h[:8], off); err != nil {
			break
		}
		size := int64(binary.BigEndian.Uint32(h[:4]))
		hdr := int64(8)
		switch size {
		case 1: // 64-bit size follows the type
			if _, err := bf.r.ReadAt(h[8:16], off+8); err != nil {
				return out
			}
			size = int64(binary.BigEndian.Uint64(h[8:16]))
			hdr = 16
		case 0: // box extends to the end
			size = end - off
		}
		if size < hdr || size > end-off {
			break
		}
		out = append(out, box{typ: string(h[4:8]), start: off + hdr, end: off + size})
		off += size
	}
	return out
}

// find returns the first box of type typ between start and end.
func (bf boxFile) find(typ string, start, end int64) (box, bool) {
	for _, b := range bf.boxes(start, end) {
		if b.typ == typ {
			return b, true
		}
	}
	return box{}, false
}

// read returns the payload of b, if it is at most limit bytes.
func (bf boxFile) read(b box, limit int64) ([]byte, error) {
	n := b.end - b.start
	if n < 0 || n > limit {
		return nil, fmt.Errorf("box %q too large (%d bytes)", b.typ, n)
	}
	buf := make([]byte, n)
	if _, err := bf.r.ReadAt(buf, b.start); err != nil {
		return nil, err
	}
	return buf, nil
}

// quickTimeDate returns the creation time from moov/mvhd. A zero or
// implausible value (before 1970 or after 2100) is no date.
func quickTimeDate(bf boxFile) (time.Time, error) {
	moov, ok := bf.find("moov", 0, bf.size)
	if !ok {
		return time.Time{}, ErrNoEmbeddedDate
	}
	mvhd, ok := bf.find("mvhd", moov.start, moov.end)
	if !ok {
		return time.Time{}, ErrNoEmbeddedDate
	}
	b, err := bf.read(mvhd, 1024)
	if err != nil {
		return time.Time{}, err
	}
	var secs uint64
	switch {
	case len(b) >= 8 && b[0] == 0:
		secs = uint64(binary.BigEndian.Uint32(b[4:8]))
	case len(b) >= 12 && b[0] == 1:
		secs = binary.BigEndian.Uint64(b[4:12])
	default:
		return time.Time{}, fmt.Errorf("unsupported mvhd box")
	}
	if secs == 0 || secs > 1<<40 {
		return time.Time{}, ErrNoEmbeddedDate
	}
	t := quickTimeEpoch.Add(time.Duration(secs) * time.Second)
	if t.Year() < 1970 || t.Year() > 2100 {
		return time.Time{}, ErrNoEmbeddedDate
	}
	// The value is UTC. EXIF dates are local wall-clock times, so convert
	// to the local zone (the TZ setting) before building folder names.
	return t.Local(), nil
}

// heicEXIFDate finds the Exif item of a HEIF file (ISO/IEC 23008-12) through
// meta/iinf and meta/iloc, and decodes its date.
func heicEXIFDate(bf boxFile) (time.Time, error) {
	meta, ok := bf.find("meta", 0, bf.size)
	if !ok {
		return time.Time{}, ErrNoEmbeddedDate
	}
	// meta is a full box: 4 bytes version and flags before its children.
	children := box{typ: "meta", start: meta.start + 4, end: meta.end}

	iinf, ok := bf.find("iinf", children.start, children.end)
	if !ok {
		return time.Time{}, ErrNoEmbeddedDate
	}
	iinfData, err := bf.read(iinf, maxIndexBox)
	if err != nil {
		return time.Time{}, err
	}
	itemID, ok := exifItemID(iinfData)
	if !ok {
		return time.Time{}, ErrNoEmbeddedDate
	}

	iloc, ok := bf.find("iloc", children.start, children.end)
	if !ok {
		return time.Time{}, ErrNoEmbeddedDate
	}
	ilocData, err := bf.read(iloc, maxIndexBox)
	if err != nil {
		return time.Time{}, err
	}
	loc, ok := itemLocation(ilocData, itemID)
	if !ok {
		return time.Time{}, ErrNoEmbeddedDate
	}

	var base int64
	switch loc.method {
	case 0: // offsets in the file
	case 1: // offsets in the idat box
		idat, ok := bf.find("idat", children.start, children.end)
		if !ok {
			return time.Time{}, ErrNoEmbeddedDate
		}
		base = idat.start
	default:
		return time.Time{}, fmt.Errorf("unsupported iloc construction method %d", loc.method)
	}

	var data []byte
	for _, e := range loc.extents {
		start := base + loc.baseOffset + e.offset
		length := e.length
		if length == 0 { // to the end of the file
			length = bf.size - start
		}
		if start < 0 || length < 0 || start+length > bf.size || int64(len(data))+length > maxExifBlock {
			return time.Time{}, fmt.Errorf("exif item outside the file")
		}
		chunk := make([]byte, length)
		if _, err := bf.r.ReadAt(chunk, start); err != nil {
			return time.Time{}, err
		}
		data = append(data, chunk...)
	}
	tiff, ok := exifPayload(data)
	if !ok {
		return time.Time{}, ErrNoEmbeddedDate
	}
	return exifDate(bytes.NewReader(tiff))
}

// exifItemID returns the ID of the item of type "Exif" in an iinf payload.
func exifItemID(b []byte) (uint32, bool) {
	if len(b) < 6 {
		return 0, false
	}
	start := 6 // version, flags, 16-bit entry count
	if b[0] != 0 {
		start = 8 // 32-bit entry count
	}
	bf := boxFile{r: bytes.NewReader(b), size: int64(len(b))}
	for _, infe := range bf.boxes(int64(start), int64(len(b))) {
		if infe.typ != "infe" {
			continue
		}
		e := b[infe.start:infe.end]
		if len(e) < 1 {
			continue
		}
		switch version := e[0]; {
		case version == 2 && len(e) >= 12:
			if string(e[8:12]) == "Exif" {
				return uint32(binary.BigEndian.Uint16(e[4:6])), true
			}
		case version == 3 && len(e) >= 14:
			if string(e[10:14]) == "Exif" {
				return binary.BigEndian.Uint32(e[4:8]), true
			}
		}
	}
	return 0, false
}

type extent struct{ offset, length int64 }

type location struct {
	method     int
	baseOffset int64
	extents    []extent
}

// itemLocation returns where the item with id is stored, from an iloc payload.
func itemLocation(b []byte, id uint32) (location, bool) {
	r := &byteReader{b: b}
	version := r.uint(1)
	r.skip(3) // flags
	sizes := r.uint(1)
	offsetSize, lengthSize := int(sizes>>4), int(sizes&0x0f)
	sizes = r.uint(1)
	baseOffsetSize, indexSize := int(sizes>>4), 0
	if version == 1 || version == 2 {
		indexSize = int(sizes & 0x0f)
	}
	var count uint64
	if version < 2 {
		count = r.uint(2)
	} else {
		count = r.uint(4)
	}
	for i := uint64(0); i < count && !r.failed; i++ {
		var itemID uint64
		if version < 2 {
			itemID = r.uint(2)
		} else {
			itemID = r.uint(4)
		}
		loc := location{}
		if version == 1 || version == 2 {
			loc.method = int(r.uint(2) & 0x0f)
		}
		r.skip(2) // data reference index
		loc.baseOffset = int64(r.uint(baseOffsetSize))
		extents := r.uint(2)
		for j := uint64(0); j < extents && !r.failed; j++ {
			if indexSize > 0 {
				r.skip(indexSize)
			}
			off := int64(r.uint(offsetSize))
			length := int64(r.uint(lengthSize))
			loc.extents = append(loc.extents, extent{offset: off, length: length})
		}
		if r.failed {
			return location{}, false
		}
		if uint32(itemID) == id {
			return loc, len(loc.extents) > 0
		}
	}
	return location{}, false
}

// exifPayload returns the TIFF data of a HEIF Exif item. The item starts
// with a 32-bit offset to the TIFF header; some writers put "Exif\0\0" in
// between, which exif.Decode accepts too.
func exifPayload(data []byte) ([]byte, bool) {
	if len(data) < 8 {
		return nil, false
	}
	off := int(binary.BigEndian.Uint32(data[:4]))
	if start := 4 + off; off >= 0 && start+4 <= len(data) && isEXIFStart(data[start:]) {
		return data[start:], true
	}
	// The offset is wrong in some files; look for the header instead.
	for i := 4; i+4 <= len(data) && i < 64; i++ {
		if isEXIFStart(data[i:]) {
			return data[i:], true
		}
	}
	return nil, false
}

func isEXIFStart(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	switch string(b[:4]) {
	case "II*\x00", "MM\x00*", "Exif":
		return true
	}
	return false
}

// byteReader reads big-endian unsigned integers of 0 to 8 bytes and
// remembers when it ran past the end.
type byteReader struct {
	b      []byte
	pos    int
	failed bool
}

func (r *byteReader) uint(n int) uint64 {
	if n < 0 || n > 8 || r.pos+n > len(r.b) {
		r.failed = true
		return 0
	}
	var v uint64
	for _, c := range r.b[r.pos : r.pos+n] {
		v = v<<8 | uint64(c)
	}
	r.pos += n
	return v
}

func (r *byteReader) skip(n int) {
	if r.pos+n > len(r.b) {
		r.failed = true
		return
	}
	r.pos += n
}
