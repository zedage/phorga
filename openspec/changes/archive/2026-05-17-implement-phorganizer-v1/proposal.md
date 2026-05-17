## Why

Photos and videos accumulate across devices, NAS systems, and cloud sync folders without any consistent structure. Manual sorting by date is tedious and error-prone. Existing tools either require a GUI, lack Docker support, or don't handle Synology NAS environments well. Phorganizer fills this gap as a headless, daemon-based organizer that runs anywhere Docker runs — with zero user interaction after initial configuration.

## What Changes

- Introduce a new Go-based CLI/daemon application: **Phorganizer**.
- Implement the **Organizer** feature: automatic date-based photo/video organization from multiple sources into a structured target directory.
- Implement the **Duplicate Detection** feature (V1): metadata- and content-based duplicate detection across sources, running independently from the organizer.
- Provide a Docker image for easy deployment on any platform (Linux, macOS, Windows, Synology NAS).
- Support YAML/JSON configuration for all settings.
- Integrate ntfy.sh for optional push notifications on run summaries and errors.

## Capabilities

### New Capabilities
- `photo-organizer`: Date-based photo/video organization with configurable target structure, move/copy modes, per-source scheduling, lightweight duplicate handling, dry-run support, and state/resume across restarts.
- `duplicate-detection`: Independent content- and metadata-based duplicate detection across multiple sources, with configurable actions (moveToFolder, skip), dry-run support, and its own schedule.
- `config-management`: YAML/JSON configuration parsing and validation with fail-fast startup behavior.
- `notifications`: Optional ntfy.sh integration for run summary and error notifications.

### Modified Capabilities
<!-- No existing capabilities — this is a greenfield project. -->

## Impact

- **New codebase**: Entire Go project created from scratch (module, packages, Dockerfile, CI).
- **Dependencies**: goexif/images4/go-phash for metadata extraction, cron library for scheduling, ntfy.sh HTTP API for notifications.
- **Deployment**: Docker image published; docker-compose example provided.
- **Filesystem**: Reads from configured source paths, writes to target paths. Move mode deletes source files after successful transfer.

