# ADR-0019: No-clobber transfers and same-volume moves with hard links

- **Status:** accepted
- **Date:** 2026-11-21
- **Specs:** ORG-004, ORG-020, ORG-024, ORG-025, DUP-005

## Context
ADR-0006 writes every transfer to a temp file and renames it into place. A rename replaces whatever already has the name. Phorga checked the destination first (`UniquePath`, `os.Stat`), so the promise "never overwrite" depended on nothing appearing between the check and the rename. Phorga's own jobs could break it: different jobs ran in parallel (fixed separately in ADR-0020), and two sources of one profile write into the same folders. A second problem: `move` always copied the data and deleted the source, even when source and target were on the same volume. On a NAS that means rewriting every video.

While changing this, a third case showed up: when the target lies inside the source, a file can already be at its own target path. `move` with `replace` then copied the file onto itself and deleted it as the source.

## Decision
- The finished temp file gets its final name with a **hard link** (`link(tmp, dst)`), and then the temp name is removed. Creating a link fails if the name is taken, so a file that appeared after the check is never replaced. The transfer fails with `destination already exists`, and the next run handles the file again.
- A **move** first tries `link(src, dst)` and removes `src`. On the same volume this moves the file without copying it. If that fails (another volume, separate Docker bind mounts, no hard links), it falls back to the verified copy.
- `replace` is the only operation that may overwrite. It uses `rename`, both for the temp file and for a same-volume move.
- No transfer runs when source and destination are the same file. The organizer skips such a file as "already in place".
- On filesystems without hard links (exFAT, FAT, some SMB mounts), Phorga falls back to "check, then rename". That keeps the old behavior and its short window.

## Consequences
- The rule "never overwrite" now holds against other writers too, wherever hard links exist (ext4 and Btrfs on Synology, APFS, NTFS).
- Moves on one volume are instant and keep the inode, so permissions, extended attributes and the mtime are unchanged.
- To get same-volume moves in Docker, mount the common parent folder once (e.g. `/volume1/photos`) instead of `incoming` and `organized` separately. Linux refuses hard links across mount points, even on the same disk.
- A failed link costs one extra system call before the fallback.
- A run that crashes between link and unlink leaves the file under both names. In copy mode that is the intended result. In move mode the next run finds the target taken by the same content and handles it as a duplicate.

## Alternatives considered
- **`renameat2(RENAME_NOREPLACE)`:** atomic and no-clobber, but Linux-only, needs `golang.org/x/sys`, and is not supported by every filesystem either.
- **`O_EXCL` placeholder at the destination, then rename over it:** replaces the placeholder atomically, but another writer could replace the placeholder in between, and a crash leaves empty files with real photo names.
- **A lock file per folder:** only protects against other Phorga processes, not against other programs.
