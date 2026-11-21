package dedup

import (
	"context"
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/testutil"
)

// bruteForceCandidates is the pairwise comparison the index replaces.
func bruteForceCandidates(files []fileInfo, mode string, i int) []int {
	var out []int
	for j := i + 1; j < len(files); j++ {
		if isDuplicate(files[i], files[j], mode) {
			out = append(out, j)
		}
	}
	return out
}

// randomHashFiles returns files whose hashes form clusters of near copies,
// plus undecodable files without a hash.
func randomHashFiles(r *rand.Rand, n int) []fileInfo {
	var centers []uint64
	for c := 0; c < n/10+1; c++ {
		centers = append(centers, r.Uint64())
	}
	files := make([]fileInfo, n)
	for i := range files {
		if r.Intn(20) == 0 {
			continue // undecodable
		}
		h := centers[r.Intn(len(centers))]
		for flips := r.Intn(9); flips > 0; flips-- { // distance 0..8 from the center
			h ^= 1 << uint(r.Intn(64))
		}
		files[i].PHash = hashFromCache(h)
	}
	return files
}

// Spec: DUP-004, DUP-003
func TestHashIndex_FindsExactlyThePairwiseMatches(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for round := 0; round < 20; round++ {
		files := randomHashFiles(r, 300)
		idx := newMatchIndex(files, "content")
		for i := range files {
			got, want := idx.candidates(i), bruteForceCandidates(files, "content", i)
			if len(got) == 0 && len(want) == 0 {
				continue
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("round %d, file %d: index %v, pairwise %v", round, i, got, want)
			}
		}
	}
}

// Spec: DUP-004, DUP-002
func TestMetadataIndex_FindsExactlyThePairwiseMatches(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	files := make([]fileInfo, 2000)
	for i := range files {
		files[i].Size = int64(1000 + r.Intn(5))
		if r.Intn(10) != 0 { // some files have no capture date
			// Equal instants in different zones must match.
			loc := time.FixedZone("x", 3600*r.Intn(3))
			files[i].ExifDate = base.Add(time.Duration(r.Intn(50)) * time.Second).In(loc)
		}
	}
	idx := newMatchIndex(files, "metadata")
	for i := range files {
		got, want := idx.candidates(i), bruteForceCandidates(files, "metadata", i)
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("file %d: index %v, pairwise %v", i, got, want)
		}
	}
}

// BenchmarkFindDuplicates_Content100k shows that comparing 100,000 hashes
// no longer takes 5 billion comparisons.
func BenchmarkFindDuplicates_Content100k(b *testing.B) {
	files := randomHashFiles(rand.New(rand.NewSource(3)), 100_000)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		idx := newMatchIndex(files, "content")
		for i := range files {
			idx.candidates(i)
		}
	}
}

// Spec: DUP-012, DUP-002
func TestCache_OldNoDateEntriesAreRecomputed(t *testing.T) {
	src := t.TempDir()
	taken := time.Date(2023, 8, 19, 17, 42, 10, 0, time.UTC)
	a := testutil.WriteHEIC(t, src, "a.heic", taken, false)
	testutil.WriteHEIC(t, src, "b.heic", taken, true) // same size, same date

	// A cache written by an older Phorga, which could not read HEIC dates.
	cachePath := filepath.Join(t.TempDir(), CacheFileName)
	old := map[string]map[string]any{}
	for _, p := range []string{a, filepath.Join(src, "b.heic")} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		old[p] = map[string]any{"size": info.Size(), "mtime": info.ModTime(), "noExif": true}
	}
	data, _ := json.Marshal(old)
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	cache := LoadCache(cachePath)
	job := config.DuplicateDetection{Sources: []string{src}, DetectionMode: "metadata", Action: config.DuplicateAction{Method: "skip"}}
	first := RunWithCache(context.Background(), job, cache)
	if first.CacheHits != 0 {
		t.Errorf("old 'no date' entries must not be used, hits = %d", first.CacheHits)
	}

	second := RunWithCache(context.Background(), job, cache)
	if second.CacheHits != 2 {
		t.Errorf("new entries must be used, hits = %d", second.CacheHits)
	}
	sizeA, _ := os.Stat(a)
	sizeB, _ := os.Stat(filepath.Join(src, "b.heic"))
	if sizeA.Size() == sizeB.Size() && first.DuplicatesPair != 1 {
		t.Errorf("pairs = %d, want 1", first.DuplicatesPair)
	}
}
