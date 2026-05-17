## Context

Phorganizer is a greenfield Go project. There is no existing codebase. The tool runs as a headless daemon inside a Docker container, organizing photos/videos by date metadata and detecting duplicates — two independent features sharing a single binary, config file, and notification system.

Target environments range from developer laptops to Synology NAS devices with limited resources. The tool must be lightweight, stateless-friendly (state persisted to a JSON file), and resilient to interruptions.

## Goals / Non-Goals

**Goals:**
- Single Go binary with daemon behavior (cron-based wake-up, not filesystem watchers).
- Clean separation between Organizer and Duplicate Detection at package level.
- Atomic file operations (temp + rename) to prevent data loss on interruption.
- Portable Docker image (multi-arch: amd64, arm64 for Synology).
- Simple JSON-file-based state persistence for resume-after-restart.

**Non-Goals:**
- No GUI, no web UI, no REST API.
- No real-time / inotify-based file watching (cron-only in V1).
- No database (SQLite, Postgres, etc.) — flat JSON state files only.
- No cloud storage integration (S3, Google Photos, etc.).
- No perceptual hash tuning parameters in V1 (fixed defaults).

## Decisions

### D1: Single binary, two feature loops
**Decision:** One Go binary runs both Organizer and Duplicate Detection as independent goroutine loops, each with its own cron scheduler.
**Rationale:** Simplifies deployment (one container, one config file). Features don't depend on each other at runtime. A shared `main()` starts both loops based on config presence.
**Alternative considered:** Separate binaries — rejected because it doubles Docker image complexity for no functional benefit.

### D2: Cron-based scheduling via `robfig/cron`
**Decision:** Use `github.com/robfig/cron/v3` for in-process cron scheduling.
**Rationale:** Mature, well-tested library. Avoids external cron dependencies. Supports standard 5-field cron expressions.
**Alternative considered:** OS-level cron + one-shot runs — rejected because it complicates Docker setup and state management.

### D3: EXIF extraction via `rwcarlsen/goexif`
**Decision:** Use `github.com/rwcarlsen/goexif` for EXIF metadata extraction. Fall back to file modification time, then to `fallbackFolder`.
**Rationale:** Lightweight, pure-Go, no CGO dependencies. Critical for Synology ARM builds.
**Alternative considered:** `dsoprea/go-exif` — more feature-rich but heavier. Can swap later if needed.

### D4: Content-based duplicate detection via perceptual hash
**Decision:** Use `github.com/corona10/goimagehash` for perceptual hashing (pHash) with a fixed Hamming distance threshold (≤8 in V1).
**Rationale:** Detects visually identical images even with different compression/resolution. Fixed threshold keeps V1 simple.
**Alternative considered:** SHA256 of raw bytes — only catches exact duplicates, misses re-encoded copies.

### D5: State persistence via JSON file
**Decision:** Each organizer profile and duplicate detection job persists state to a `.state.json` file in the change/config directory.
**Rationale:** No external dependencies. Atomic writes (write temp → rename) prevent corruption. Easy to inspect/debug.
**Alternative considered:** SQLite — more robust but adds CGO dependency, problematic for ARM cross-compilation.

### D6: Atomic file operations
**Decision:** All file writes (copy/move to target, state saves) use temp-file + rename pattern.
**Rationale:** Prevents partial files on crash/power loss. Move mode deletes source only after successful rename at destination.

### D7: Package structure
```
cmd/phorganizer/        # main entry point
internal/
  config/               # YAML/JSON parsing, validation
  organizer/            # Organizer feature loop
  dedup/                # Duplicate Detection feature loop
  metadata/             # EXIF extraction, date resolution
  fileops/              # Atomic copy/move, temp files
  notify/               # ntfy.sh client
  state/                # JSON state read/write
  scheduler/            # Cron wrapper
```
**Rationale:** `internal/` prevents external imports. Feature packages (`organizer`, `dedup`) are self-contained with shared utilities underneath.

## Risks / Trade-offs

- **[Large photo libraries]** → Full recursive scan on each run could be slow. Mitigation: state-based resume skips already-processed files. V2 could add incremental scanning.
- **[pHash memory usage]** → Loading many images for hashing could spike memory on NAS. Mitigation: process files sequentially, don't batch-load. Stream one image at a time.
- **[No file locking]** → Concurrent runs of the same source could cause conflicts. Mitigation: simple lock-file mechanism per source to prevent overlap.
- **[EXIF library limitations]** → `goexif` doesn't support all RAW formats. Mitigation: fall back to file mtime gracefully. Log unsupported formats for user awareness.
- **[ntfy.sh availability]** → Network failures must not block organizer runs. Mitigation: fire-and-forget notification with timeout, log errors but continue.

