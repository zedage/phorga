package dedup

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/nfnt/resize"
)

// scene draws a textured 512×384 "photo": overlapping waves, so the image
// has structure everywhere, like a real one. tone maps 0–1 brightness to
// 0–1 and models a brightness/gamma edit. When patch is true, a 32×24 area
// (0.4 % of the image) gets a different pattern, like a face that changed
// between burst shots.
func scene(tone func(float64) float64, patch bool) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 512, 384))
	for y := 0; y < 384; y++ {
		for x := 0; x < 512; x++ {
			fx, fy := float64(x), float64(y)
			v := 0.5 + 0.2*math.Sin(fx/18) + 0.15*math.Cos(fy/14) + 0.1*math.Sin((fx+fy)/26)
			if patch && x >= 192 && x < 224 && y >= 120 && y < 144 {
				// Fine stripes on top of the scene: the area keeps its
				// average brightness, as a face does when the eyes close.
				v += 0.15 * math.Sin((fx-fy)/3)
			}
			l := uint8(255 * math.Max(0, math.Min(1, tone(v))))
			img.Set(x, y, color.RGBA{l, uint8(float64(l) * 0.8), uint8(float64(l) * 0.6), 255})
		}
	}
	return img
}

func identity(v float64) float64 { return v }

func writeJPEG(t *testing.T, dir, name string, img image.Image, quality int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatal(err)
	}
	return path
}

// pHashDistance makes sure a "not a duplicate" test really reaches the
// detail check: the pair must pass the pHash stage.
func pHashDistance(t *testing.T, a, b string) int {
	t.Helper()
	ha, err := computePHash(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := computePHash(b)
	if err != nil {
		t.Fatal(err)
	}
	d, err := ha.Distance(hb)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// Spec: DUP-014
func TestDedup_ContentMode_BurstShotWithLocalChangeIsNotADuplicate(t *testing.T) {
	src := t.TempDir()
	a := writeJPEG(t, src, "DSC_2462.jpg", scene(identity, false), 90)
	b := writeJPEG(t, src, "DSC_2463.jpg", scene(identity, true), 90)
	if d := pHashDistance(t, a, b); d > maxHammingDistance {
		t.Fatalf("precondition: pHash distance = %d, want ≤ %d so the detail check decides", d, maxHammingDistance)
	}

	result := Run(context.Background(), contentJob("skip", "", src))
	if result.DuplicatesPair != 0 {
		t.Errorf("pairs = %d, want 0: the images differ in one area", result.DuplicatesPair)
	}
	if result.Errors != 0 {
		t.Errorf("errors = %d, want 0", result.Errors)
	}
}

// Spec: DUP-003, DUP-014
func TestDedup_ContentMode_ResizedRecompressedCopyIsADuplicate(t *testing.T) {
	src1, src2 := t.TempDir(), t.TempDir()
	orig := scene(identity, false)
	writeJPEG(t, src1, "photo.jpg", orig, 95)
	writeJPEG(t, src2, "photo-small.jpg", resize.Resize(256, 192, orig, resize.Lanczos3), 60)

	result := Run(context.Background(), contentJob("skip", "", src1, src2))
	if result.DuplicatesPair != 1 {
		t.Errorf("pairs = %d, want 1", result.DuplicatesPair)
	}
}

// Spec: DUP-014
func TestDedup_ContentMode_BrightnessEditedCopyIsADuplicate(t *testing.T) {
	src1, src2 := t.TempDir(), t.TempDir()
	writeJPEG(t, src1, "photo.jpg", scene(identity, false), 90)
	brighter := func(v float64) float64 { return math.Pow(math.Max(v, 0), 0.8) }
	writeJPEG(t, src2, "photo-edited.jpg", scene(brighter, false), 90)

	result := Run(context.Background(), contentJob("skip", "", src1, src2))
	if result.DuplicatesPair != 1 {
		t.Errorf("pairs = %d, want 1", result.DuplicatesPair)
	}
}

// Spec: DUP-012, DUP-014
func TestCache_DetailFingerprintsAreCached(t *testing.T) {
	src := t.TempDir()
	writeJPEG(t, src, "a.jpg", scene(identity, false), 90)
	writeJPEG(t, src, "b.jpg", scene(identity, true), 90)
	writeJPEG(t, src, "c.jpg", scene(identity, false), 70)
	path := filepath.Join(t.TempDir(), CacheFileName)
	job := contentJob("skip", "", src)

	first := RunWithCache(context.Background(), job, LoadCache(path))
	second := RunWithCache(context.Background(), job, LoadCache(path))

	if first.DetailsComputed != 3 {
		t.Errorf("first run computed %d detail fingerprints, want 3 (all files are candidates)", first.DetailsComputed)
	}
	if second.DetailsComputed != 0 || second.CacheHits != 3 {
		t.Errorf("second run: computed = %d, hits = %d, want 0 and 3 (no image is read)", second.DetailsComputed, second.CacheHits)
	}
	if first.DuplicatesPair != 1 || second.DuplicatesPair != 1 {
		t.Errorf("pairs = %d and %d, want 1 both times (a and c)", first.DuplicatesPair, second.DuplicatesPair)
	}
}

// Spec: DUP-014
func TestDetailDiff_TileThatDiffersDecides(t *testing.T) {
	a := detailOf(scene(identity, false))
	if d := detailDiff(a, a); d != 0 {
		t.Errorf("same fingerprint: diff = %.3f, want 0", d)
	}
	if d := detailDiff(a, detailOf(scene(identity, true))); d <= maxDetailDiff {
		t.Errorf("local change: diff = %.3f, want > %.2f", d, maxDetailDiff)
	}
}
