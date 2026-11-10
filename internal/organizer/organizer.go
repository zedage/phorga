package organizer

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/fileops"
	"github.com/zedage/phorga/internal/metadata"
	"github.com/zedage/phorga/internal/state"
)

// RunResult holds statistics for an organizer run.
type RunResult struct {
	Processed int
	Skipped   int
	Errors    int
	DryRun    bool
}

// Run executes the organizer for a single source against a profile.
func Run(profile config.OrganizerProfile, source config.OrganizerSource, st *state.State, statePath string) RunResult {
	result := RunResult{DryRun: profile.DryRun}

	slog.Info("organizer run starting",
		"source", source.Path,
		"target", profile.Target,
		"mode", source.Mode,
		"dryRun", profile.DryRun,
	)

	err := filepath.WalkDir(source.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Error("error accessing path", "path", path, "error", err)
			result.Errors++
			return nil // continue with next file
		}

		if d.IsDir() {
			return nil
		}

		if !metadata.IsSupported(path) {
			return nil
		}

		// Check state for already-processed files
		info, statErr := d.Info()
		if statErr != nil {
			slog.Error("unable to stat file", "path", path, "error", statErr)
			result.Errors++
			return nil
		}

		if st.IsProcessed(path, info.Size(), info.ModTime()) {
			result.Skipped++
			return nil
		}

		if err := processFile(profile, source, path, info, st, &result); err != nil {
			slog.Error("failed to process file", "path", path, "error", err)
			result.Errors++
		}

		return nil
	})

	if err != nil {
		slog.Error("walk error", "source", source.Path, "error", err)
	}

	// Save state
	if !profile.DryRun {
		if err := state.Save(st, statePath); err != nil {
			slog.Error("failed to save state", "error", err)
		}
	}

	slog.Info("organizer run complete",
		"source", source.Path,
		"processed", result.Processed,
		"skipped", result.Skipped,
		"errors", result.Errors,
	)

	return result
}

func processFile(profile config.OrganizerProfile, source config.OrganizerSource, path string, info fs.FileInfo, st *state.State, result *RunResult) error {
	// Extract date
	date, method, err := metadata.ExtractDate(path)
	if err != nil {
		// No date available
		return handleNoDate(profile, source, path, result)
	}

	slog.Debug("date resolved", "path", path, "date", date, "method", method)

	// Build target path
	subDir, err := metadata.ResolveTargetPath(profile.TargetStructure, date)
	if err != nil {
		slog.Error("failed to resolve target path", "source", path, "error", err)
		result.Errors++
		return err
	}
	targetDir := filepath.Join(profile.Target, subDir)
	targetPath := filepath.Join(targetDir, filepath.Base(path))

	// Check for duplicates at target
	if _, statErr := os.Stat(targetPath); statErr == nil {
		// File already exists at target — handle duplicate
		return handleDuplicate(profile, source, path, targetPath, info, st, result, method)
	}

	// Execute copy/move
	return executeTransfer(profile, source, path, targetPath, info, st, result, method)
}

func handleNoDate(profile config.OrganizerProfile, source config.OrganizerSource, path string, result *RunResult) error {
	if profile.FallbackFolder != "" {
		targetPath := filepath.Join(profile.FallbackFolder, filepath.Base(path))

		if profile.DryRun {
			slog.Info("[DRY-RUN] would transfer to fallback", "source", path, "target", targetPath, "mode", source.Mode)
			result.Processed++
			return nil
		}

		if err := transferFile(source.Mode, path, targetPath); err != nil {
			return fmt.Errorf("transferring to fallback: %w", err)
		}
		result.Processed++
		return nil
	}

	slog.Warn("skipping file (no date, no fallbackFolder)", "path", path)
	result.Skipped++
	return nil
}

func handleDuplicate(profile config.OrganizerProfile, source config.OrganizerSource, srcPath, targetPath string, info fs.FileInfo, st *state.State, result *RunResult, dateMethod string) error {
	if source.Duplicates == nil {
		// No duplicate config → skip
		slog.Info("skipping duplicate (no duplicate config)", "source", srcPath, "target", targetPath)
		result.Skipped++
		return nil
	}

	if !isDuplicate(source.Duplicates.DetectionMode, srcPath, targetPath) {
		// Not actually a duplicate by detection criteria — add suffix
		suffix := fileops.GenerateSuffix()
		newName := fileops.AddSuffix(filepath.Base(srcPath), suffix)
		targetPath = filepath.Join(filepath.Dir(targetPath), newName)
		return executeTransfer(profile, source, srcPath, targetPath, info, st, result, dateMethod)
	}

	method := source.Duplicates.Action.Method

	if profile.DryRun {
		slog.Info("[DRY-RUN] duplicate detected", "source", srcPath, "target", targetPath, "action", method)
		result.Processed++
		return nil
	}

	switch method {
	case "replace":
		return executeTransfer(profile, source, srcPath, targetPath, info, st, result, dateMethod)
	case "addSuffix":
		suffix := fileops.GenerateSuffix()
		newName := fileops.AddSuffix(filepath.Base(srcPath), suffix)
		newTarget := filepath.Join(filepath.Dir(targetPath), newName)
		return executeTransfer(profile, source, srcPath, newTarget, info, st, result, dateMethod)
	case "moveToFolder":
		dupTarget := filepath.Join(source.Duplicates.Action.DuplicateFolder, filepath.Base(srcPath))
		return executeTransfer(profile, source, srcPath, dupTarget, info, st, result, dateMethod)
	case "skip":
		slog.Info("skipping duplicate", "source", srcPath, "target", targetPath)
		result.Skipped++
		return nil
	default:
		return fmt.Errorf("unknown duplicate action method: %s", method)
	}
}

func isDuplicate(detectionMode, srcPath, targetPath string) bool {
	// Parse detection mode (+ = AND, | = OR)
	isOr := strings.Contains(detectionMode, "|")
	sep := "+"
	if isOr {
		sep = "|"
	}
	fields := strings.Split(detectionMode, sep)

	results := make([]bool, len(fields))
	for i, field := range fields {
		field = strings.TrimSpace(field)
		switch field {
		case "filename":
			results[i] = filepath.Base(srcPath) == filepath.Base(targetPath)
		case "filedate":
			srcInfo, err1 := os.Stat(srcPath)
			tgtInfo, err2 := os.Stat(targetPath)
			results[i] = err1 == nil && err2 == nil && srcInfo.ModTime().Equal(tgtInfo.ModTime())
		case "exif":
			// Only real EXIF dates count; files without EXIF never match (ORG-011).
			srcDate, err1 := metadata.ExtractEXIFDate(srcPath)
			tgtDate, err2 := metadata.ExtractEXIFDate(targetPath)
			results[i] = err1 == nil && err2 == nil && srcDate.Equal(tgtDate)
		default:
			results[i] = false
		}
	}

	if isOr {
		for _, r := range results {
			if r {
				return true
			}
		}
		return false
	}
	// AND
	for _, r := range results {
		if !r {
			return false
		}
	}
	return true
}

func executeTransfer(profile config.OrganizerProfile, source config.OrganizerSource, srcPath, targetPath string, info fs.FileInfo, st *state.State, result *RunResult, dateMethod string) error {
	if profile.DryRun {
		slog.Info("[DRY-RUN] would transfer", "source", srcPath, "target", targetPath, "mode", source.Mode, "dateMethod", dateMethod)
		result.Processed++
		return nil
	}

	if err := transferFile(source.Mode, srcPath, targetPath); err != nil {
		return err
	}

	st.MarkProcessed(srcPath, info.Size(), info.ModTime())
	result.Processed++
	slog.Info("file transferred", "source", srcPath, "target", targetPath, "mode", source.Mode, "dateMethod", dateMethod)
	return nil
}

func transferFile(mode, src, dst string) error {
	switch mode {
	case "move":
		return fileops.AtomicMove(src, dst)
	case "copy":
		return fileops.AtomicCopy(src, dst)
	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}
}

// StatePath returns the state file path for a given source.
func StatePath(stateDir, profileTarget, sourcePath string) string {
	// Create a unique but readable state file name
	clean := func(s string) string {
		s = strings.ReplaceAll(s, "/", "_")
		s = strings.ReplaceAll(s, "\\", "_")
		s = strings.ReplaceAll(s, ":", "")
		s = strings.TrimLeft(s, "_")
		return s
	}
	name := fmt.Sprintf("org-%s-%s.state.json", clean(profileTarget), clean(sourcePath))
	return filepath.Join(stateDir, name)
}

// RunWithTime is a helper that wraps Run with timing.
func RunWithTime(profile config.OrganizerProfile, source config.OrganizerSource, st *state.State, statePath string) (RunResult, time.Duration) {
	start := time.Now()
	result := Run(profile, source, st, statePath)
	return result, time.Since(start)
}
