package runstatus

import (
	"path/filepath"
	"testing"
	"time"
)

// Spec: CFG-008
func TestForgetDropsRecordsAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	r := Load(path)
	r.Finish("organizer-0-source-0", Record{Start: time.Now(), Counters: map[string]int{"processed": 1}})
	r.Finish("dedup-0", Record{Start: time.Now(), Counters: map[string]int{}})
	r.Finish("dedup-1", Record{Start: time.Now(), DryRun: true})

	r.Forget("organizer-0-source-0")
	r.Forget("dedup-1") // dry run: in memory only
	r.Forget("unknown")

	if _, ok := r.Last("organizer-0-source-0"); ok {
		t.Error("forgotten record still in memory")
	}
	if _, ok := r.Last("dedup-1"); ok {
		t.Error("forgotten dry-run record still in memory")
	}
	reloaded := Load(path)
	if _, ok := reloaded.Last("organizer-0-source-0"); ok {
		t.Error("forgotten record still on disk")
	}
	if _, ok := reloaded.Last("dedup-0"); !ok {
		t.Error("other records must stay on disk")
	}
}
