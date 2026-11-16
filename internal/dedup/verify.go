package dedup

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"os"

	"github.com/nfnt/resize"
)

// The detail check is the second stage of content mode (DUP-014). pHash only
// sees the coarse layout of an image, so burst shots of the same scene match
// although a face or a person differs. The detail fingerprint keeps a small
// grayscale thumbnail; two candidates are compared tile by tile, and one tile
// that differs is enough to tell them apart.
//
// The comparison uses the direction of the brightness step between
// neighbouring pixels, not the brightness itself. Re-encoding, resizing and
// tone changes (brightness, gamma) keep these directions; a changed face or a
// moved person does not. Values measured on real photos (ADR-0017): copies
// ≤ 0.07, burst shots ≥ 0.22.
const (
	detailSize    = 64   // thumbnail edge in pixels
	detailTiles   = 16   // tiles per edge, so a tile is 4×4 pixels
	detailMinStep = 10   // brightness step (0–255) below which a pixel pair is flat
	maxDetailDiff = 0.12 // a tile with a larger share of flipped steps differs
)

// computeDetail decodes the image at path and returns its detail
// fingerprint: a detailSize×detailSize grayscale thumbnail.
func computeDetail(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%w: decoding image: %v", errNotDecodable, err)
	}
	return detailOf(img), nil
}

// detailOf scales img to the thumbnail and converts it to luminance.
func detailOf(img image.Image) []byte {
	small := resize.Resize(detailSize, detailSize, img, resize.Bilinear)
	b := small.Bounds()
	out := make([]byte, detailSize*detailSize)
	for y := 0; y < detailSize; y++ {
		for x := 0; x < detailSize; x++ {
			r, g, bl, _ := small.At(b.Min.X+x, b.Min.Y+y).RGBA()
			lum := (299*r + 587*g + 114*bl) / 1000 // 0–65535
			out[y*detailSize+x] = byte(lum >> 8)
		}
	}
	return out
}

// detailDiff returns the worst tile's share of neighbour steps (to the right
// and down) whose direction differs between a and b. Pairs that are flat in
// both images are ignored, so JPEG noise in plain areas does not count.
func detailDiff(a, b []byte) float64 {
	const ts = detailSize / detailTiles
	worst := 0.0
	for ty := 0; ty < detailTiles; ty++ {
		for tx := 0; tx < detailTiles; tx++ {
			flipped := 0
			for y := ty * ts; y < (ty+1)*ts; y++ {
				for x := tx * ts; x < (tx+1)*ts; x++ {
					if x+1 < detailSize && stepFlipped(a, b, y*detailSize+x, y*detailSize+x+1) {
						flipped++
					}
					if y+1 < detailSize && stepFlipped(a, b, y*detailSize+x, (y+1)*detailSize+x) {
						flipped++
					}
				}
			}
			if d := float64(flipped) / float64(2*ts*ts); d > worst {
				worst = d
			}
		}
	}
	return worst
}

// stepFlipped reports whether the brightness step from pixel i to pixel j
// goes in different directions in a and b.
func stepFlipped(a, b []byte, i, j int) bool {
	da := int(a[j]) - int(a[i])
	db := int(b[j]) - int(b[i])
	if abs(da) < detailMinStep && abs(db) < detailMinStep {
		return false
	}
	return (da > 0) != (db > 0)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// verifier runs the detail check for candidate pairs. It keeps the
// fingerprints of this run in memory, takes them from the cache when the
// file is unchanged, and computes and caches them otherwise (DUP-012).
type verifier struct {
	ctx      context.Context
	cache    *Cache
	result   *RunResult
	details  map[string][]byte
	failed   map[string]bool
	computed int
}

func newVerifier(ctx context.Context, cache *Cache, result *RunResult) *verifier {
	return &verifier{ctx: ctx, cache: cache, result: result, details: map[string][]byte{}, failed: map[string]bool{}}
}

// same reports whether two pHash candidates also match in detail. A file
// whose fingerprint cannot be computed never matches.
func (v *verifier) same(a, b fileInfo) bool {
	da, ok := v.detail(a)
	if !ok {
		return false
	}
	db, ok := v.detail(b)
	if !ok {
		return false
	}
	diff := detailDiff(da, db)
	if diff > maxDetailDiff {
		slog.Debug("similar images differ in detail, not duplicates",
			"a", a.Path, "b", b.Path, "detailDiff", diff)
		return false
	}
	return true
}

func (v *verifier) detail(fi fileInfo) ([]byte, bool) {
	if d, ok := v.details[fi.Path]; ok {
		return d, true
	}
	if v.failed[fi.Path] {
		return nil, false
	}
	if e, ok := v.cache.lookup(fi.Path, fi.Size, fi.Mtime); ok && len(e.Detail) == detailSize*detailSize {
		v.details[fi.Path] = e.Detail
		return e.Detail, true
	}
	d, err := computeDetail(fi.Path)
	if err != nil {
		slog.Error("unable to check image details", "path", fi.Path, "error", err)
		v.result.Errors++
		v.failed[fi.Path] = true
		return nil, false
	}
	v.computed++
	v.details[fi.Path] = d
	v.cache.update(fi.Path, fi.Size, fi.Mtime, func(e *cacheEntry) { e.Detail = d })
	return d, true
}
