package dedup

import (
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
	"github.com/zedage/phorga/internal/fileops"
	"github.com/zedage/phorga/internal/metadata"
)

const maxHammingDistance = 5

// RunResult holds statistics for a duplicate detection run.
type RunResult struct {
	TotalFiles     int
	DuplicatesPair int
	ActionsApplied int
	Errors         int
	DryRun         bool
}

// fileInfo stores gathered metadata about a scanned file.
type fileInfo struct {
	Path     string
	Size     int64
	ExifDate time.Time
	PHash    *goimagehash.ImageHash
}

// Run executes a duplicate detection job for all configured sources.
func Run(job config.DuplicateDetection) RunResult {
	result := RunResult{DryRun: job.DryRun}

	slog.Info("duplicate detection run starting",
		"sources", job.Sources,
		"mode", job.DetectionMode,
		"dryRun", job.DryRun,
	)

	// Scan all sources
	var files []fileInfo
	for _, src := range job.Sources {
		scanned := scanSource(src, job.DetectionMode, &result)
		files = append(files, scanned...)
	}
	result.TotalFiles = len(files)

	// Find duplicates
	duplicates := findDuplicates(files, job.DetectionMode)
	result.DuplicatesPair = len(duplicates)

	// Apply actions
	for _, pair := range duplicates {
		if err := applyAction(job, pair, &result); err != nil {
			slog.Error("failed to apply duplicate action", "file", pair.duplicate.Path, "error", err)
			result.Errors++
		}
	}

	slog.Info("duplicate detection run complete",
		"totalFiles", result.TotalFiles,
		"duplicates", result.DuplicatesPair,
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

func scanSource(srcPath string, detectionMode string, result *RunResult) []fileInfo {
	var files []fileInfo

	filepath.WalkDir(srcPath, func(path string, d fs.DirEntry, err error) error {
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
			Path: path,
			Size: info.Size(),
		}

		// Extract metadata when needed
		// Only real EXIF dates count; files without EXIF never match (DUP-002).
		if detectionMode == "metadata" {
			date, err := metadata.ExtractEXIFDate(path)
			if err == nil {
				fi.ExifDate = date
			}
		}

		// Compute perceptual hash for content mode
		if detectionMode == "content" {
			hash, err := computePHash(path)
			if err != nil {
				slog.Debug("unable to compute phash, skipping", "path", path, "error", err)
			} else {
				fi.PHash = hash
			}
		}

		files = append(files, fi)
		return nil
	})

	return files
}

func computePHash(path string) (*goimagehash.ImageHash, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return nil, fmt.Errorf("computing phash: %w", err)
	}

	return hash, nil
}

func findDuplicates(files []fileInfo, detectionMode string) []duplicatePair {
	var pairs []duplicatePair

	// Each file is reported as a duplicate at most once, against the first
	// earlier file it matches that is not itself a duplicate. For a group of
	// N matching files this yields N-1 pairs sharing one canonical (DUP-004).
	isDup := make([]bool, len(files))
	for i := 0; i < len(files); i++ {
		if isDup[i] {
			continue
		}
		for j := i + 1; j < len(files); j++ {
			if isDup[j] {
				continue
			}
			if isDuplicate(files[i], files[j], detectionMode) {
				pairs = append(pairs, duplicatePair{
					canonical: files[i],
					duplicate: files[j],
				})
				isDup[j] = true
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
		dst := filepath.Join(job.Action.DuplicateFolder, filepath.Base(pair.duplicate.Path))
		// Avoid overwriting in duplicate folder
		if _, err := os.Stat(dst); err == nil {
			suffix := fileops.GenerateSuffix()
			dst = filepath.Join(job.Action.DuplicateFolder, fileops.AddSuffix(filepath.Base(pair.duplicate.Path), suffix))
		}
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
