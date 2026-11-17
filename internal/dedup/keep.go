package dedup

import (
	"image"
	"log/slog"
	"os"
)

// keeper chooses which file of a duplicate group is kept as the original
// (DUP-004): the most pixels, then the larger file, then the file found
// first. Resolutions are read from the image header only, kept in memory
// for this run and cached (DUP-012), so unchanged files are not opened again.
type keeper struct {
	cache  *Cache
	pixels map[string]int64
}

func newKeeper(cache *Cache) *keeper {
	return &keeper{cache: cache, pixels: map[string]int64{}}
}

// best returns the index into files of the file to keep from group. group
// is in the order the files were found.
func (k *keeper) best(files []fileInfo, group []int) int {
	keep := group[0]
	for _, g := range group[1:] {
		if k.better(files[g], files[keep]) {
			keep = g
		}
	}
	return keep
}

// better reports whether a should be kept instead of b. Ties return false,
// so the file found first wins.
func (k *keeper) better(a, b fileInfo) bool {
	pa, pb := k.pixelsOf(a), k.pixelsOf(b)
	if pa != pb {
		return pa > pb
	}
	return a.Size > b.Size
}

// pixelsOf returns width × height of the image, or 0 when it cannot be
// read (e.g. videos, HEIC). Read failures are not cached, because they may
// go away.
func (k *keeper) pixelsOf(fi fileInfo) int64 {
	if p, ok := k.pixels[fi.Path]; ok {
		return p
	}
	if e, ok := k.cache.lookup(fi.Path, fi.Size, fi.Mtime); ok && e.Width > 0 && e.Height > 0 {
		p := int64(e.Width) * int64(e.Height)
		k.pixels[fi.Path] = p
		return p
	}
	w, h, err := readResolution(fi.Path)
	if err != nil {
		slog.Debug("unable to read image resolution, counting it as 0 pixels", "path", fi.Path, "error", err)
		k.pixels[fi.Path] = 0
		return 0
	}
	p := int64(w) * int64(h)
	k.pixels[fi.Path] = p
	k.cache.update(fi.Path, fi.Size, fi.Mtime, func(e *cacheEntry) { e.Width, e.Height = w, h })
	return p
}

func readResolution(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}
