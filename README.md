# Phorga

Headless photo/video organizer and duplicate detector. Runs as a daemon, organizes media by date metadata into a configurable folder structure, and detects duplicates across sources.

## Features

- **Organizer**: date-based file organization (EXIF → filename → source path → mtime fallback), move/copy, per-source cron scheduling, lightweight duplicate handling, dry-run, state/resume
- **Duplicate Detection**: independent metadata- or content-based (perceptual hash) dedup across sources, moveToFolder/skip actions, dry-run
- **Notifications**: optional ntfy.sh push notifications on run completion
- **Docker-first**: single container, multi-arch (amd64/arm64), works on Synology NAS

## Quick Start

```bash
cp config.example.yaml config.yaml
# edit config.yaml with your paths and schedules

# Run directly
go build -o phorga ./cmd/phorga
./phorga --config config.yaml

# Or with Docker
docker compose up -d
```

## Configuration

YAML or JSON. See [`config.example.yaml`](config.example.yaml) for a complete example.

```yaml
organizer:
  - target: /photos/organized
    targetStructure: "{year}/{month}"   # tokens: {year}, {month}, {monthWithName}, {day}
    fallbackFolder: /photos/unknown     # files without date metadata
    sources:
      - path: /photos/incoming
        mode: move                      # move | copy
        schedule: "0 2 * * *"
        duplicates:
          detectionMode: filename       # filename, filedate, exif (combine with + or |)
          action:
            method: skip                # replace | addSuffix | moveToFolder | skip

duplicateDetection:
  - enabled: true
    schedule: "0 4 * * *"
    sources: [/photos/organized, /photos/archive]
    detectionMode: content              # content | metadata
    action:
      method: moveToFolder
      duplicateFolder: /photos/duplicates
```

Config resolution: `--config` flag → `PHORGA_CONFIG` env → `./config.yaml`

## Build & Test

```bash
go build ./...
go test ./...
```

## Demo / Test Setup

Generate sample images and run phorga locally:

```bash
go run testdata/tools/generate.go            # generate/reset test images
go run ./cmd/phorga --config data/config.yaml  # run with test config
```

Reset to original state at any time by re-running the generate command. See [`data/README.md`](data/README.md) for details.

## Project Structure

```
cmd/phorga/         Main entry point
internal/
  config/                YAML/JSON parsing, validation
  organizer/             Date-based organization engine
  dedup/                 Duplicate detection engine
  metadata/              EXIF, filename & path date extraction, target path resolution
  fileops/               Atomic copy/move operations
  state/                 JSON state persistence (resume support)
  scheduler/             Cron wrapper with overlap prevention
  notify/                ntfy.sh client
```

## Docker

```bash
docker build -t phorga .
docker compose up -d
```

Mount your source/target directories and `config.yaml` as volumes. State is persisted in `/data/state`.

## Documentation

Architecture documentation (arc42): [`docs/arc42/arc42.adoc`](docs/arc42/arc42.adoc)

## License

[Beer-Ware License](LICENSE) (Revision 42)

