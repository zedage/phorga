# Phorga

Headless photo/video organizer and duplicate detector. Runs as a daemon, organizes media by date metadata into a configurable folder structure, and detects duplicates across sources.

## Features

- **Organizer**: date-based file organization (EXIF → filename → source path → mtime fallback), move/copy, per-source cron scheduling, lightweight duplicate handling, dry-run, state/resume
- **Duplicate Detection**: independent metadata- or content-based (perceptual hash) dedup across sources, moveToFolder/skip actions, dry-run
- **Web UI**: optional built-in configuration UI with dropdowns, help texts, live previews, a folder picker and a job status page, in DE/EN/IT/FR
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
    targetStructure: "{year}/{month}"   # tokens explained below
    fallbackFolder: /photos/unknown     # files without date metadata
    sources:
      - path: /photos/incoming
        mode: move                      # move | copy
        schedule: "0 2 * * *"
        duplicates:
          detectionMode: filename       # filename, filedate, exif (EXIF date only); combine with + or |
          action:
            method: skip                # replace | addSuffix | moveToFolder | skip

duplicateDetection:
  - enabled: true
    schedule: "0 4 * * *"
    sources: [/photos/organized, /photos/archive]
    detectionMode: content              # content (pHash) | metadata (size + EXIF date)
    action:
      method: moveToFolder
      duplicateFolder: /photos/duplicates

webUI:
  enabled: true
  address: 127.0.0.1:8080
```

`webUI` is optional. The UI is intended for **local/private deployments**. Do not expose it publicly without additional protection.

### Web UI

Phorga can expose a small built-in configuration UI from the same process that runs the scheduler.

- Server-rendered HTML with HTMX-compatible validation/save endpoints. HTMX and the stylesheet are embedded in the binary, so no internet access is needed. Light/dark mode follows the system setting, with a manual switch in the top bar.
- UI localization: German, English, Italian, French
- Displays the currently loaded effective configuration values
- Dropdowns for fields with fixed values, and a help text under every field
- Live previews: the folder a `targetStructure` produces (for 15 March 2024), and the next 3 runs of each cron `schedule`
- Validation errors are shown next to the field they belong to
- Folder picker ("Browse…" dialog and autocomplete) for all path fields, limited to `webUI.browseRoots`
- Status page (`/ui/status`): next run, running state and last result of every job, refreshed every 10 s
- Saves either back to the active config file or to a separate config file

Enable it in config:

```yaml
webUI:
  enabled: true
  address: 127.0.0.1:8080
```

Or override at startup:

```bash
go run ./cmd/phorga --config config.yaml --web-ui
go run ./cmd/phorga --config config.yaml --web-ui --web-ui-addr 127.0.0.1:9090
```

The UI shows the values currently loaded at startup. Saving to a separate file writes a new config file but does not automatically reload the running scheduler.

**Folder picker.** It is off until you list the folders it may browse:

```yaml
webUI:
  enabled: true
  address: 127.0.0.1:8080
  browseRoots:
    - /photos   # absolute paths; in Docker, the mounted media folder
```

The picker shows only folders (no files, no hidden folders) below these roots, and never creates or changes anything.

**Status page.** The last result of each job is stored in `<stateDir>/run-status.json`, so it survives a restart. Dry runs are shown too, but only kept in memory, so a dry run never writes a file.

### TargetStructure Tokens

Phorga supports **multilingual month and day names** in `targetStructure`. Tokens are **case-sensitive** for output formatting.

#### Supported Tokens

| Token | Format | Language | Example |
|-------|--------|----------|---------|
| `{year}`, `{jahr}`, `{année}`, `{anno}` | 4-digit numeric | EN/DE/FR/IT | 2024 |
| `{month}` | 2-digit numeric | English context | 03 |
| `{monat}` | month name | Deutsch | **märz** (case: `{märz}`, `{März}`, `{MÄRZ}`) |
| `{mois}` | month name | Français | **mars** (case: `{mars}`, `{Mars}`, `{MARS}`) |
| `{mese}` | month name | Italiano | **marzo** (case: `{marzo}`, `{Marzo}`, `{MARZO}`) |
| `{monthWithName}` | numeric + English name | English | 03-march |
| `{monthNumber}` | 2-digit zero-padded | all | 03 |
| `{day}`, `{tag}`, `{jour}`, `{giorno}` | 2-digit numeric | EN/DE/FR/IT | 15 |
| `{dayNumber}` | 2-digit zero-padded | all | 15 |

#### Case Sensitivity

Month names respect the case of the placeholder token:

```yaml
# Examples with date 2024-03-15:

targetStructure: "{year}/{monat}"      # → 2024/märz
targetStructure: "{year}/{Monat}"      # → 2024/März
targetStructure: "{year}/{MONAT}"      # → 2024/MÄRZ

targetStructure: "{year}/{mois}/{jour}"           # → 2024/mars/15
targetStructure: "{year}/{Mois}/{Jour}"           # → 2024/Mars/15
targetStructure: "{year}/{monthNumber}-{Monat}"   # → 2024/03-März
```

#### Examples for Different Users

**German user:**
```yaml
targetStructure: "{year}/{Monat}"              # 2024/März
targetStructure: "{year}/{Monat}/{Tag}"        # (Tag=numeric 15)
```

**French user:**
```yaml
targetStructure: "{year}/{Mois}"               # 2024/Mars
targetStructure: "{year}/{mois}/{jour}"        # 2024/mars/15
```

**Italian user:**
```yaml
targetStructure: "{year}/{Mese}"               # 2024/Marzo
targetStructure: "{anno}/{mese}/{giorno}"      # 2024/marzo/15
```

**Mixed format user:**
```yaml
targetStructure: "{year}/{monthNumber}-{Monat}/{dayNumber}"  # 2024/03-März/15
targetStructure: "{year}/{monthNumber}_{Monat}"              # 2024/03_März
```

## Build & Test

```bash
go build ./...
go test ./...
```

## Demo / Test Setup

Generate sample images and run Phorga locally:

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
  runstatus/             Last run result per job for the status page
  webui/                 Optional built-in configuration UI (HTTP, templates, i18n, folder picker, status page)
```

## Docker

```bash
docker build -t phorga .
docker compose up -d

# Multi-arch (e.g. for an arm64 NAS); Go cross-compiles, no emulation needed for the build
docker buildx build --platform linux/amd64,linux/arm64 -t <registry>/phorga --push .
```

Mount your source/target directories and `config.yaml` as volumes. State is persisted in `/data/state`.

**Web UI in Docker:** inside the container, `127.0.0.1` is not reachable from the host. Set `webUI.address: 0.0.0.0:8080` in `config.yaml` and publish the port on the host's loopback only, e.g. `-p 127.0.0.1:8080:8080` (see the commented `ports` entry in [`docker-compose.yml`](docker-compose.yml)). The UI has no authentication, so do not publish it on all interfaces.

## Documentation

- [`docs/product.md`](docs/product.md): what Phorga does and why
- [`docs/specs/`](docs/specs/README.md): exact, testable requirements per feature
- [`docs/arc42/arc42.adoc`](docs/arc42/arc42.adoc): architecture
- [`docs/decisions/`](docs/decisions/README.md): architecture decision records

## Contributing

Start with [`AGENTS.md`](AGENTS.md). It covers the repo layout, commands, conventions, and the change workflow (spec → code + tests → docs) for people and AI agents.

## License

[Beer-Ware License](LICENSE) (Revision 42)

