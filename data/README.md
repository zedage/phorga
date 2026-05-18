# Phorga Test Data

This directory contains a ready-to-use test setup for Phorga.

## Structure

```
data/
  config.yaml      # Test configuration (organizer + duplicate detection)
  source/           # Source photos to be organized (generated)
  target/           # Target directory (organized output)
  duplicates/       # Duplicate folder
  fallback/         # Fallback for files without date metadata
  state/            # State files (resume support)
```

## Setup & Reset

Generate (or reset) all test images:

```bash
go run testdata/tools/generate.go
```

This creates ~12 sample images in `data/source/` including:
- JPEG photos with embedded EXIF dates (various months/years)
- PNG screenshots without EXIF
- Duplicate images (same content, different filenames)
- A nested `phone-backup/` subdirectory
- A `.txt` file (should be skipped)
- One pre-existing file in `data/target/` (to test duplicate handling)

## Run Phorga

```bash
go run ./cmd/phorga --config data/config.yaml
```

The demo config runs every minute. Press `Ctrl+C` to stop.

## Dry-Run First

Edit `data/config.yaml` and set `dryRun: true` to preview actions without moving files:

```yaml
organizer:
  - dryRun: true
    ...
```

## What to Expect

After one organizer run, `data/target/` will contain:

```
target/
  2024/
    07/   ← vacation-beach.jpg, vacation-sunset.jpg, IMG_20240715_copy.jpg
    12/   ← christmas-dinner.jpg (with suffix if duplicate detected)
  2025/
    01/   ← newyear-fireworks.jpg
    02/   ← diagram-arch.png (mtime-based)
    03/   ← screenshot-desktop.png (mtime-based)
    04/   ← spring-garden.jpg
    05/   ← selfie.jpg, food-photo.jpg
```

Files without metadata go to `data/fallback/`.
The `notes.txt` file is silently skipped.

