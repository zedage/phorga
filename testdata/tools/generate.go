// Package main generates sample test images for the Phorga demo/test setup.
// Run: go run testdata/tools/generate.go
// This creates various image types in data/source/ and data/target/ for testing.
package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"time"
)

func main() {
	sourceDir := filepath.Join("data", "source")
	targetDir := filepath.Join("data", "target")

	// Clean and recreate
	for _, dir := range []string{sourceDir, targetDir, "data/duplicates", "data/fallback", "data/state"} {
		os.RemoveAll(dir)
		os.MkdirAll(dir, 0755)
	}

	fmt.Println("Generating test images...")

	// --- SOURCE images (to be organized) ---

	// 1. JPEG photos with EXIF-like dates (embedded in JFIF comment as simulation)
	generateJPEG(filepath.Join(sourceDir, "vacation-beach.jpg"), 640, 480,
		color.RGBA{R: 64, G: 164, B: 220, A: 255}, "wave",
		time.Date(2024, 7, 15, 14, 30, 0, 0, time.UTC))

	generateJPEG(filepath.Join(sourceDir, "vacation-sunset.jpg"), 640, 480,
		color.RGBA{R: 255, G: 128, B: 50, A: 255}, "gradient",
		time.Date(2024, 7, 15, 20, 45, 0, 0, time.UTC))

	generateJPEG(filepath.Join(sourceDir, "christmas-dinner.jpg"), 800, 600,
		color.RGBA{R: 180, G: 30, B: 30, A: 255}, "dots",
		time.Date(2024, 12, 25, 18, 0, 0, 0, time.UTC))

	generateJPEG(filepath.Join(sourceDir, "newyear-fireworks.jpg"), 800, 600,
		color.RGBA{R: 20, G: 20, B: 80, A: 255}, "sparkle",
		time.Date(2025, 1, 1, 0, 5, 0, 0, time.UTC))

	generateJPEG(filepath.Join(sourceDir, "spring-garden.jpg"), 640, 480,
		color.RGBA{R: 50, G: 180, B: 50, A: 255}, "checker",
		time.Date(2025, 4, 10, 11, 0, 0, 0, time.UTC))

	// 2. PNG images
	generatePNG(filepath.Join(sourceDir, "screenshot-desktop.png"), 1024, 768,
		color.RGBA{R: 40, G: 40, B: 40, A: 255}, "grid",
		time.Date(2025, 3, 5, 9, 15, 0, 0, time.UTC))

	generatePNG(filepath.Join(sourceDir, "diagram-arch.png"), 512, 512,
		color.RGBA{R: 255, G: 255, B: 255, A: 255}, "blocks",
		time.Date(2025, 2, 20, 16, 30, 0, 0, time.UTC))

	// 3. Duplicate: same visual content as vacation-beach.jpg but different filename
	generateJPEG(filepath.Join(sourceDir, "IMG_20240715_copy.jpg"), 640, 480,
		color.RGBA{R: 64, G: 164, B: 220, A: 255}, "wave",
		time.Date(2024, 7, 15, 14, 30, 0, 0, time.UTC))

	// 4. Duplicate with same filename as an existing one (filename-based dup detection)
	generateJPEG(filepath.Join(sourceDir, "christmas-dinner-dup.jpg"), 800, 600,
		color.RGBA{R: 180, G: 30, B: 30, A: 255}, "dots",
		time.Date(2024, 12, 25, 18, 0, 0, 0, time.UTC))

	// 5. File without meaningful metadata (will use mtime fallback)
	generatePNG(filepath.Join(sourceDir, "no-metadata.png"), 256, 256,
		color.RGBA{R: 128, G: 128, B: 128, A: 255}, "solid",
		time.Time{}) // zero time = current time

	// 6. Nested source directory
	nestedDir := filepath.Join(sourceDir, "phone-backup")
	os.MkdirAll(nestedDir, 0755)
	generateJPEG(filepath.Join(nestedDir, "selfie.jpg"), 480, 640,
		color.RGBA{R: 200, G: 180, B: 160, A: 255}, "sparkle",
		time.Date(2025, 5, 1, 12, 0, 0, 0, time.UTC))

	generateJPEG(filepath.Join(nestedDir, "food-photo.jpg"), 640, 480,
		color.RGBA{R: 200, G: 150, B: 80, A: 255}, "dots",
		time.Date(2025, 5, 2, 19, 30, 0, 0, time.UTC))

	// 7. Non-image file (should be skipped)
	os.WriteFile(filepath.Join(sourceDir, "notes.txt"), []byte("This is not an image and should be skipped by Phorga."), 0644)

	// --- TARGET images (pre-existing, to test duplicate handling) ---

	// Pre-existing file with same name as a source file
	targetSubDir := filepath.Join(targetDir, "2024", "12")
	os.MkdirAll(targetSubDir, 0755)
	generateJPEG(filepath.Join(targetSubDir, "christmas-dinner.jpg"), 800, 600,
		color.RGBA{R: 160, G: 20, B: 20, A: 255}, "dots",
		time.Date(2024, 12, 25, 17, 0, 0, 0, time.UTC))

	fmt.Println("Generated test images:")
	filepath.Walk("data", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		fmt.Printf("  %s (%d bytes)\n", path, info.Size())
		return nil
	})

	fmt.Println("\nTest setup ready! Use 'go run testdata/tools/generate.go' to reset at any time.")
}

// --- Image generators ---

func generateJPEG(path string, w, h int, base color.RGBA, pattern string, mtime time.Time) {
	img := createPatternImage(w, h, base, pattern)

	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating %s: %v\n", path, err)
		return
	}
	defer f.Close()

	// Write minimal EXIF with DateTimeOriginal if mtime is set
	if !mtime.IsZero() {
		writeMinimalJPEGWithEXIF(f, img, mtime)
	} else {
		jpeg.Encode(f, img, &jpeg.Options{Quality: 85})
	}

	// Set file modification time
	if !mtime.IsZero() {
		os.Chtimes(path, mtime, mtime)
	}

	fmt.Printf("  Created: %s\n", path)
}

func generatePNG(path string, w, h int, base color.RGBA, pattern string, mtime time.Time) {
	img := createPatternImage(w, h, base, pattern)

	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating %s: %v\n", path, err)
		return
	}
	defer f.Close()

	png.Encode(f, img)

	if !mtime.IsZero() {
		os.Chtimes(path, mtime, mtime)
	}

	fmt.Printf("  Created: %s\n", path)
}

func createPatternImage(w, h int, base color.RGBA, pattern string) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := base
			switch pattern {
			case "solid":
				// no change
			case "gradient":
				factor := float64(y) / float64(h)
				c.R = uint8(float64(c.R) * (1 - factor*0.5))
				c.G = uint8(float64(c.G) * (1 - factor*0.5))
				c.B = uint8(float64(c.B) * (1 - factor*0.3))
			case "checker":
				if (x/32+y/32)%2 == 0 {
					c.R = 255 - c.R
					c.G = 255 - c.G
					c.B = 255 - c.B
				}
			case "wave":
				offset := int(20 * math.Sin(float64(x)*0.05))
				if (y+offset)%40 < 20 {
					c.R = uint8(min255(int(c.R) + 40))
					c.G = uint8(min255(int(c.G) + 40))
				}
			case "dots":
				cx, cy := x%48-24, y%48-24
				if cx*cx+cy*cy < 100 {
					c.R = uint8(min255(int(c.R) + 80))
					c.G = uint8(min255(int(c.G) + 80))
					c.B = uint8(min255(int(c.B) + 80))
				}
			case "grid":
				if x%64 < 2 || y%64 < 2 {
					c = color.RGBA{R: 80, G: 80, B: 80, A: 255}
				}
			case "blocks":
				bx, by := x/64, y/64
				if (bx+by)%3 == 0 {
					c = color.RGBA{R: 100, G: 149, B: 237, A: 255} // cornflower blue
				} else if (bx+by)%3 == 1 {
					c = color.RGBA{R: 240, G: 240, B: 240, A: 255} // light gray
				}
			case "sparkle":
				hash := uint32(x*2654435761+y*40503) >> 28
				if hash == 0 {
					c = color.RGBA{R: 255, G: 255, B: 200, A: 255}
				}
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func min255(v int) int {
	if v > 255 {
		return 255
	}
	return v
}

// writeMinimalJPEGWithEXIF writes a JPEG with a minimal EXIF header containing DateTimeOriginal.
func writeMinimalJPEGWithEXIF(f *os.File, img image.Image, dt time.Time) {
	// Format EXIF datetime
	dtStr := dt.Format("2006:01:02 15:04:05")
	dtBytes := append([]byte(dtStr), 0) // null-terminated

	// Build minimal EXIF IFD
	// TIFF header (little-endian)
	tiffHeader := []byte{
		'I', 'I', // little-endian
		0x2A, 0x00, // TIFF magic
		0x08, 0x00, 0x00, 0x00, // offset to IFD0
	}

	// IFD0 with one entry pointing to ExifIFD
	// ExifIFD offset = 8 (tiff header) + 2 (count) + 12 (entry) + 4 (next IFD) = 26
	exifIFDOffset := uint32(26)
	ifd0 := []byte{
		0x01, 0x00, // 1 entry
		// Entry: ExifIFD pointer (tag 0x8769)
		0x69, 0x87, // tag
		0x04, 0x00, // type LONG
		0x01, 0x00, 0x00, 0x00, // count
	}
	ifd0 = append(ifd0, uint32ToLE(exifIFDOffset)...)
	ifd0 = append(ifd0, 0x00, 0x00, 0x00, 0x00) // next IFD = 0

	// ExifIFD with DateTimeOriginal (tag 0x9003)
	// Value offset = 26 + 2 + 12 + 4 = 44
	valueOffset := uint32(44)
	exifIFD := []byte{
		0x01, 0x00, // 1 entry
		// Entry: DateTimeOriginal
		0x03, 0x90, // tag 0x9003
		0x02, 0x00, // type ASCII
	}
	exifIFD = append(exifIFD, uint32ToLE(uint32(len(dtBytes)))...)
	exifIFD = append(exifIFD, uint32ToLE(valueOffset)...)
	exifIFD = append(exifIFD, 0x00, 0x00, 0x00, 0x00) // next IFD = 0

	// Assemble EXIF data
	exifData := []byte("Exif\x00\x00") // EXIF header
	exifData = append(exifData, tiffHeader...)
	exifData = append(exifData, ifd0...)
	exifData = append(exifData, exifIFD...)
	exifData = append(exifData, dtBytes...)

	// Write JPEG with APP1 (EXIF) marker
	// SOI
	f.Write([]byte{0xFF, 0xD8})
	// APP1
	app1Len := uint16(len(exifData) + 2)
	f.Write([]byte{0xFF, 0xE1})
	f.Write([]byte{byte(app1Len >> 8), byte(app1Len)})
	f.Write(exifData)

	// Now encode the actual image data — we need to write a valid JPEG after the EXIF
	// Use a temp buffer
	tmpFile, _ := os.CreateTemp("", "phorg-*.jpg")
	tmpPath := tmpFile.Name()
	jpeg.Encode(tmpFile, img, &jpeg.Options{Quality: 85})
	tmpFile.Close()

	// Read back, skip SOI (first 2 bytes), append rest
	raw, _ := os.ReadFile(tmpPath)
	os.Remove(tmpPath)
	if len(raw) > 2 {
		f.Write(raw[2:]) // skip SOI of inner JPEG
	}
}

func uint32ToLE(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}
