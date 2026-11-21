package organizer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/exclude"
	"github.com/zedage/phorga/internal/fileops"
	"github.com/zedage/phorga/internal/metadata"
	"github.com/zedage/phorga/internal/runstatus"
	"github.com/zedage/phorga/internal/state"
)

// stateSaveInterval is the number of processed files after which the state
// is saved during a run (ORG-015).
var stateSaveInterval = 200

// saveState is a variable so tests can observe when the state is written.
var saveState = state.Save

// RunResult holds statistics for an organizer run.
type RunResult struct {
	Processed int
	Skipped   int
	Errors    int
	DryRun    bool
	// Canceled is true when the run stopped early because ctx was
	// cancelled (ORG-021).
	Canceled bool
}

// job holds what one organizer run works with.
type job struct {
	profile config.OrganizerProfile
	source  config.OrganizerSource
	// detect is the parsed duplicates.detectionMode (ORG-011); only set
	// when the source has a duplicates block.
	detect config.DetectionExpr
	st     *state.State
	result *RunResult
}

// newJob prepares a run. The configuration was validated at startup, so a
// detectionMode that does not parse only happens in tests; it then matches
// nothing.
func newJob(profile config.OrganizerProfile, source config.OrganizerSource, st *state.State, result *RunResult) *job {
	j := &job{profile: profile, source: source, st: st, result: result}
	if source.Duplicates != nil {
		if expr, err := config.ParseDetectionMode(source.Duplicates.DetectionMode); err == nil {
			j.detect = expr
		} else {
			slog.Error("invalid duplicates.detectionMode, no file counts as a duplicate", "source", source.Path, "error", err)
		}
	}
	return j
}

// Run executes the organizer for a single source against a profile. When ctx
// is cancelled, it stops before the next file, saves the state and returns
// with Canceled set (ORG-021).
func Run(ctx context.Context, profile config.OrganizerProfile, source config.OrganizerSource, st *state.State, statePath string) RunResult {
	result := RunResult{DryRun: profile.DryRun}
	j := newJob(profile, source, st, &result)

	slog.Info("organizer run starting",
		"source", source.Path,
		"target", profile.Target,
		"mode", source.Mode,
		"dryRun", profile.DryRun,
	)

	// Count first, so progress can show "x of y" (WEB-014).
	runstatus.Report(ctx, runstatus.PhaseCounting, 0, 0)
	ex := exclude.New(source.Exclude)
	total := metadata.CountSupported(ctx, source.Path, ex)
	visited := 0

	savedAt := 0
	err := filepath.WalkDir(source.Path, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			result.Canceled = true
			return filepath.SkipAll
		}
		// System, hidden and excluded entries (ORG-022, ORG-023).
		if d != nil && ex.Skip(source.Path, path, d.IsDir()) {
			if d.IsDir() {
				slog.Info("skipping folder", "path", path)
				return filepath.SkipDir
			}
			return nil
		}
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
		visited++
		defer func() { runstatus.Report(ctx, runstatus.PhaseProcessing, visited, total) }()

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

		if err := j.processFile(path, info); err != nil {
			slog.Error("failed to process file", "path", path, "error", err)
			result.Errors++
		}

		// Save progress during the run, so a crash does not lose it (ORG-015).
		if !profile.DryRun && result.Processed-savedAt >= stateSaveInterval {
			if err := saveState(st, statePath); err != nil {
				slog.Error("failed to save state", "error", err)
			}
			savedAt = result.Processed
		}

		return nil
	})

	if err != nil {
		slog.Error("walk error", "source", source.Path, "error", err)
	}

	// Save state
	if !profile.DryRun {
		if err := saveState(st, statePath); err != nil {
			slog.Error("failed to save state", "error", err)
		}
	}

	msg := "organizer run complete"
	if result.Canceled {
		msg = "organizer run stopped early because of shutdown"
	}
	slog.Info(msg,
		"source", source.Path,
		"processed", result.Processed,
		"skipped", result.Skipped,
		"errors", result.Errors,
	)

	return result
}

func (j *job) processFile(path string, info fs.FileInfo) error {
	// Resolve the date; the mtime step is left out with ignoreModTime
	// (ORG-002).
	date, method, err := metadata.ResolveDate(path, !j.profile.IgnoreModTime)
	if err != nil {
		if errors.Is(err, metadata.ErrNoDate) {
			return j.handleNoDate(path, info)
		}
		return err
	}

	slog.Debug("date resolved", "path", path, "date", date, "method", method)

	// Build target path
	subDir, err := metadata.ResolveTargetPath(j.profile.TargetStructure, date)
	if err != nil {
		return fmt.Errorf("resolving target path: %w", err)
	}
	targetDir := filepath.Join(j.profile.Target, subDir)
	targetPath := filepath.Join(targetDir, filepath.Base(path))

	if targetInfo, statErr := os.Stat(targetPath); statErr == nil {
		// The file is already where it belongs, e.g. because the target is
		// inside the source (ORG-024).
		if os.SameFile(info, targetInfo) {
			slog.Debug("file is already in place", "path", path)
			if !j.profile.DryRun {
				j.st.MarkProcessed(path, info.Size(), info.ModTime())
			}
			j.result.Skipped++
			return nil
		}
		// Another file has this name at the target: handle a duplicate.
		return j.handleDuplicate(path, targetPath, info, method)
	}

	// Execute copy/move
	return j.executeTransfer(path, targetPath, info, method, false)
}

func (j *job) handleNoDate(path string, info fs.FileInfo) error {
	if j.profile.FallbackFolder != "" {
		// Never overwrite a file that is already in the fallback folder (ORG-020).
		targetPath := fileops.UniquePath(filepath.Join(j.profile.FallbackFolder, filepath.Base(path)))

		if j.profile.DryRun {
			slog.Info("[DRY-RUN] would transfer to fallback", "source", path, "target", targetPath, "mode", j.source.Mode)
			j.result.Processed++
			return nil
		}

		if err := transferFile(j.source.Mode, path, targetPath, false); err != nil {
			return fmt.Errorf("transferring to fallback: %w", err)
		}
		// Remember the file, so copy mode does not copy it again on every run.
		j.st.MarkProcessed(path, info.Size(), info.ModTime())
		j.result.Processed++
		slog.Info("file transferred to fallback folder", "source", path, "target", targetPath, "mode", j.source.Mode)
		return nil
	}

	slog.Warn("skipping file (no date, no fallbackFolder)", "path", path)
	j.result.Skipped++
	return nil
}

func (j *job) handleDuplicate(srcPath, targetPath string, info fs.FileInfo, dateMethod string) error {
	dup := j.source.Duplicates
	if dup == nil {
		// No duplicate config → skip
		slog.Info("skipping duplicate (no duplicate config)", "source", srcPath, "target", targetPath)
		j.result.Skipped++
		return nil
	}

	if !isDuplicate(j.detect, srcPath, targetPath) {
		// Not actually a duplicate by detection criteria — add suffix
		return j.executeTransfer(srcPath, fileops.SuffixedPath(targetPath), info, dateMethod, false)
	}

	method := dup.Action.Method

	if j.profile.DryRun {
		slog.Info("[DRY-RUN] duplicate detected", "source", srcPath, "target", targetPath, "action", method)
		j.result.Processed++
		return nil
	}

	switch method {
	case "replace":
		return j.executeTransfer(srcPath, targetPath, info, dateMethod, true)
	case "addSuffix":
		return j.executeTransfer(srcPath, fileops.SuffixedPath(targetPath), info, dateMethod, false)
	case "moveToFolder":
		// Never overwrite a file that is already in the duplicate folder (ORG-020).
		dupTarget := fileops.UniquePath(filepath.Join(dup.Action.DuplicateFolder, filepath.Base(srcPath)))
		return j.executeTransfer(srcPath, dupTarget, info, dateMethod, false)
	case "skip":
		slog.Info("skipping duplicate", "source", srcPath, "target", targetPath)
		j.result.Skipped++
		return nil
	default:
		return fmt.Errorf("unknown duplicate action method: %s", method)
	}
}

// isDuplicate evaluates a parsed detectionMode for a file and the file with
// the same name at the target (ORG-011).
func isDuplicate(expr config.DetectionExpr, srcPath, targetPath string) bool {
	if len(expr.Fields) == 0 {
		return false
	}
	for _, field := range expr.Fields {
		match := fieldMatches(field, srcPath, targetPath)
		if expr.Any && match {
			return true
		}
		if !expr.Any && !match {
			return false
		}
	}
	return !expr.Any
}

func fieldMatches(field, srcPath, targetPath string) bool {
	switch field {
	case "filename":
		return filepath.Base(srcPath) == filepath.Base(targetPath)
	case "filedate":
		srcInfo, err1 := os.Stat(srcPath)
		tgtInfo, err2 := os.Stat(targetPath)
		return err1 == nil && err2 == nil && srcInfo.ModTime().Equal(tgtInfo.ModTime())
	case "exif":
		// Only embedded capture dates count; files without one never
		// match (ORG-011).
		srcDate, err1 := metadata.ExtractEmbeddedDate(srcPath)
		tgtDate, err2 := metadata.ExtractEmbeddedDate(targetPath)
		return err1 == nil && err2 == nil && srcDate.Equal(tgtDate)
	default:
		return false
	}
}

func (j *job) executeTransfer(srcPath, targetPath string, info fs.FileInfo, dateMethod string, replace bool) error {
	if j.profile.DryRun {
		slog.Info("[DRY-RUN] would transfer", "source", srcPath, "target", targetPath, "mode", j.source.Mode, "dateMethod", dateMethod)
		j.result.Processed++
		return nil
	}

	if err := transferFile(j.source.Mode, srcPath, targetPath, replace); err != nil {
		return err
	}

	j.st.MarkProcessed(srcPath, info.Size(), info.ModTime())
	j.result.Processed++
	slog.Info("file transferred", "source", srcPath, "target", targetPath, "mode", j.source.Mode, "dateMethod", dateMethod)
	return nil
}

// transferFile copies or moves src to dst. Only replace may overwrite an
// existing dst (ORG-012, ORG-020).
func transferFile(mode, src, dst string, replace bool) error {
	switch {
	case mode == "move" && replace:
		return fileops.AtomicMoveReplace(src, dst)
	case mode == "move":
		return fileops.AtomicMove(src, dst)
	case mode == "copy" && replace:
		return fileops.AtomicCopyReplace(src, dst)
	case mode == "copy":
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
func RunWithTime(ctx context.Context, profile config.OrganizerProfile, source config.OrganizerSource, st *state.State, statePath string) (RunResult, time.Duration) {
	start := time.Now()
	result := Run(ctx, profile, source, st, statePath)
	return result, time.Since(start)
}
