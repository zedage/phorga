package dedup

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// Spec: DUP-010
func TestDedup_CancelledRunMovesNothing(t *testing.T) {
	src1, src2 := t.TempDir(), t.TempDir()
	dupDir := t.TempDir()
	createTestImage(t, src1, "a.jpg", 100, 100, 100)
	createTestImage(t, src2, "b.jpg", 100, 100, 100)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := Run(ctx, contentJob("moveToFolder", dupDir, src1, src2))

	if !result.Canceled || result.ActionsApplied != 0 {
		t.Fatalf("result = %+v, want canceled without actions", result)
	}
	if n := countFiles(t, dupDir); n != 0 {
		t.Errorf("duplicate folder has %d files, want 0", n)
	}
	if countFiles(t, src1) != 1 || countFiles(t, src2) != 1 {
		t.Error("source files must stay in place")
	}
}

// Spec: DUP-011
func TestDedup_LogsScanProgress(t *testing.T) {
	src := t.TempDir()
	for i := 0; i < 5; i++ {
		createTestImage(t, src, fmt.Sprintf("img%d.jpg", i), uint8(40*i), 10, 10)
	}
	var buf bytes.Buffer
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	origEvery := progressEvery
	progressEvery = 2
	t.Cleanup(func() { slog.SetDefault(orig); progressEvery = origEvery })

	Run(context.Background(), contentJob("skip", "", src))

	if n := strings.Count(buf.String(), "duplicate detection scan progress"); n != 2 {
		t.Errorf("progress lines = %d, want 2 (after 2 and 4 files):\n%s", n, buf.String())
	}
	if !strings.Contains(buf.String(), `"scannedFiles":4`) {
		t.Errorf("progress must report the number of scanned files:\n%s", buf.String())
	}
}
