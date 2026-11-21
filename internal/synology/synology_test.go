package synology

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/zedage/phorga/internal/config"
)

func mkdir(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(parts...)
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatal(err)
	}
	return p
}

// Spec: SYN-001
func TestDiscoverListsUsersWithPhotos(t *testing.T) {
	homes := t.TempDir()
	mkdir(t, homes, "bob", "Photos", "MobileBackup", "iPhone")
	mkdir(t, homes, "Alice", "Photos")
	mkdir(t, homes, "carol", "Documents") // no Synology Photos
	mkdir(t, homes, "@eaDir", "Photos")
	mkdir(t, homes, "#recycle", "Photos")
	mkdir(t, homes, ".hidden", "Photos")
	if err := os.WriteFile(filepath.Join(homes, "notes.txt"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	users, err := Discover(homes)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 || users[0].Name != "Alice" || users[1].Name != "bob" {
		t.Fatalf("expected Alice and bob, got %+v", users)
	}
	alice, bob := users[0], users[1]
	if alice.HasMobileBackup || !alice.Readable {
		t.Errorf("alice: %+v", alice)
	}
	if !bob.HasMobileBackup || !bob.Readable {
		t.Errorf("bob: %+v", bob)
	}
	if bob.Photos != filepath.Join(homes, "bob", "Photos") || bob.MobileBackup != filepath.Join(homes, "bob", "Photos", "MobileBackup") {
		t.Errorf("bob paths: %+v", bob)
	}
}

// Spec: SYN-001
func TestDiscoverMissingHomesFails(t *testing.T) {
	if _, err := Discover(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected an error for a missing homes folder")
	}
}

// Spec: SYN-001
func TestDiscoverMarksUnreadableHome(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permissions cannot be withheld here")
	}
	homes := t.TempDir()
	locked := mkdir(t, homes, "dave")
	mkdir(t, locked, "Photos")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0755) })

	users, err := Discover(homes)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Name != "dave" || users[0].Readable {
		t.Fatalf("expected dave as not readable, got %+v", users)
	}
}

// Spec: SYN-001
func TestDiscoverNeverWrites(t *testing.T) {
	homes := t.TempDir()
	mkdir(t, homes, "bob", "Photos")
	before, _ := os.ReadDir(filepath.Join(homes, "bob", "Photos"))
	if _, err := Discover(homes); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadDir(filepath.Join(homes, "bob", "Photos"))
	if len(before) != len(after) {
		t.Fatal("Discover must not create anything")
	}
}

// Spec: SYN-002, CFG-011
func TestProfileMovesMobileBackupIntoPhotosAsDryRun(t *testing.T) {
	u := User{Name: "bob", Photos: "/homes/bob/Photos", MobileBackup: "/homes/bob/Photos/MobileBackup"}
	p := Profile(u)
	if p.Name != "bob" || !p.DryRun || p.Target != u.Photos || p.TargetStructure != "{year}/{month}" {
		t.Fatalf("unexpected profile %+v", p)
	}
	if len(p.Sources) != 1 {
		t.Fatalf("expected one source, got %+v", p.Sources)
	}
	s := p.Sources[0]
	if s.Path != u.MobileBackup || s.Mode != "move" || s.Schedule != "0 3 * * *" {
		t.Fatalf("unexpected source %+v", s)
	}
	if s.Duplicates == nil || s.Duplicates.DetectionMode != "filename+exif" || s.Duplicates.Action.Method != "skip" {
		t.Fatalf("unexpected duplicates %+v", s.Duplicates)
	}
	cfg := &config.Config{Organizer: []config.OrganizerProfile{p}}
	if problems := config.ValidateProblems(cfg); len(problems) > 0 {
		t.Fatalf("profile must be valid, got %v", problems)
	}
}

// Spec: SYN-002
func TestConfigured(t *testing.T) {
	u := User{Name: "bob", Photos: "/homes/bob/Photos", MobileBackup: "/homes/bob/Photos/MobileBackup"}
	cases := []struct {
		name    string
		profile config.OrganizerProfile
		want    bool
	}{
		{"same target", config.OrganizerProfile{Target: "/homes/bob/Photos/"}, true},
		{"source inside Photos", config.OrganizerProfile{Target: "/other", Sources: []config.OrganizerSource{{Path: "/homes/bob/Photos/Upload"}}}, true},
		{"other user", config.OrganizerProfile{Target: "/homes/alice/Photos", Sources: []config.OrganizerSource{{Path: "/homes/alice/Photos/MobileBackup"}}}, false},
		{"name prefix only", config.OrganizerProfile{Target: "/homes/bob/PhotosOld"}, false},
	}
	for _, tc := range cases {
		if got := Configured([]config.OrganizerProfile{tc.profile}, u); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
