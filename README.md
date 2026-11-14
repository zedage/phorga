# Phorga

Headless photo/video organizer and duplicate detector. Runs as a daemon, organizes media by date metadata into a configurable folder structure, and detects duplicates across sources.

## Features

- **Organizer**: date-based file organization (EXIF → filename → source path → mtime fallback), move/copy, per-source cron scheduling, lightweight duplicate handling, dry-run, state/resume. Transfers keep the file's modification time, are verified before the source is deleted, and never overwrite silently
- **Duplicate Detection**: independent metadata- or content-based (perceptual hash) dedup across sources, moveToFolder/skip actions, dry-run
- **Web UI**: optional built-in configuration UI with dropdowns, help texts, live previews, a folder picker and a job status page with live progress, in DE/EN/IT/FR
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

Stop Phorga with Ctrl+C (or `docker stop`). It finishes the current file and exits within a few seconds; press Ctrl+C again to quit at once.

## Configuration

YAML or JSON. Below is a short example; [`config.example.yaml`](config.example.yaml) is a complete, commented one, and the [reference](#reference) lists every field.

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

### Where Phorga looks for its config

Phorga uses the first of these that is set:

1. `--config <path>`
2. the environment variable `PHORGA_CONFIG` (set to `/config/config.yaml` in the Docker image)
3. `config.yaml` in the current working directory

Files ending in `.yaml`, `.yml` or `.json` are accepted. The config is read **once at startup**; after a change, restart Phorga. An invalid config stops Phorga at startup with exit code 1 and a list of all problems found. The config must define at least one `organizer` profile or one `duplicateDetection` job.

| Flag | Meaning |
|---|---|
| `--config <path>` | Config file to use (see above) |
| `--web-ui` | Start the web UI even if `webUI.enabled` is `false` |
| `--web-ui-addr <host:port>` | Listen address of the web UI, overrides `webUI.address` |

### Reference

Relative paths are resolved against the working directory. In Docker, use the paths *inside* the container (the right-hand side of your volume mounts).

**Top level**

| Field | Type | Required | Default | Meaning |
|---|---|---|---|---|
| `stateDir` | path | no | `/data/state` | Folder for Phorga's own files (see [Files in `stateDir`](#files-in-statedir)). Created if missing. |
| `organizer` | list | one of the two | – | Organizer profiles, see below. |
| `duplicateDetection` | list | one of the two | – | Duplicate detection jobs, see below. |
| `webUI` | object | no | off | Built-in web UI, see below. |

**`organizer[]`**: one entry per target folder

| Field | Type | Required | Default | Meaning |
|---|---|---|---|---|
| `target` | path | yes | – | Folder the organized files go into. |
| `targetStructure` | text | no | `{year}/{month}` | Subfolder template below `target`, see [tokens](#targetstructure-tokens). Unknown tokens are rejected. |
| `fallbackFolder` | path | no | – | Where files go when no date can be determined. Without it, such files are skipped and logged. |
| `dryRun` | bool | no | `false` | Only log what would happen. No file is changed, not even Phorga's state. |
| `ntfy.url` | URL | no | – | ntfy topic URL that receives a short summary after every run, e.g. `https://ntfy.sh/my-topic`. |
| `sources` | list | yes, ≥ 1 | – | Folders to take files from, see below. |

**`organizer[].sources[]`**: one entry per source folder, each with its own schedule

| Field | Type | Required | Default | Meaning |
|---|---|---|---|---|
| `path` | path | yes | – | Folder to read from, including all subfolders. |
| `mode` | `move` \| `copy` | yes | – | `move` deletes the source file after a verified copy; `copy` leaves it in place. |
| `schedule` | cron | yes | – | When this source runs, see [Schedules](#schedules). |
| `duplicates` | object | no | – | What to do when a file with the same name already exists in the target. Without it, such files are skipped and logged. |
| `duplicates.detectionMode` | text | yes, if `duplicates` | – | When an existing file counts as the same: `filename` (same name), `filedate` (same modification time), `exif` (same EXIF capture date; files without EXIF never match). Combine with `+` (all must match) or `\|` (any may match), not both. If it doesn't match, the file is kept with a random suffix. |
| `duplicates.action.method` | `replace` \| `addSuffix` \| `moveToFolder` \| `skip` | yes, if `duplicates` | – | `replace` overwrites the existing file; `addSuffix` keeps both (`photo-a3f9.jpg`); `moveToFolder` puts the new file into `duplicateFolder`; `skip` leaves it in the source. |
| `duplicates.action.duplicateFolder` | path | for `moveToFolder` | – | Folder for duplicates. Existing files there are never overwritten. |

**`duplicateDetection[]`**: one entry per job

| Field | Type | Required | Default | Meaning |
|---|---|---|---|---|
| `enabled` | bool | no | `false` | Only enabled jobs are scheduled. Disabled jobs are still validated. |
| `sources` | list of paths | yes, ≥ 1 | – | Folders to compare, including all subfolders. Duplicates are found across all of them. |
| `schedule` | cron | yes | – | When the job runs, see [Schedules](#schedules). |
| `detectionMode` | `content` \| `metadata` | yes | – | `content` compares what images look like (perceptual hash; JPEG and PNG only); `metadata` compares file size and EXIF capture date (much faster; files without EXIF never match). |
| `action.method` | `moveToFolder` \| `skip` | yes | – | `moveToFolder` moves each duplicate into `duplicateFolder` (the first file found stays); `skip` only logs it. Nothing is ever deleted. |
| `action.duplicateFolder` | path | for `moveToFolder` | – | Folder for duplicates. Existing files there are never overwritten. |
| `dryRun` | bool | no | `false` | Only log what would be done. No photo is moved. |
| `ntfy.url` | URL | no | – | ntfy topic URL for a summary after every run. |

**`webUI`**

| Field | Type | Required | Default | Meaning |
|---|---|---|---|---|
| `enabled` | bool | no | `false` | Start the web UI together with Phorga (also possible with `--web-ui`). |
| `address` | `host:port` | no | `127.0.0.1:8080` | Listen address. In Docker use `0.0.0.0:8080` and publish the port on `127.0.0.1` only. |
| `browseRoots` | list of absolute paths | no | – | Folders the folder picker may show. Each must exist. Without it, the picker is off. |

### Schedules

`schedule` is a standard 5-field cron expression: `minute hour day-of-month month day-of-week`, evaluated in the **local time of the machine** (in Docker, set `TZ`, e.g. `TZ=Europe/Zurich`). A run never overlaps with itself: if the previous run is still busy, the next one is skipped.

| Expression | Runs |
|---|---|
| `0 2 * * *` | every day at 02:00 |
| `30 3 * * 0` | every Sunday at 03:30 |
| `0 */6 * * *` | every 6 hours |
| `*/15 * * * *` | every 15 minutes |

The web UI shows the next three run times while you type.

### Files in `stateDir`

| File | Written by | Content |
|---|---|---|
| `org-<target>-<source>.state.json` | organizer, one per source | Files already handled, so later runs skip them. Never written in a dry run. |
| `run-status.json` | all jobs | Last result of each job for the status page. Dry runs are not stored. |

All files are written atomically. Deleting a state file makes the organizer look at all files of that source again (already organized files are recognized as duplicates).

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

## Known Limits

- **HEIC and video dates** come from the filename, the folder or the modification time. Phorga reads capture dates only from EXIF in JPEG and TIFF-based RAW files.
- **Content duplicate detection** compares JPEG and PNG only. HEIC, RAW and videos never match. Every run decodes every image again, which takes long on a large library.
- **Content duplicate detection keeps the first file it finds**, not the best one, and very similar shots (bursts) can count as duplicates. Duplicates are only moved to `duplicateFolder`, never deleted, so review that folder before you delete anything.
- **Other file types stay in the source without a log line**, for example DNG, GIF, WebP, TIFF, AVI, M4V, 3GP and sidecars (XMP, AAE). The photo and the video of a Live Photo can land in different folders.
- **Organizer duplicates with `detectionMode: filename` and `method: replace`** overwrite a different photo that happens to have the same name. Prefer `filename+exif` with `skip` or `moveToFolder`.
- **The container runs as root by default**, so new files on the NAS belong to root. Set `user: "<uid>:<gid>"` in `docker-compose.yml` to change that.

**Before you point Phorga at your only copy of your photos:** make a backup, start with `mode: copy` and `dryRun: true`, check the log and the status page, and switch to `mode: move` only after a few clean runs.

## Build & Test

```bash
go build ./...
go vet ./...
go test ./...
```

CI runs the same three commands on every pull request ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)).

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

