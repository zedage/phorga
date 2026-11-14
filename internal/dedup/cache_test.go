package dedup

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/runstatus"
)

// threeImages creates two identical images and one different one.
func threeImages(t *testing.T) (src1, src2 string) {
	t.Helper()
	src1, src2 = t.TempDir(), t.TempDir()
	createTestImage(t, src1, "a.jpg", 100, 100, 100)
	createTestImage(t, src2, "b.jpg", 100, 100, 100)
	createCheckerImage(t, src2, "c.jpg")
	return src1, src2
}

// Spec: DUP-012
func TestCache_SecondRunUsesCache(t *testing.T) {
	src1, src2 := threeImages(t)
	cache := LoadCache(filepath.Join(t.TempDir(), CacheFileName))
	job := contentJob("skip", "", src1, src2)

	first := RunWithCache(context.Background(), job, cache)
	second := RunWithCache(context.Background(), job, cache)

	if first.CacheHits != 0 || cache.Len() != 3 {
		t.Fatalf("first run: hits = %d, cached = %d, want 0 and 3", first.CacheHits, cache.Len())
	}
	if second.CacheHits != 3 {
		t.Errorf("second run: hits = %d, want 3", second.CacheHits)
	}
	if first.DuplicatesPair != 1 || second.DuplicatesPair != 1 {
		t.Errorf("pairs = %d and %d, want 1 both times", first.DuplicatesPair, second.DuplicatesPair)
	}
}

// Spec: DUP-012
func TestCache_MetadataModeUsesCache(t *testing.T) {
	src := t.TempDir()
	createTestFile(t, src, "a.jpg", "no exif")
	createTestFile(t, src, "b.jpg", "no exif either")
	cache := LoadCache("")
	job := config.DuplicateDetection{Sources: []string{src}, DetectionMode: "metadata", Action: config.DuplicateAction{Method: "skip"}}

	RunWithCache(context.Background(), job, cache)
	second := RunWithCache(context.Background(), job, cache)
	if second.CacheHits != 2 {
		t.Errorf("hits = %d, want 2 (files without EXIF are remembered too)", second.CacheHits)
	}
}

// Spec: DUP-012
func TestCache_ChangedFileIsRecomputed(t *testing.T) {
	src1, src2 := threeImages(t)
	cache := LoadCache("")
	job := contentJob("skip", "", src1, src2)
	RunWithCache(context.Background(), job, cache)

	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(src1, "a.jpg"), later, later); err != nil {
		t.Fatal(err)
	}
	second := RunWithCache(context.Background(), job, cache)
	if second.CacheHits != 2 {
		t.Errorf("hits = %d, want 2 (the changed file is computed again)", second.CacheHits)
	}
}

// Spec: DUP-012
func TestCache_PersistsAndReloads(t *testing.T) {
	src1, src2 := threeImages(t)
	path := filepath.Join(t.TempDir(), "state", CacheFileName)
	job := contentJob("skip", "", src1, src2)
	RunWithCache(context.Background(), job, LoadCache(path))

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cache file not written: %v", err)
	}
	reloaded := LoadCache(path)
	if reloaded.Len() != 3 {
		t.Fatalf("reloaded cache has %d entries, want 3", reloaded.Len())
	}
	if r := RunWithCache(context.Background(), job, reloaded); r.CacheHits != 3 || r.DuplicatesPair != 1 {
		t.Errorf("after reload: hits = %d, pairs = %d, want 3 and 1", r.CacheHits, r.DuplicatesPair)
	}
}

// Spec: DUP-007, DUP-012
func TestCache_DryRunWritesCacheButNoPhoto(t *testing.T) {
	src1, src2 := threeImages(t)
	dupDir := t.TempDir()
	path := filepath.Join(t.TempDir(), CacheFileName)
	job := contentJob("moveToFolder", dupDir, src1, src2)
	job.DryRun = true

	RunWithCache(context.Background(), job, LoadCache(path))

	if _, err := os.Stat(path); err != nil {
		t.Errorf("a dry run must write the cache (ADR-0015): %v", err)
	}
	if countFiles(t, dupDir) != 0 || countFiles(t, src1) != 1 || countFiles(t, src2) != 2 {
		t.Error("a dry run must not move any photo")
	}
}

// Spec: DUP-012
func TestCache_RemembersUndecodableFiles(t *testing.T) {
	src := t.TempDir()
	createTestFile(t, src, "clip.mp4", "not an image")
	cache := LoadCache("")
	job := contentJob("skip", "", src)

	RunWithCache(context.Background(), job, cache)
	if second := RunWithCache(context.Background(), job, cache); second.CacheHits != 1 {
		t.Errorf("hits = %d, want 1 (a video is not opened again)", second.CacheHits)
	}
}

// Spec: DUP-012
func TestCache_PruneKeepsOtherJobsEntries(t *testing.T) {
	srcA, srcB := t.TempDir(), t.TempDir()
	createTestImage(t, srcA, "a.jpg", 10, 10, 10)
	createTestImage(t, srcB, "b.jpg", 20, 20, 20)
	cache := LoadCache("")
	RunWithCache(context.Background(), contentJob("skip", "", srcA), cache)
	RunWithCache(context.Background(), contentJob("skip", "", srcB), cache)
	if cache.Len() != 2 {
		t.Fatalf("cached = %d, want 2", cache.Len())
	}

	// The file of job A disappears; only A's entry is removed.
	os.Remove(filepath.Join(srcA, "a.jpg"))
	RunWithCache(context.Background(), contentJob("skip", "", srcA), cache)
	if _, ok := cache.entries[filepath.Join(srcA, "a.jpg")]; ok {
		t.Error("entry of the removed file must be pruned")
	}
	if _, ok := cache.entries[filepath.Join(srcB, "b.jpg")]; !ok {
		t.Error("entry of another job's folder must be kept")
	}
}

// Spec: DUP-012
func TestCache_CorruptFileStartsFresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), CacheFileName)
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := LoadCache(path)
	if cache.Len() != 0 {
		t.Fatalf("corrupt cache must start empty, has %d entries", cache.Len())
	}
	src1, src2 := threeImages(t)
	if r := RunWithCache(context.Background(), contentJob("skip", "", src1, src2), cache); r.DuplicatesPair != 1 {
		t.Errorf("pairs = %d, want 1", r.DuplicatesPair)
	}
	if LoadCache(path).Len() != 3 {
		t.Error("the corrupt file must be replaced by a valid cache")
	}
}

// Spec: DUP-010, DUP-012
func TestCache_InterruptedRunKeepsComputedFingerprints(t *testing.T) {
	src1, src2 := threeImages(t)
	path := filepath.Join(t.TempDir(), CacheFileName)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Stop after two files were scanned.
	ctx = runstatus.WithReporter(ctx, func(phase string, done, total int) {
		if phase == runstatus.PhaseScanning && done == 2 {
			cancel()
		}
	})

	r := RunWithCache(ctx, contentJob("skip", "", src1, src2), LoadCache(path))
	if !r.Canceled {
		t.Fatal("run should have stopped early")
	}
	if n := LoadCache(path).Len(); n != 2 {
		t.Errorf("saved cache has %d entries, want the 2 computed before the stop", n)
	}
}

// Spec: DUP-012
func TestCache_SharedByConcurrentJobs(t *testing.T) {
	src1, src2 := threeImages(t)
	cache := LoadCache(filepath.Join(t.TempDir(), CacheFileName))
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			RunWithCache(context.Background(), contentJob("skip", "", src1, src2), cache)
		}()
	}
	wg.Wait()
	if cache.Len() != 3 {
		t.Errorf("cached = %d, want 3", cache.Len())
	}
}
