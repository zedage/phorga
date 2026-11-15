package metadata

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/zedage/phorga/internal/exclude"
)

func writeFiles(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, name := range names {
		path := filepath.Join(root, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Spec: WEB-014
func TestCountSupported(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, "a.jpg", "b.JPEG", "sub/c.mp4", "sub/deeper/d.heic", "notes.txt", "sub/e.xmp")

	if n := CountSupported(context.Background(), root, nil); n != 4 {
		t.Errorf("CountSupported = %d, want 4", n)
	}
	if n := CountSupported(context.Background(), filepath.Join(root, "missing"), nil); n != 0 {
		t.Errorf("missing folder: CountSupported = %d, want 0", n)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if n := CountSupported(ctx, root, nil); n != 0 {
		t.Errorf("cancelled: CountSupported = %d, want 0", n)
	}
}

// Spec: ORG-022, ORG-023
func TestCountSupported_SkipsExcludedEntries(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root,
		"IMG_1234.JPG",
		"@eaDir/IMG_1234.JPG/SYNO_PHOTO_THUMB_SM.jpg",
		"._IMG_1234.JPG",
		"Screenshots/a.png",
		"2024/b.jpg",
	)
	if n := CountSupported(context.Background(), root, exclude.New([]string{"screenshots"})); n != 2 {
		t.Errorf("CountSupported = %d, want 2 (IMG_1234.JPG and 2024/b.jpg)", n)
	}
}
