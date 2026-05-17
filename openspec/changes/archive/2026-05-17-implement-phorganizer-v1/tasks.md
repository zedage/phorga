## 1. Project Setup

- [x] 1.1 Initialize Go module (`go mod init github.com/zedage/phorganizer`)
- [x] 1.2 Create package structure: `cmd/phorganizer/`, `internal/config/`, `internal/organizer/`, `internal/dedup/`, `internal/metadata/`, `internal/fileops/`, `internal/notify/`, `internal/state/`, `internal/scheduler/`
- [x] 1.3 Add core dependencies: `robfig/cron/v3`, `rwcarlsen/goexif`, `corona10/goimagehash`, `gopkg.in/yaml.v3`
- [x] 1.4 Create `Dockerfile` (multi-stage build, multi-arch: amd64, arm64)
- [x] 1.5 Create example `config.yaml` with both organizer and duplicateDetection sections

## 2. Configuration Management

- [x] 2.1 Define Go structs for the full config model (organizer profiles, duplicate detection, ntfy)
- [x] 2.2 Implement YAML and JSON config file parsing
- [x] 2.3 Implement config file path resolution (CLI `--config` → env `PHORGANIZER_CONFIG` → `./config.yaml`)
- [x] 2.4 Implement fail-fast validation: required fields, valid modes, valid cron expressions, valid targetStructure tokens, moveToFolder requires duplicateFolder
- [x] 2.5 Write unit tests for config parsing and validation (valid, invalid, edge cases)

## 3. Metadata Extraction

- [x] 3.1 Implement EXIF date extraction (`DateTimeOriginal`) using goexif
- [x] 3.2 Implement date fallback chain: EXIF → file modification time → skip/fallbackFolder
- [x] 3.3 Implement `targetStructure` token resolution (`{year}`, `{month}`, `{monthWithName}`, `{day}`)
- [x] 3.4 Implement supported file type detection (jpg, jpeg, png, heic, raw, cr2, nef, arw, mp4, mov)
- [x] 3.5 Write unit tests for date extraction and target path resolution

## 4. File Operations

- [x] 4.1 Implement atomic copy (temp file → rename at destination)
- [x] 4.2 Implement atomic move (atomic copy → delete source on success)
- [x] 4.3 Implement automatic target directory creation
- [x] 4.4 Implement duplicate suffix generation (random 4-char alphanumeric)
- [x] 4.5 Write unit tests for copy, move, and suffix generation

## 5. State Persistence

- [x] 5.1 Define state model (processed files: path + size + mtime)
- [x] 5.2 Implement atomic JSON state file writes (temp + rename)
- [x] 5.3 Implement state loading with corruption recovery (warn + full re-scan)
- [x] 5.4 Write unit tests for state persistence and recovery

## 6. Organizer Feature

- [x] 6.1 Implement source directory recursive scanning with file type filtering
- [x] 6.2 Implement organize loop: scan → resolve date → build target path → copy/move
- [x] 6.3 Implement lightweight duplicate handling (filename, filedate, exif) with actions (replace, addSuffix, moveToFolder, skip)
- [x] 6.4 Implement dry-run mode (log planned actions, no filesystem mutations)
- [x] 6.5 Implement fallbackFolder handling for files without usable date
- [x] 6.6 Integrate state persistence (skip already-processed files, update state after each file)
- [x] 6.7 Implement error resilience (log file-level errors, continue with next file)
- [x] 6.8 Write integration tests for the full organize flow (copy, move, duplicates, dry-run, resume)

## 7. Duplicate Detection Feature

- [x] 7.1 Implement metadata-based detection (fileSize + exifDateTimeOriginal)
- [x] 7.2 Implement content-based detection using perceptual hash (goimagehash, fixed Hamming threshold ≤ 8)
- [x] 7.3 Implement cross-source matching across all configured sources
- [x] 7.4 Implement duplicate actions (moveToFolder, skip)
- [x] 7.5 Implement dry-run mode for duplicate detection
- [x] 7.6 Implement error resilience (single file failure doesn't abort the run)
- [x] 7.7 Write integration tests for duplicate detection (metadata, content, cross-source, dry-run)

## 8. Scheduling

- [x] 8.1 Implement cron scheduler wrapper using `robfig/cron/v3`
- [x] 8.2 Register organizer sources and duplicate detection jobs with their respective cron expressions
- [x] 8.3 Implement overlap prevention (lock per source/job, skip + log if already running)
- [x] 8.4 Write unit tests for scheduler registration and overlap prevention

## 9. Notifications

- [x] 9.1 Implement ntfy.sh HTTP client (POST with title + message body)
- [x] 9.2 Implement run summary notifications (success: file counts; failure: error counts)
- [x] 9.3 Implement fire-and-forget with timeout (notification failure must not block runs)
- [x] 9.4 Write unit tests for notification client (mock HTTP)

## 10. Main Entry Point & Docker

- [x] 10.1 Implement `cmd/phorganizer/main.go`: parse config, validate, start scheduler, block on signal
- [x] 10.2 Implement graceful shutdown (SIGTERM/SIGINT → finish current file → exit)
- [x] 10.3 Finalize Dockerfile and `docker-compose.yml` example
- [x] 10.4 End-to-end smoke test with Docker (organize + dedup with sample files)

