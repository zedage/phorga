package dedup

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/corona10/goimagehash"
	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/exclude"
	"github.com/zedage/phorga/internal/fileops"
	"github.com/zedage/phorga/internal/metadata"
	"github.com/zedage/phorga/internal/runstatus"
)

const maxHammingDistance = 5

// progressEvery is the number of scanned files after which the scan logs
// its progress (DUP-011). It is a variable so tests can shorten it.
var progressEvery = 500

// RunResult holds statistics for a duplicate detection run.
type RunResult struct {
	TotalFiles     int
	DuplicatesPair int
	ActionsApplied int
	Errors         int
	DryRun         bool
	// Canceled is true when the run stopped early because ctx was
	// cancelled (DUP-010).
	Canceled bool
	// CacheHits is the number of files whose fingerprint came from the
	// cache instead of being computed (DUP-012).
	CacheHits int
	// DetailsComputed is the number of detail fingerprints computed (not
	// taken from the cache) for pHash candidates in content mode (DUP-014).
	DetailsComputed int
}

// fileInfo stores gathered metadata about a scanned file.
type fileInfo struct {
	Path     string
	Size     int64
	Mtime    time.Time
	ExifDate time.Time
	PHash    *goimagehash.ImageHash
}

// Run executes a duplicate detection job without a fingerprint cache.
func Run(ctx context.Context, job config.DuplicateDetection) RunResult {
	return RunWithCache(ctx, job, nil)
}

// RunWithCache executes a duplicate detection job for all configured
// sources. Fingerprints of unchanged files come from cache and new ones are
// added to it (DUP-012); a nil cache computes everything. When ctx is
// cancelled, it stops before the next file and applies no further actions
// (DUP-010).
func RunWithCache(ctx context.Context, job config.DuplicateDetection, cache *Cache) RunResult {
	result := RunResult{DryRun: job.DryRun}

	slog.Info("duplicate detection run starting",
		"sources", job.Sources,
		"mode", job.DetectionMode,
		"dryRun", job.DryRun,
	)

	// Count first, so progress can show "x of y" (WEB-014).
	runstatus.Report(ctx, runstatus.PhaseCounting, 0, 0)
	ex := exclude.New(job.Exclude)
	total := 0
	for _, src := range job.Sources {
		total += metadata.CountSupported(ctx, src, ex)
	}

	// Scan all sources
	sc := &scanner{ctx: ctx, mode: job.DetectionMode, cache: cache, exclude: ex, total: total, result: &result}
	for _, src := range job.Sources {
		sc.scan(src)
	}
	files := sc.files
	result.TotalFiles = len(files)
	// Keep what was computed, also when the run was interrupted. Dry runs
	// write the cache too; it never touches a photo (ADR-0015).
	if ctx.Err() == nil {
		seen := make(map[string]bool, len(files))
		for _, f := range files {
			seen[f.Path] = true
		}
		if removed := cache.prune(job.Sources, seen); removed > 0 {
			slog.Info("removed vanished files from the duplicate detection cache", "files", removed)
		}
	}
	cache.Save()
	slog.Info("duplicate detection scan complete",
		"files", len(files), "fromCache", result.CacheHits, "computed", sc.computed)
	if ctx.Err() != nil {
		return canceled(result)
	}

	// Find duplicates. In content mode, pHash candidates are confirmed by
	// the detail check (DUP-014); its fingerprints go into the cache too.
	v := newVerifier(ctx, cache, &result)
	// Each group keeps its best file (DUP-004); resolutions are cached too.
	duplicates := findDuplicates(ctx, files, job.DetectionMode, v, newKeeper(cache))
	result.DetailsComputed = v.computed
	if len(duplicates) > 0 || v.computed > 0 {
		cache.Save()
	}
	if ctx.Err() != nil {
		return canceled(result)
	}
	result.DuplicatesPair = len(duplicates)

	// Apply actions
	for i, pair := range duplicates {
		if ctx.Err() != nil {
			return canceled(result)
		}
		runstatus.Report(ctx, runstatus.PhaseActing, i, len(duplicates))
		if err := applyAction(job, pair, &result); err != nil {
			slog.Error("failed to apply duplicate action", "file", pair.duplicate.Path, "error", err)
			result.Errors++
		}
	}

	slog.Info("duplicate detection run complete",
		"totalFiles", result.TotalFiles,
		"duplicates", result.DuplicatesPair,
		"detailsComputed", result.DetailsComputed,
		"actions", result.ActionsApplied,
		"errors", result.Errors,
	)

	return result
}

// canceled marks a run as stopped by shutdown and logs it (DUP-010).
func canceled(result RunResult) RunResult {
	result.Canceled = true
	slog.Info("duplicate detection run stopped early because of shutdown",
		"scannedFiles", result.TotalFiles,
		"actions", result.ActionsApplied,
		"errors", result.Errors,
	)
	return result
}

// duplicatePair represents a detected duplicate with reference to the canonical file.
type duplicatePair struct {
	canonical fileInfo
	duplicate fileInfo
}

// scanner collects the supported files of all sources of one run.
type scanner struct {
	ctx      context.Context
	mode     string
	cache    *Cache
	exclude  *exclude.Matcher // skipped entries (DUP-013)
	total    int
	result   *RunResult
	files    []fileInfo
	computed int // fingerprints computed (not from cache) in this run
}

// errNotDecodable marks files that are readable but not an image Phorga
// can decode, such as videos. Only those are remembered as "no hash".
var errNotDecodable = errors.New("not a decodable image")

// scan appends the supported files below srcPath. It stops when ctx is
// cancelled, logs its progress every progressEvery files and saves the cache
// every cacheSaveEvery computed fingerprints.
func (sc *scanner) scan(srcPath string) {
	ctx, result := sc.ctx, sc.result
	filepath.WalkDir(srcPath, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return filepath.SkipAll
		}
		// System, hidden and excluded entries (DUP-013).
		if d != nil && sc.exclude.Skip(srcPath, path, d.IsDir()) {
			if d.IsDir() {
				slog.Info("skipping folder", "path", path)
				return filepath.SkipDir
			}
			return nil
		}
		if err != nil {
			slog.Error("error accessing path", "path", path, "error", err)
			result.Errors++
			return nil
		}

		if d.IsDir() {
			return nil
		}

		if !metadata.IsSupported(path) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			slog.Error("unable to stat file", "path", path, "error", err)
			result.Errors++
			return nil
		}

		fi := fileInfo{
			Path:  path,
			Size:  info.Size(),
			Mtime: info.ModTime(),
		}
		if sc.fromCache(&fi, info.ModTime()) {
			result.CacheHits++
		} else if sc.compute(&fi, info.ModTime()) {
			sc.computed++
			if sc.computed%cacheSaveEvery == 0 {
				sc.cache.Save()
			}
		}

		sc.files = append(sc.files, fi)
		runstatus.Report(ctx, runstatus.PhaseScanning, len(sc.files), sc.total)
		if len(sc.files)%progressEvery == 0 {
			slog.Info("duplicate detection scan progress", "scannedFiles", len(sc.files), "fromCache", result.CacheHits, "source", srcPath)
		}
		return nil
	})
}

// fromCache fills fi from the cache when the file is unchanged and the
// cache has a result for this run's mode.
func (sc *scanner) fromCache(fi *fileInfo, mtime time.Time) bool {
	e, ok := sc.cache.lookup(fi.Path, fi.Size, mtime)
	if !ok {
		return false
	}
	switch sc.mode {
	case "content":
		if e.PHash != nil {
			fi.PHash = hashFromCache(*e.PHash)
			return true
		}
		return e.NoPHash
	case "metadata":
		if e.ExifDate != nil {
			fi.ExifDate = *e.ExifDate
			return true
		}
		return e.NoExif
	}
	return false
}

// compute reads the file to fill fi and stores the result in the cache. It
// reports whether there was a result worth caching; read errors are not
// cached, because they may go away.
func (sc *scanner) compute(fi *fileInfo, mtime time.Time) bool {
	switch sc.mode {
	case "metadata":
		// Only real EXIF dates count; files without EXIF never match (DUP-002).
		date, err := metadata.ExtractEXIFDate(fi.Path)
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			slog.Debug("unable to read file", "path", fi.Path, "error", err)
			return false
		}
		if err == nil {
			fi.ExifDate = date
		}
		sc.cache.update(fi.Path, fi.Size, mtime, func(e *cacheEntry) {
			if err == nil {
				e.ExifDate, e.NoExif = &date, false
			} else {
				e.ExifDate, e.NoExif = nil, true
			}
		})
		return true
	case "content":
		hash, err := computePHash(fi.Path)
		if err != nil && !errors.Is(err, errNotDecodable) {
			slog.Debug("unable to read file", "path", fi.Path, "error", err)
			return false
		}
		if err != nil {
			slog.Debug("unable to compute phash, skipping", "path", fi.Path, "error", err)
		} else {
			fi.PHash = hash
		}
		sc.cache.update(fi.Path, fi.Size, mtime, func(e *cacheEntry) {
			if err == nil {
				v := hash.GetHash()
				e.PHash, e.NoPHash = &v, false
			} else {
				e.PHash, e.NoPHash = nil, true
			}
		})
		return true
	}
	return false
}

func computePHash(path string) (*goimagehash.ImageHash, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%w: decoding image: %v", errNotDecodable, err)
	}

	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return nil, fmt.Errorf("%w: computing phash: %v", errNotDecodable, err)
	}

	return hash, nil
}

// findDuplicates groups matching files. An index finds the candidates of
// each file, so the run does not compare every pair (DUP-004). In content
// mode, v confirms each pHash match with the detail check (DUP-014); v may
// be nil in other modes. k chooses the file each group keeps.
func findDuplicates(ctx context.Context, files []fileInfo, detectionMode string, v *verifier, k *keeper) []duplicatePair {
	var pairs []duplicatePair
	idx := newMatchIndex(files, detectionMode)

	// The first file not yet in a group starts one; every later file not
	// yet in a group that matches it joins. Each group keeps its best file
	// and yields N-1 pairs against it (DUP-004).
	inGroup := make([]bool, len(files))
	for i := 0; i < len(files); i++ {
		if ctx.Err() != nil {
			return nil
		}
		runstatus.Report(ctx, runstatus.PhaseComparing, i, len(files))
		if inGroup[i] {
			continue
		}
		group := []int{i}
		for _, j := range idx.candidates(i) {
			if inGroup[j] || !isDuplicate(files[i], files[j], detectionMode) {
				continue
			}
			if detectionMode == "content" {
				// Decoding takes time, so check for shutdown before each one.
				if ctx.Err() != nil {
					return nil
				}
				if !v.same(files[i], files[j]) {
					continue
				}
			}
			group = append(group, j)
			inGroup[j] = true
		}
		if len(group) == 1 {
			continue
		}
		keep := k.best(files, group)
		for _, g := range group {
			if g != keep {
				pairs = append(pairs, duplicatePair{canonical: files[keep], duplicate: files[g]})
			}
		}
	}

	return pairs
}

func isDuplicate(a, b fileInfo, detectionMode string) bool {
	switch detectionMode {
	case "metadata":
		// Match by fileSize + exifDateTimeOriginal
		return a.Size == b.Size &&
			!a.ExifDate.IsZero() && !b.ExifDate.IsZero() &&
			a.ExifDate.Equal(b.ExifDate)

	case "content":
		if a.PHash == nil || b.PHash == nil {
			return false
		}
		distance, err := a.PHash.Distance(b.PHash)
		if err != nil {
			return false
		}
		return distance <= maxHammingDistance

	default:
		return false
	}
}

func applyAction(job config.DuplicateDetection, pair duplicatePair, result *RunResult) error {
	method := job.Action.Method

	if job.DryRun {
		slog.Info("[DRY-RUN] duplicate detected",
			"canonical", pair.canonical.Path,
			"duplicate", pair.duplicate.Path,
			"action", method,
		)
		result.ActionsApplied++
		return nil
	}

	switch method {
	case "moveToFolder":
		// Never overwrite a file that is already in the duplicate folder.
		dst := fileops.UniquePath(filepath.Join(job.Action.DuplicateFolder, filepath.Base(pair.duplicate.Path)))
		if err := fileops.AtomicMove(pair.duplicate.Path, dst); err != nil {
			return fmt.Errorf("moving duplicate to folder: %w", err)
		}
		slog.Info("duplicate moved",
			"from", pair.duplicate.Path,
			"to", dst,
			"canonical", pair.canonical.Path,
		)
		result.ActionsApplied++

	case "skip":
		slog.Info("duplicate detected (skip)",
			"canonical", pair.canonical.Path,
			"duplicate", pair.duplicate.Path,
		)
		result.ActionsApplied++

	default:
		return fmt.Errorf("unknown duplicate action method: %s", method)
	}

	return nil
}
