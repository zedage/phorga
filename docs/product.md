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
- It only deletes a source file after the copy has been written to disk and checked for completeness.
- It keeps each file's own date (the modification time) when it copies or moves it.
- It never overwrites a file, unless you explicitly choose "replace" for duplicates. This holds even if another program writes a file with the same name at the same moment.
- It runs one job at a time, so its own jobs never get in each other's way.
- It has a dry-run mode to see what would happen.
- One broken file never stops a run.

## Organizer

The organizer takes files from one or more **source** folders and sorts them into a **target** folder. You can have several targets, each with its own sources. For example, "family photos" and "work photos" can be organized separately.

**How the date is found.** Phorga looks at the capture date stored in the file first: EXIF in JPEG and RAW files, the same EXIF data inside iPhone HEIC photos, and the recording time in MP4 and MOV videos ([ADR-0021](decisions/0021-built-in-heic-and-video-date-readers.md)). Some files have none (screenshots, WhatsApp images), so it then tries the filename (`IMG_20240715_…`), then folder names (`…/2019/07/…`), and only as a last resort the file's modification time. More details are in [ADR-0003](decisions/0003-date-resolution-order.md).

**Folder layout.** The `targetStructure` template decides how folders are named. `{year}/{month}` gives `2024/03`. You can use month names in German, French or Italian, and the spelling of the placeholder controls the case: `{Monat}` gives `März`, `{mois}` gives `mars`, `{MESE}` gives `MARZO`. The full token table is in the [README](../README.md#targetstructure-tokens).

**Move or copy.** Each source chooses whether files are moved (the source is cleaned up) or copied (the source stays untouched). A move within the same disk just renames the file, so even large videos move instantly ([ADR-0019](decisions/0019-no-clobber-transfers-with-hard-links.md)).

**When a file with the same name is already there.** The organizer can do a quick check for duplicates: same name, same file date, or same stored capture date (files without one never match on the capture date). You choose what happens then: replace it, keep both with a short random suffix, put the new one into a duplicates folder, or skip it. "Replace" is only allowed when the check looks at more than the name, so it can never overwrite a different photo that just happens to be called `IMG_0001.jpg` too. A file that is already in its right place is left alone. This is deliberately lightweight. Real content comparison is the job of duplicate detection (below).

**Files without a date.** Normally every file has at least a modification time. If that is only the day you copied the files, set `ignoreModTime`: files without a real date then go to an optional `fallbackFolder` instead of a wrong month. If a file with the same name is already there, the new one gets a short random suffix.

**Remembering progress.** Phorga remembers which files it has already handled. After a restart, it continues where it left off instead of starting over. It writes this down during a run as well, so even a crash in the middle of a large first run loses little.

**Schedules.** Every source has its own schedule in cron format, for example "every night at 2:00". A run never overlaps with itself. If a job is due while another one is still running, it waits for it and the status page says so ([ADR-0020](decisions/0020-run-one-job-at-a-time.md)).

**Folders that are not photos.** A NAS keeps hidden folders inside your photo shares: Synology stores thumbnails in `@eaDir`, deleted files in `#recycle` and snapshots in `#snapshot`. Phorga always skips these, and every file or folder whose name starts with a dot. You can leave out more with an `exclude` list per source, e.g. `Screenshots` or `2019/Scans`. Skipped folders are logged only at debug level, so they do not fill the normal log ([ADR-0016](decisions/0016-fixed-skip-list-and-simple-exclude-patterns.md)).

Exact rules: [specs/organizer.md](specs/organizer.md)

## Duplicate detection

Duplicate detection is a **separate feature** with its own folders, schedule and settings ([ADR-0001](decisions/0001-single-binary-independent-features.md)). It looks across *all* configured folders together. This finds the same holiday photo in your phone backup and in your archive.

Two modes:

- **content** compares what the images look like (a "perceptual hash"), so re-saved or resized copies are found too ([ADR-0004](decisions/0004-perceptual-hash-for-content-dedup.md)). Images that look alike are then compared area by area, so burst shots where someone blinked or moved are *not* treated as duplicates, while brightened or re-compressed copies still are ([ADR-0017](decisions/0017-detail-check-after-phash.md)). This works for images, not for videos.
- **metadata** is faster. It treats files as duplicates when they have the same size and the same stored capture date, which also works for HEIC photos and videos. Files without one are never matched this way, so no false matches come from copy dates.

**It stays fast on large libraries.** Phorga does not compare every photo with every other one. It sorts the fingerprints so that it only compares photos that could possibly match, which takes a fraction of a second even for 100,000 photos.

**It remembers what it has seen.** The first run reads every file, which takes a while on a large library. Phorga keeps each file's fingerprint in a cache, so later runs only read new or changed files and take seconds instead of minutes. Dry runs fill the cache too; it never touches your photos ([ADR-0015](decisions/0015-dedup-fingerprint-cache-written-in-dry-runs.md)).

It skips the same system and hidden folders as the organizer, and has its own `exclude` list. Without this, a Synology thumbnail would look like a copy of its photo, and the real photo could be moved away as the duplicate.

If several copies of the same picture are found, the best one stays and all others count as its duplicates. Best means the most pixels, then the larger file. So a camera photo stays and the small copy WhatsApp made of it is the duplicate, whatever the files are called ([ADR-0018](decisions/0018-keep-highest-resolution-duplicate.md)). Found duplicates are either moved to a duplicates folder or just reported. Nothing is deleted. A dry run shows the results without touching anything.

Exact rules: [specs/duplicate-detection.md](specs/duplicate-detection.md)

## Configuration

Everything is configured in one YAML or JSON file. Phorga checks the whole file at startup and refuses to start if anything is wrong. It then lists every problem at once, so you can fix them all in one go. Phorga finds the file via `--config`, the `PHORGA_CONFIG` environment variable, or `./config.yaml`.

You don't have to write that file yourself. If it doesn't exist yet, Phorga starts anyway, without any jobs, and opens its web UI so you can set it up in the browser. The first save creates the file ([ADR-0022](decisions/0022-live-apply-and-setup-mode.md)).

A complete, commented example is in [`config.example.yaml`](../config.example.yaml), and the reference is in the [README](../README.md#configuration).

Exact rules: [specs/configuration.md](specs/configuration.md)

## Web UI

For people who prefer not to edit YAML, Phorga can show a small **configuration page** in the browser, in German, English, Italian or French. It shows the configuration Phorga is currently running with. You can check your changes and then save them. Saving to the active file applies them right away, without a restart: new schedules start at once, and a job that is running at that moment finishes calmly with its old settings. Only a different state folder or web UI address needs a restart, and Phorga tells you so. You can also save into a separate file to try something out; that changes nothing in the running Phorga ([ADR-0022](decisions/0022-live-apply-and-setup-mode.md), [ADR-0009](decisions/0009-ui-save-destination-and-no-hot-reload.md)).

The page is meant to be usable without knowing the config format:

- **An overview first.** Each profile and duplicate detection job is a card that is closed by default and shows at a glance where files come from and where they go: the source folders with `copy` or `move` and their schedule, an arrow, and the target folder with its folder structure. One click opens the settings; rarely used ones sit under "More options". Settings you seldom touch (state folder, web UI, where to save) are folded away too. A sidebar jumps between sections, and the language can be switched in the top bar.
- **Your own names.** Give a profile or job a name such as "Robin's phone"; it is shown on the card and on the status page. Without one, it is simply numbered.
- **Add and remove** profiles, sources and duplicate detection jobs with a button. New entries start as dry runs, so nothing is touched until you have checked the log.
- **Plain-language choices.** Dropdowns say what each choice does, e.g. "copy – keep the original". When to treat a file as a duplicate is a set of checkboxes ("same file name", "same capture date", …) instead of a code like `filename+exif`. A short **help text** sits under every field.
- **Edit the file directly.** Advanced users can open the config file as text from the top of the page, change it and save. Phorga checks it with the same rules and applies it right away, and comments in the file are kept.
- **Live previews.** While you type a folder structure such as `{year}/{month}`, you see the folder it produces for an example date. While you type a schedule, you see its next three run times.
- **Errors next to the field.** "Validate" marks each wrong field directly, not only in a list at the top.
- **Folder picker.** A "Browse…" button and suggestions while typing, so you can pick folders on the NAS instead of typing paths. For safety it only shows folders below the locations you list in `webUI.browseRoots`, and it never changes anything. Without that list, it is off ([ADR-0013](decisions/0013-folder-picker-limited-to-browse-roots.md)).
- **Status page.** At the top, how many jobs are running, waiting or idle. Then, for every job: when it runs next, whether it is running right now or waiting for another job, and what the last run did (files processed, duplicates found, errors). While a job runs, you see what it is doing, for example "Scanning files: 1500 / 7402 (20 %)" with a progress bar and how long it has been running. Click a job to jump to its schedule on the configuration page. It refreshes by itself every 10 seconds. The last results survive a restart. Dry runs are shown but not stored ([ADR-0014](decisions/0014-persisted-run-status-without-dry-runs.md)).

- **Light and dark mode.** The page follows the light or dark setting of your system automatically. A switch in the top bar lets you force light or dark instead; your browser remembers the choice.

- **Help built in.** A Help page in the top bar has the step-by-step setup guide (Synology NAS or another computer), and a short summary of this installation: where the config file is, whether the Synology homes folder was found and how many users it has, and whether a password is set.

The page works without internet access. It is meant for your own computer or home network only, and by default it is reachable only from the same machine. A NAS has no browser, though, so to use the page from a laptop or phone you can protect it with a **password** (set in the container's environment, never in the config) and reach it through the NAS's reverse proxy with HTTPS. Phorga warns in its log when the page is open to the network without a password. Other websites you visit cannot change Phorga's settings through your browser ([ADR-0023](decisions/0023-optional-web-ui-password.md), [ADR-0008](decisions/0008-built-in-server-rendered-web-ui.md)).

Exact rules: [specs/web-ui.md](specs/web-ui.md)

## Synology Photos users

On a Synology NAS, each user's Synology Photos library is the `Photos` folder in their home folder, and the phone app uploads into `Photos/MobileBackup`. Phorga finds these users by itself when the NAS folder `homes` is available to it, and lists them at the top of the configuration page. Tick the users whose phone uploads should be sorted and click "Add selected users". Each one gets a ready-made profile: every night, new files in `MobileBackup` are moved into `Photos/<year>/<month>`. Photos that are already there with the same name and capture date stay where they are. The profile starts as a dry run. Check the log or status page, then switch dry run off and save.

Users who already have a profile are marked as set up. A user whose home folder Phorga may not read is marked "no access", which means the container's user lacks permission for it.

Exact rules: [specs/synology.md](specs/synology.md)

## Notifications

If you add an [ntfy](https://ntfy.sh) URL, Phorga sends a short summary to your phone after every run: how many files were processed or skipped, how many duplicates were found, and whether there were errors. If a notification cannot be delivered, the run itself is not affected.

Exact rules: [specs/notifications.md](specs/notifications.md)

## Platforms

Phorga is a single Go binary without external dependencies. It runs on Linux, macOS and Windows, and as a multi-arch Docker image (amd64/arm64), for example on a Synology NAS. A ready-made image is published on Docker Hub as `mnrd001/phorga`. Releases are named by date, e.g. `2026.11.0`, and `latest` is always the newest release, so a NAS only gets a new Phorga when it was released; the version shows in the web UI. It has no shell and runs as an unprivileged user; you tell it which NAS user to run as. Supported file types: JPEG, PNG, HEIC, RAW (raw, cr2, nef, arw), MP4 and MOV.

## Non-goals

- **No database and no cloud service.** State is plain JSON files ([ADR-0005](decisions/0005-json-state-files.md)).
- **No photo viewer or editing.** Phorga only moves and copies files.
- **No deletion of duplicates.** They are moved aside or reported, and a person decides.
- **No public-facing web app.** The UI is a local admin page, not a multi-user service.
- **No tunable dedup algorithms (yet).** The thresholds are fixed to keep the configuration simple.

## Starting and stopping

Phorga runs until you stop it, with Ctrl+C in a terminal or `docker stop` for the container. It then finishes the file it is working on, tells you in the log which jobs it is waiting for, and exits within a few seconds. A run that was interrupted this way is not shown as the last result on the status page, and no notification is sent for it. The organizer remembers how far it got and continues there next time.

If a stop ever takes too long, press Ctrl+C a second time to end Phorga immediately.

Exact rules: [specs/runtime.md](specs/runtime.md)
