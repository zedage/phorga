# ADR-0016: Fixed skip list for system folders, simple exclude patterns

- **Status:** accepted
- **Date:** 2026-11-15
- **Specs:** ORG-022, ORG-023, DUP-013, CFG-007

## Context
A dry run on a Synology showed many `@eaDir/IMG_….JPG/SYNO_PHOTO_THUMB_SM.jpg` files. DSM creates these folders for thumbnails and hides them in File Station, but a container sees them. The organizer would have sorted the thumbnails like photos. Worse, content duplicate detection matches a thumbnail with its original, and because `@` sorts before letters, the thumbnail is found first, becomes the kept file, and the real photo is moved as the duplicate (DUP-004).

Users also want to leave out their own folders, such as screenshots or exports.

## Decision
- **A fixed list, always on.** Both jobs skip `@eaDir`, `#recycle` and `#snapshot` folders (exact names, as DSM writes them) and every file or folder whose name starts with `.`. There is no option to turn this off. Only entries below the source are affected, so a source path that itself contains such a name still works.
- **An optional `exclude` list per organizer source and per duplicate detection job**, not a global one, so the jobs stay independent (ADR-0001).
- **Simple pattern syntax.** A pattern without `/` matches a name at any depth; a pattern with `/` matches the path relative to the source; a trailing `/` matches folders only. Wildcards are those of Go's `path.Match`. Matching ignores case. There is no `**` and no `!` re-include.
- **One log line per skipped folder** at info level, so the user can see what was left out. Skipped files are not logged one by one.
- One package, `internal/exclude`, is used by both jobs and by the file count for the progress display, so they cannot disagree.

## Consequences
- Thumbnails, recycle bins, snapshots and macOS `._` files no longer reach the organizer or duplicate detection. Cache entries for them disappear after the next complete duplicate detection scan.
- A user who keeps real photos in a dot-folder below a source cannot include them; they must point a source at that folder directly.
- On a library with many photo folders, each run logs one `skipping folder` line per `@eaDir` folder. These lines are at debug level (ORG-022), so the normal info log stays readable.
- The syntax cannot express "this folder at any depth below 2019" (`2019/**/Scans`) or exceptions. Those can be added later without breaking existing patterns.

## Alternatives considered
Full gitignore syntax (`**`, `!`, anchoring rules) was rejected for now: it needs a library or noticeably more code, and its rules are easy to get wrong. A global `exclude` list was rejected to keep jobs independent. A switch to disable the fixed list was rejected because skipping thumbnails is never wrong for a photo library, and an off switch would bring back the risk of moving originals. Logging at debug level was rejected because Phorga has no log level option, so the lines would never be seen.
