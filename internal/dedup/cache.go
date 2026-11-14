package dedup

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/corona10/goimagehash"
	"github.com/zedage/phorga/internal/fileops"
)

// CacheFileName is the name of the fingerprint cache inside the state
// directory (DUP-012).
const CacheFileName = "dedup-cache.json"

// cacheSaveEvery is the number of new fingerprints after which a run saves
// the cache, so an interrupted run keeps its work (DUP-012).
var cacheSaveEvery = 500

// cacheEntry holds what a scan computed for one file. It is valid only while
// the file's size and modification time are unchanged. Each mode fills its
// own fields, so jobs with different modes can share one entry.
type cacheEntry struct {
	Size  int64     `json:"size"`
	Mtime time.Time `json:"mtime"`

	// content mode: the perceptual hash, or NoPHash when the file cannot
	// be decoded (videos, HEIC, RAW), so it is not tried again.
	PHash   *uint64 `json:"phash,omitempty"`
	NoPHash bool    `json:"noPhash,omitempty"`

	// metadata mode: the EXIF capture date, or NoExif when there is none.
	ExifDate *time.Time `json:"exifDate,omitempty"`
	NoExif   bool       `json:"noExif,omitempty"`
}

func (e cacheEntry) matches(size int64, mtime time.Time) bool {
	return e.Size == size && e.Mtime.Equal(mtime)
}

// Cache remembers fingerprints across runs, keyed by path, size and
// modification time. One Cache is shared by all duplicate detection jobs of
// the process; it is safe for concurrent use and is the only writer of its
// file, so jobs cannot overwrite each other's work.
type Cache struct {
	mu      sync.Mutex
	path    string
	entries map[string]cacheEntry
}

// CachePath returns the cache file path for a state directory.
func CachePath(stateDir string) string {
	return filepath.Join(stateDir, CacheFileName)
}

// LoadCache reads the cache from path. A missing file starts empty; an
// unreadable or corrupt file is logged and starts empty, never failing.
// An empty path keeps the cache in memory only.
func LoadCache(path string) *Cache {
	c := &Cache{path: path, entries: map[string]cacheEntry{}}
	if path == "" {
		return c
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("reading duplicate detection cache failed, starting fresh", "path", path, "error", err)
		}
		return c
	}
	if err := json.Unmarshal(data, &c.entries); err != nil {
		slog.Warn("duplicate detection cache is corrupt, starting fresh", "path", path, "error", err)
		c.entries = map[string]cacheEntry{}
	}
	return c
}

// lookup returns the entry for path if the file is unchanged.
func (c *Cache) lookup(path string, size int64, mtime time.Time) (cacheEntry, bool) {
	if c == nil {
		return cacheEntry{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[path]
	if !ok || !e.matches(size, mtime) {
		return cacheEntry{}, false
	}
	return e, true
}

// update merges what a scan computed into the entry for path. An entry for
// an older version of the file is replaced.
func (c *Cache) update(path string, size int64, mtime time.Time, set func(*cacheEntry)) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[path]
	if !ok || !e.matches(size, mtime) {
		e = cacheEntry{Size: size, Mtime: mtime}
	}
	set(&e)
	c.entries[path] = e
}

// prune removes entries below roots whose files were not seen by a
// complete scan of those roots. Entries of other folders are kept, because
// another job may own them.
func (c *Cache) prune(roots []string, seen map[string]bool) int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	removed := 0
	for path := range c.entries {
		if seen[path] || !underAny(path, roots) {
			continue
		}
		delete(c.entries, path)
		removed++
	}
	return removed
}

func underAny(path string, roots []string) bool {
	for _, root := range roots {
		root = filepath.Clean(root)
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// Save writes the cache atomically. Errors are logged, never returned to
// the job: a missing cache only costs time on the next run.
func (c *Cache) Save() {
	if c == nil || c.path == "" {
		return
	}
	c.mu.Lock()
	data, err := json.Marshal(c.entries)
	c.mu.Unlock()
	if err != nil {
		slog.Error("encoding duplicate detection cache failed", "error", err)
		return
	}
	if err := fileops.WriteFileAtomic(c.path, data, 0644); err != nil {
		slog.Error("writing duplicate detection cache failed", "path", c.path, "error", err)
	}
}

// Len returns the number of cached files.
func (c *Cache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

func hashFromCache(v uint64) *goimagehash.ImageHash {
	return goimagehash.NewImageHash(v, goimagehash.PHash)
}
