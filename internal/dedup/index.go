package dedup

import (
	"math/bits"
	"sort"
)

// matchIndex finds, for a file, the later files that match it, without
// comparing it with every other file (DUP-004):
//
//   - metadata mode: files are grouped by (size, capture date); only files
//     in the same group can match.
//   - content mode: multi-index hashing (see hashChunks) finds all hashes
//     within the Hamming distance limit while comparing a file only with
//     the few files that share a part of its hash.
//
// candidates returns exactly the files that a pairwise comparison with
// isDuplicate would find, so the grouping does not change.
type matchIndex interface {
	candidates(i int) []int
}

func newMatchIndex(files []fileInfo, mode string) matchIndex {
	switch mode {
	case "metadata":
		return newMetadataIndex(files)
	case "content":
		return newHashIndex(files)
	}
	return emptyIndex{}
}

type emptyIndex struct{}

func (emptyIndex) candidates(int) []int { return nil }

// metadataKey identifies files that are duplicates in metadata mode.
type metadataKey struct {
	size int64
	date int64 // capture date as Unix nanoseconds, so equal instants match
}

type metadataIndex struct {
	files   []fileInfo
	buckets map[metadataKey][]int // ascending file indices
}

func newMetadataIndex(files []fileInfo) *metadataIndex {
	idx := &metadataIndex{files: files, buckets: map[metadataKey][]int{}}
	for i, f := range files {
		if f.ExifDate.IsZero() {
			continue // files without a capture date never match (DUP-002)
		}
		k := metadataKey{size: f.Size, date: f.ExifDate.UnixNano()}
		idx.buckets[k] = append(idx.buckets[k], i)
	}
	return idx
}

func (idx *metadataIndex) candidates(i int) []int {
	f := idx.files[i]
	if f.ExifDate.IsZero() {
		return nil
	}
	bucket := idx.buckets[metadataKey{size: f.Size, date: f.ExifDate.UnixNano()}]
	// The bucket is sorted; return the files after i.
	pos := sort.SearchInts(bucket, i+1)
	return bucket[pos:]
}

// hashChunks splits a 64-bit hash into maxHammingDistance+1 bit ranges.
// Two hashes that differ in at most maxHammingDistance bits leave at least
// one range unchanged (pigeonhole principle), so a file only needs to be
// compared with the files that share one of its chunks.
var hashChunks = []struct{ shift, bits uint }{
	{0, 11}, {11, 11}, {22, 11}, {33, 11}, {44, 10}, {54, 10},
}

func chunk(h uint64, c int) uint64 {
	return (h >> hashChunks[c].shift) & (1<<hashChunks[c].bits - 1)
}

// hashIndex finds the hashes within maxHammingDistance by multi-index
// hashing: one table per chunk, from chunk value to the files that have it.
type hashIndex struct {
	files  []fileInfo
	tables []map[uint64][]int // per chunk; ascending file indices
	seen   []int              // per file: the last i whose candidates included it
}

func hammingDistance(a, b uint64) int { return bits.OnesCount64(a ^ b) }

func newHashIndex(files []fileInfo) *hashIndex {
	if len(hashChunks) != maxHammingDistance+1 {
		panic("dedup: hashChunks must have maxHammingDistance+1 entries")
	}
	idx := &hashIndex{files: files, tables: make([]map[uint64][]int, len(hashChunks)), seen: make([]int, len(files))}
	for c := range idx.tables {
		idx.tables[c] = map[uint64][]int{}
	}
	for i, f := range files {
		idx.seen[i] = -1
		if f.PHash == nil {
			continue // not decodable: never a candidate (DUP-003)
		}
		h := f.PHash.GetHash()
		for c := range hashChunks {
			k := chunk(h, c)
			idx.tables[c][k] = append(idx.tables[c][k], i)
		}
	}
	return idx
}

func (idx *hashIndex) candidates(i int) []int {
	f := idx.files[i]
	if f.PHash == nil {
		return nil
	}
	h := f.PHash.GetHash()
	var out []int
	for c := range hashChunks {
		list := idx.tables[c][chunk(h, c)]
		for _, j := range list[sort.SearchInts(list, i+1):] {
			if idx.seen[j] == i {
				continue // already found through another chunk
			}
			idx.seen[j] = i
			if hammingDistance(h, idx.files[j].PHash.GetHash()) <= maxHammingDistance {
				out = append(out, j)
			}
		}
	}
	sort.Ints(out)
	return out
}
