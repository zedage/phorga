// Package synology finds Synology Photos users in the NAS home folders and
// builds a ready-to-use organizer profile for each of them (SYN-001,
// SYN-002). It only reads folder names and never writes anything.
package synology

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zedage/phorga/internal/config"
)

// Folder names inside a home folder, as Synology Photos creates them.
const (
	PhotosFolder       = "Photos"       // personal space of Synology Photos
	MobileBackupFolder = "MobileBackup" // default target of the mobile app backup
)

// Defaults of a profile created for a user (SYN-002).
const (
	DefaultStructure = "{year}/{month}"
	DefaultSchedule  = "0 3 * * *"
	DefaultMode      = "move"
	DefaultDetection = "filename+exif"
	DefaultAction    = "skip"
)

// User is one Synology Photos user found below the homes folder.
type User struct {
	Name         string // name of the home folder, which is the DSM user name
	Photos       string // <homes>/<name>/Photos
	MobileBackup string // <homes>/<name>/Photos/MobileBackup
	// HasMobileBackup is true when the MobileBackup folder exists.
	HasMobileBackup bool
	// Readable is false when Phorga may not open the Photos folder, so
	// the container user lacks permissions for this home.
	Readable bool
}

// Discover lists the users below homes that have a Photos folder, sorted by
// name. Hidden entries and DSM system folders (names starting with ".", "@"
// or "#") are left out. A user whose home or Photos folder Phorga may not
// open is listed with Readable false, so the UI can explain why. It returns
// an error only when homes itself cannot be read.
func Discover(homes string) ([]User, error) {
	entries, err := os.ReadDir(homes)
	if err != nil {
		return nil, fmt.Errorf("reading homes folder: %w", err)
	}
	var users []User
	for _, entry := range entries {
		name := entry.Name()
		if skipName(name) || !isDirEntry(homes, entry) {
			continue
		}
		photos := filepath.Join(homes, name, PhotosFolder)
		u := User{Name: name, Photos: photos, MobileBackup: filepath.Join(photos, MobileBackupFolder)}
		info, err := os.Stat(photos)
		switch {
		case errors.Is(err, fs.ErrPermission):
			u.Readable = false
		case err != nil || !info.IsDir():
			continue // no Synology Photos for this user
		default:
			u.Readable = canList(photos)
			if mb, err := os.Stat(u.MobileBackup); err == nil && mb.IsDir() {
				u.HasMobileBackup = true
			}
		}
		users = append(users, u)
	}
	sort.Slice(users, func(i, j int) bool { return strings.ToLower(users[i].Name) < strings.ToLower(users[j].Name) })
	return users, nil
}

// Profile returns the organizer profile for a user (SYN-002): new files in
// MobileBackup are moved into Photos/{year}/{month} every night. It starts as
// a dry run, so nothing changes until the user turns dry run off. Files with
// the same name and capture date as a file already in Photos are left in
// MobileBackup; other name clashes get a random suffix (ORG-011).
func Profile(u User) config.OrganizerProfile {
	return config.OrganizerProfile{
		Name:            u.Name,
		DryRun:          true,
		Target:          u.Photos,
		TargetStructure: DefaultStructure,
		Sources: []config.OrganizerSource{{
			Path:     u.MobileBackup,
			Mode:     DefaultMode,
			Schedule: DefaultSchedule,
			Duplicates: &config.OrganizerDuplicate{
				DetectionMode: DefaultDetection,
				Action:        config.DuplicateAction{Method: DefaultAction},
			},
		}},
	}
}

// Configured reports whether cfg already has an organizer profile for the
// user: one whose target is the user's Photos folder or that reads from a
// folder inside it.
func Configured(profiles []config.OrganizerProfile, u User) bool {
	for _, p := range profiles {
		if samePath(p.Target, u.Photos) {
			return true
		}
		for _, s := range p.Sources {
			if within(u.Photos, s.Path) {
				return true
			}
		}
	}
	return false
}

func skipName(name string) bool {
	return name == "" || strings.ContainsAny(name[:1], ".@#")
}

// isDirEntry follows symlinks, so a linked home folder counts too.
func isDirEntry(dir string, entry os.DirEntry) bool {
	if entry.IsDir() {
		return true
	}
	if entry.Type()&os.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, entry.Name()))
	return err == nil && info.IsDir()
}

func canList(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	return err == nil || errors.Is(err, io.EOF)
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func within(root, p string) bool {
	if p == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(p))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
