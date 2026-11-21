package dedup

import (
	"math/rand"
	"reflect"
	"testing"
	"time"
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
