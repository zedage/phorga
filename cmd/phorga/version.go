package main

import "runtime/debug"

// version is the Phorga version, set at build time with
// -ldflags "-X main.version=2026.11.0" (RUN-004). Releases use calendar
// versions YYYY.MM.MICRO (ADR-0024).
var version = "dev"

// currentVersion returns version, or for a build without it (go build,
// go run) "dev" plus the commit from the Go build info, if there is one.
func currentVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	var rev string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	switch {
	case rev == "":
		return version
	case dirty:
		return version + "+" + rev + ".dirty"
	default:
		return version + "+" + rev
	}
}
