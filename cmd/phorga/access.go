package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// checkStateAccess checks that Phorga can read every file below the state
// directory and write in every folder of it (RUN-003). State, run status
// and cache files are replaced through a temp file in their folder, so the
// folder needs write access, not the file. A probe file is created and
// removed at once in each folder; it is not a state file and does not
// change one, so this also runs before dry runs.
func checkStateAccess(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return probeWrite(path)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("cannot read %s: %w", path, err)
		}
		return f.Close()
	})
}

// checkConfigWritable checks that a save from the web UI can replace the
// config file (RUN-003): its folder must be writable, and an existing file
// too (a read-only bind mount of the file alone fails here).
func checkConfigWritable(path string) error {
	if err := probeWrite(filepath.Dir(path)); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	return f.Close()
}

// probeWrite creates and removes a temp file in dir.
func probeWrite(dir string) error {
	f, err := os.CreateTemp(dir, ".phorga-access-*")
	if err != nil {
		return fmt.Errorf("cannot write in %s: %w", dir, err)
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}

// processUser returns log attributes with the user and group Phorga runs
// as, where the platform has them (not on Windows).
func processUser() []any {
	if uid := os.Getuid(); uid >= 0 {
		return []any{"uid", uid, "gid", os.Getgid()}
	}
	return nil
}
