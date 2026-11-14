package metadata

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Spec: WEB-014
func TestCountSupported(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.jpg", "b.JPEG", "sub/c.mp4", "sub/deeper/d.heic", "notes.txt", "sub/e.xmp"} {
		path := filepath.Join(root, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if n := CountSupported(context.Background(), root); n != 4 {
		t.Errorf("CountSupported = %d, want 4", n)
	}
	if n := CountSupported(context.Background(), filepath.Join(root, "missing")); n != 0 {
		t.Errorf("missing folder: CountSupported = %d, want 0", n)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if n := CountSupported(ctx, root); n != 0 {
		t.Errorf("cancelled: CountSupported = %d, want 0", n)
	}
}
