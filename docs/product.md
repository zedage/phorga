# Phorga: The Other Photo Organizer

This document explains **what Phorga does and why**, for people. It is the place to start if you want to understand the product before reading code.

- Exact, testable rules for each behavior: [specs/](specs/README.md)
- How it is built: [arc42 architecture](arc42/arc42.adoc)
- Why it is built this way: [decisions/](decisions/README.md)
- How to install and configure it: [README](../README.md)

---

## Vision

Photos pile up in many places: phone backups, messenger downloads, camera cards, old archive folders. Phorga runs quietly in the background, usually as a Docker container on a NAS. It wakes up on a schedule and puts every photo and video into a clean folder per year and month, using the date the picture was **taken**, not the date the file was copied. It can also find duplicates across folders and set them aside.

Phorga is meant to be **set up once and then left alone**. That is why safety matters more than features:

- It never leaves half-written files behind.
- It only deletes a source file after the copy has been written successfully.
- It has a dry-run mode to see what would happen.
- One broken file never stops a run.

## Organizer

The organizer takes files from one or more **source** folders and sorts them into a **target** folder. You can have several targets, each with its own sources. For example, "family photos" and "work photos" can be organized separately.

**How the date is found.** Phorga looks at the EXIF capture date first. Many files have none (screenshots, WhatsApp images, videos), so it then tries the filename (`IMG_20240715_…`), then folder names (`…/2019/07/…`), and only as a last resort the file's modification time. More details are in [ADR-0003](decisions/0003-date-resolution-order.md).

**Folder layout.** The `targetStructure` template decides how folders are named. `{year}/{month}` gives `2024/03`. You can use month names in German, French or Italian, and the spelling of the placeholder controls the case: `{Monat}` gives `März`, `{mois}` gives `mars`, `{MESE}` gives `MARZO`. The full token table is in the [README](../README.md#targetstructure-tokens).

**Move or copy.** Each source chooses whether files are moved (the source is cleaned up) or copied (the source stays untouched).

**When a file with the same name is already there.** The organizer can do a quick check for duplicates: same name, same file date, or same capture date. You choose what happens then: replace it, keep both with a short random suffix, put the new one into a duplicates folder, or skip it. This is deliberately lightweight. Real content comparison is the job of duplicate detection (below).

**Files without a date** go to an optional `fallbackFolder`.

**Remembering progress.** Phorga remembers which files it has already handled. After a restart, it continues where it left off instead of starting over.

**Schedules.** Every source has its own schedule in cron format, for example "every night at 2:00". A run never overlaps with itself.

Exact rules: [specs/organizer.md](specs/organizer.md)

## Duplicate detection

Duplicate detection is a **separate feature** with its own folders, schedule and settings ([ADR-0001](decisions/0001-single-binary-independent-features.md)). It looks across *all* configured folders together. This finds the same holiday photo in your phone backup and in your archive.

Two modes:

- **content** compares what the images look like (a "perceptual hash"), so re-saved or resized copies are found too ([ADR-0004](decisions/0004-perceptual-hash-for-content-dedup.md)). This works for images, not for videos.
- **metadata** is faster. It treats files as duplicates when they have the same size and the same capture date.

Found duplicates are either moved to a duplicates folder or just reported. Nothing is deleted. A dry run shows the results without touching anything.

Exact rules: [specs/duplicate-detection.md](specs/duplicate-detection.md)

## Configuration

Everything is configured in one YAML or JSON file. Phorga checks the whole file at startup and refuses to start if anything is wrong. It then lists every problem at once, so you can fix them all in one go. Phorga finds the file via `--config`, the `PHORGA_CONFIG` environment variable, or `./config.yaml`.

A complete, commented example is in [`config.example.yaml`](../config.example.yaml), and the reference is in the [README](../README.md#configuration).

Exact rules: [specs/configuration.md](specs/configuration.md)

## Web UI

For people who prefer not to edit YAML, Phorga can show a small **configuration page** in the browser, in German, English, Italian or French. It shows the configuration Phorga is currently running with. You can check your changes, and then save either over the active file or into a new file to try them out. Phorga picks up saved changes after a restart ([ADR-0009](decisions/0009-ui-save-destination-and-no-hot-reload.md)).

The UI has no login. It is meant for your own computer or home network only, and by default it is reachable only from the same machine ([ADR-0008](decisions/0008-built-in-server-rendered-web-ui.md)).

Exact rules: [specs/web-ui.md](specs/web-ui.md)

## Notifications

If you add an [ntfy](https://ntfy.sh) URL, Phorga sends a short summary to your phone after every run: how many files were processed or skipped, how many duplicates were found, and whether there were errors. If a notification cannot be delivered, the run itself is not affected.

Exact rules: [specs/notifications.md](specs/notifications.md)

## Platforms

Phorga is a single Go binary without external dependencies. It runs on Linux, macOS and Windows, and as a multi-arch Docker image (amd64/arm64), for example on a Synology NAS. Supported file types: JPEG, PNG, HEIC, RAW (raw, cr2, nef, arw), MP4 and MOV.

## Non-goals

- **No database and no cloud service.** State is plain JSON files ([ADR-0005](decisions/0005-json-state-files.md)).
- **No photo viewer or editing.** Phorga only moves and copies files.
- **No deletion of duplicates.** They are moved aside or reported, and a person decides.
- **No public-facing web app.** The UI is a local admin page, not a multi-user service.
- **No tunable dedup algorithms (yet).** The thresholds are fixed to keep the configuration simple.
