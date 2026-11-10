// Package testutil provides helpers shared by Phorga's tests.
package testutil

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// WriteJPEG writes a small solid-color JPEG without EXIF data to dir/name.
func WriteJPEG(t testing.TB, dir, name string, c color.RGBA) string {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, solid(c), &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return write(t, dir, name, buf.Bytes())
}

// WriteJPEGWithEXIF writes a small solid-color JPEG to dir/name whose EXIF
// DateTimeOriginal is set to dt.
func WriteJPEGWithEXIF(t testing.TB, dir, name string, c color.RGBA, dt time.Time) string {
	t.Helper()
	var img bytes.Buffer
	if err := jpeg.Encode(&img, solid(c), &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	raw := img.Bytes()

	exifData := exifAPP1(dt)
	var out bytes.Buffer
	out.Write([]byte{0xFF, 0xD8}) // SOI
	out.Write([]byte{0xFF, 0xE1}) // APP1
	length := uint16(len(exifData) + 2)
	out.Write([]byte{byte(length >> 8), byte(length)})
	out.Write(exifData)
	out.Write(raw[2:]) // encoded image without its own SOI
	return write(t, dir, name, out.Bytes())
}

// exifAPP1 builds a minimal little-endian EXIF block containing only
// DateTimeOriginal (tag 0x9003) in the Exif sub-IFD.
func exifAPP1(dt time.Time) []byte {
	value := append([]byte(dt.Format("2006:01:02 15:04:05")), 0)

	le := func(v uint32) []byte {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, v)
		return b
	}

	var b bytes.Buffer
	b.WriteString("Exif\x00\x00")
	// TIFF header: little-endian, magic 42, IFD0 at offset 8.
	b.Write([]byte{'I', 'I', 0x2A, 0x00})
	b.Write(le(8))
	// IFD0 (offset 8): one entry, ExifIFD pointer -> offset 26.
	b.Write([]byte{0x01, 0x00, 0x69, 0x87, 0x04, 0x00})
	b.Write(le(1))
	b.Write(le(26))
	b.Write(le(0))
	// Exif IFD (offset 26): one entry, DateTimeOriginal ASCII at offset 44.
	b.Write([]byte{0x01, 0x00, 0x03, 0x90, 0x02, 0x00})
	b.Write(le(uint32(len(value))))
	b.Write(le(44))
	b.Write(le(0))
	b.Write(value)
	return b.Bytes()
}

func solid(c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func write(t testing.TB, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
