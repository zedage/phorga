# ADR-0015: Fingerprint cache for duplicate detection, written in dry runs too

- **Status:** accepted
- **Date:** 2026-11-14
- **Specs:** DUP-012, DUP-007

## Context
Duplicate detection kept nothing between runs. In `content` mode every run decoded every image again to compute its perceptual hash. For a real library of about 7,400 JPEGs that took about 17 minutes per run, although almost no file had changed since the run before.

The obvious fix is to remember each file's fingerprint. Two questions came with it:

1. Where to store it. The organizer already keeps a JSON state file per source in `stateDir` ([ADR-0005](0005-json-state-files.md)).
2. Whether a dry run may write it. AGENTS.md and the specs say a dry run never changes a file, including state files.

## Decision
- **One cache file**, `<stateDir>/dedup-cache.json`, shared by all duplicate detection jobs. Entries are keyed by path and are valid only while size and modification time are unchanged (the same identity the organizer uses). Each mode stores its own result (pHash or EXIF date), including "no result" for files that cannot be decoded or have no EXIF, so they are not opened again. Read errors are not cached.
- **Not the organizer's state file.** The organizer's files record *source* files that may be gone after a move; duplicate detection usually scans other folders. Above all, every job reads its file at the start and writes it back whole; two jobs sharing one file would overwrite each other's changes.
- **No write contention.** The cache is loaded once at startup into one in-process object that all jobs share. It guards its map with a mutex and is the only writer of its file, which it writes atomically (`fileops.WriteFileAtomic`). A run saves it every 500 newly computed fingerprints and at the end of its scan, also when it was interrupted.
- **Dry runs write the cache.** This is a deliberate, documented exception to the dry-run rule. The cache only remembers what was computed; it never changes, moves or deletes a photo, and Phorga can always rebuild it.
- **Pruning** after a complete scan removes entries below the job's own sources whose files are gone. Entries of other folders are kept, because another job may own them.
- A missing or corrupt cache starts empty with a warning; a failed write is logged and never fails the job.

## Consequences
- After the first run, only new or changed images are decoded. A re-run of the library above takes seconds instead of minutes.
- Dry runs become fast after their first run, and the cache is already filled when a job is switched to real runs.
- `dryRun` no longer means "writes no file at all" for duplicate detection; it means "changes no photo". AGENTS.md and DUP-007 say so.
- The cache can grow stale if files change without a new modification time (rare; some tools preserve mtime on edit). Deleting `dedup-cache.json` forces a full re-scan.
- The file grows with the library, about 150–200 bytes per file.

## Alternatives considered
Storing fingerprints in the organizer's state file was rejected for the reasons above. One cache file per job was rejected, because jobs often scan the same folders and would compute the same fingerprints twice. Keeping the cache in memory for dry runs only was rejected, because every dry run would then take the full time again. A database (SQLite) was rejected as in ADR-0005: it needs CGO or a large dependency, and a JSON file of a few MB is fast enough.
