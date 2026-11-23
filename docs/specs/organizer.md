# Spec: Organizer (`ORG`)

> Human explanation: [product.md › Organizer](../product.md#organizer) · Architecture: [arc42 §6 Organizer Run](../arc42/arc42.adoc) · Format: [README](README.md)

The organizer sorts media files from source directories into a date-based folder structure under a target directory. Each `organizer[]` profile has one target and one or more sources.

---

### ORG-001 Date-based organization
**Status:** implemented

The system MUST place each supported media file from a source into `<target>/<resolved targetStructure>/<original filename>`. The date used is the file's capture date, determined as described in ORG-002.

- **WHEN** a source contains a JPEG with EXIF DateTimeOriginal `2024-03-15 14:30:00` and `targetStructure` is `{year}/{month}`
  **THEN** the file is placed in `<target>/2024/03/`

**Code:** `internal/organizer/organizer.go`, `internal/metadata/metadata.go`, `internal/metadata/targetpath.go`
**Tests:** `internal/organizer/organizer_test.go#TestOrganize_CopyMode`, `internal/organizer/organizer_test.go#TestOrganize_MoveMode`

### ORG-002 Date resolution order
**Status:** implemented

The system MUST determine a file's date with the first source that succeeds, in this order:

1. The **embedded capture date**:
   - JPEG and TIFF-based RAW: EXIF `DateTimeOriginal` (falling back to EXIF `DateTime`)
   - HEIC/HEIF: the same EXIF fields, read from the file's `Exif` item
   - MP4/MOV: the creation time of the movie header (`mvhd`). It is stored in UTC and MUST be converted to the local time zone (`TZ`) before the folder is chosen. A value of 0 or outside 1970–2100 counts as no date.
2. A date pattern in the filename (e.g. `IMG_20240715_143000.jpg`, `2024-07-15 14-30-00.jpg`, `IMG-20240715-WA0001.jpg`)
3. A date pattern in the source directory path (e.g. `/photos/2019/07/…`, `/photos/2019-07-15/…`, `/photos/2019/März/…`)
4. The file modification time (mtime), unless the profile sets `ignoreModTime: true`

The readers MUST be pure Go (no CGO, ADR-0021) and MUST treat a truncated or malformed file as "no embedded date", never crash or read beyond a few MB.

- **WHEN** a file has no EXIF date but is named `IMG_20240715_143000.jpg`
  **THEN** the date `2024-07-15` is used, even if the mtime differs
- **WHEN** a file has no EXIF date and no date in its name, but lies in `/photos/2019/07/`
  **THEN** a date in July 2019 is used
- **WHEN** none of the above yields a date
  **THEN** the file's mtime is used
- **WHEN** an iPhone photo `IMG_4711.HEIC` has EXIF `DateTimeOriginal` `2023-08-19 17:42` and was copied to the NAS today
  **THEN** it goes to `2023/08/`
- **WHEN** a video's `mvhd` creation time is `2021-12-24 12:00 UTC` and `TZ` is `Europe/Zurich`
  **THEN** the date is `2021-12-24 13:00` local time
- **WHEN** a video's creation time is 0 and it lies in `/photos/2018/05/`
  **THEN** a date in May 2018 is used (path)
- **WHEN** `ignoreModTime` is `true` and a file has no embedded, filename or path date
  **THEN** no date is found and ORG-003 applies

**Code:** `internal/metadata/metadata.go`, `internal/metadata/embedded.go`, `internal/organizer/organizer.go`
**Tests:** `internal/metadata/metadata_test.go#TestExtractDate_FallbackToMtime`, `internal/metadata/metadata_test.go#TestExtractFilenameDate`, `internal/metadata/metadata_test.go#TestExtractDate_FilenameBeforeMtime`, `internal/metadata/metadata_test.go#TestExtractPathDate`, `internal/metadata/metadata_test.go#TestExtractDate_PathBeforeMtime`, `internal/metadata/exif_test.go#TestExtractDate_EXIFBeforeFilename`, `internal/metadata/embedded_test.go#TestExtractDate_HEICUsesEXIF`, `internal/metadata/embedded_test.go#TestExtractDate_VideoUsesQuickTimeCreationTime`, `internal/metadata/embedded_test.go#TestExtractDate_VideoWithoutCreationTimeFallsBack`, `internal/metadata/embedded_test.go#TestExtractEmbeddedDate_BrokenFilesAreNoDate`, `internal/metadata/embedded_test.go#TestResolveDate_WithoutModTime`, `internal/organizer/dates_test.go#TestOrganize_HEICAndVideoUseEmbeddedDates`, `internal/organizer/dates_test.go#TestOrganize_IgnoreModTimeSendsUndatedFilesToFallback`, `internal/webui/ignoremodtime_test.go#TestIgnoreModTimeIsEditable`, `internal/config/detection_test.go#TestSaveAndLoadKeepsIgnoreModTime`
**Notes:** The old openspec spec said "EXIF → mtime". The filename and path steps were added later, and this spec now matches the code. See [ADR-0003](../decisions/0003-date-resolution-order.md). Until 2026-11, HEIC and video files had no embedded date step and usually ended up with the mtime, see [ADR-0021](../decisions/0021-built-in-heic-and-video-date-readers.md). Apple also stores a local creation date with time zone in the QuickTime `meta` box (`com.apple.quicktime.creationdate`); it is not read. Some cameras write local time into `mvhd` instead of UTC, which shifts their videos by the zone offset. The HEIC tests use synthetic files; a HEIC written by macOS ImageIO was checked manually.

### ORG-003 Fallback folder for files without a date
**Status:** implemented

If no date can be determined for a file (only possible with `ignoreModTime: true`, ORG-002), the system MUST transfer it to `fallbackFolder` when one is configured (respecting `mode` and `dryRun`). Otherwise it MUST skip the file and log a warning. A file that is already in `fallbackFolder` MUST NOT be overwritten (ORG-020), and a transferred file MUST be recorded in the state (ORG-015), so `copy` mode does not copy it again on every run.

- **WHEN** no date can be determined and `fallbackFolder` is set
  **THEN** the file is copied/moved to `fallbackFolder`
- **WHEN** no date can be determined and `fallbackFolder` is not set
  **THEN** the file is skipped and a warning is logged
- **WHEN** `ignoreModTime` is `true`, `fallbackFolder` is set and a source holds `scan.png` (no date anywhere) and `IMG_20240715_143000.jpg`
  **THEN** `scan.png` goes to `fallbackFolder` and the other file to `2024/07/`

**Code:** `internal/organizer/organizer.go`
**Tests:** `internal/organizer/organizer_test.go#TestOrganize_FallbackFolder`, `internal/organizer/dates_test.go#TestOrganize_IgnoreModTimeSendsUndatedFilesToFallback`, `internal/organizer/dates_test.go#TestOrganize_IgnoreModTimeWithoutFallbackSkips`, `internal/organizer/organizer_test.go#TestHandleNoDate_WithFallback_Copy`, `internal/organizer/organizer_test.go#TestHandleNoDate_WithFallback_Move`, `internal/organizer/organizer_test.go#TestHandleNoDate_WithFallback_DryRun`, `internal/organizer/organizer_test.go#TestHandleNoDate_WithoutFallback`, `internal/organizer/safety_test.go#TestHandleNoDate_FallbackNeverOverwrites`, `internal/organizer/safety_test.go#TestHandleNoDate_CopyIsRemembered`
**Notes:** Without `ignoreModTime`, mtime is the last step of ORG-002 and always succeeds, so `fallbackFolder` was unreachable until 2026-11. `ignoreModTime` is useful when the mtime is just the day the files were copied.

### ORG-004 Move and copy modes
**Status:** implemented

Each source MUST have a `mode` of `move` or `copy`. Writes MUST be atomic (temp file + rename, or a hard link, ORG-020). In `move` mode, the source file MUST be deleted only after the target write succeeded. A move within one volume does not copy at all (ORG-025).

- **WHEN** mode is `copy` and the target write succeeds
  **THEN** the source file is unchanged
- **WHEN** mode is `move` and the target write succeeds
  **THEN** the source file is removed
- **WHEN** a copy fails midway
  **THEN** no temp file is left in the target directory

**Code:** `internal/organizer/organizer.go`, `internal/fileops/fileops.go`
**Tests:** `internal/organizer/organizer_test.go#TestOrganize_CopyMode`, `internal/organizer/organizer_test.go#TestOrganize_MoveMode`, `internal/organizer/organizer_test.go#TestTransferFile_InvalidMode`, `internal/fileops/fileops_test.go#TestAtomicCopy`, `internal/fileops/fileops_test.go#TestAtomicMove`, `internal/fileops/fileops_test.go#TestAtomicCopy_NoTempLeftOnError`, `internal/fileops/noclobber_test.go#TestAtomicMove_OtherVolumeCopiesAndRemovesSource`

### ORG-005 Target structure and default
**Status:** implemented

`targetStructure` MUST be a path template with `{token}` placeholders. Literal text outside tokens is kept as-is. If it is empty, it MUST default to `{year}/{month}`.

English tokens: `{year}` (4 digits), `{month}` (2 digits), `{day}` (2 digits), `{monthWithName}` (`01-january`), `{monthNumber}`, `{dayNumber}`.

- **WHEN** `targetStructure` is not set
  **THEN** `{year}/{month}` is used
- **WHEN** `targetStructure` is `{year}/{monthWithName}/{day}` and the date is 2024-01-15
  **THEN** the path is `2024/01-january/15`
- **WHEN** `targetStructure` is `photos/{year}/{monthWithName}/{day}` and the date is 2024-01-15
  **THEN** the path is `photos/2024/01-january/15`

**Code:** `internal/metadata/targetpath.go`, `internal/config/loader.go`
**Tests:** `internal/metadata/metadata_test.go#TestResolveTargetPath`, `internal/config/defaults_test.go#TestLoadAppliesDefaults`

### ORG-006 Multilingual tokens
**Status:** implemented

The system MUST support German, French and Italian tokens alongside English:

| Meaning | EN | DE | FR | IT |
|---|---|---|---|---|
| Year (4 digits) | `{year}` | `{jahr}` | `{année}` | `{anno}` |
| Month **name** | — | `{monat}` | `{mois}` | `{mese}` |
| Day (2 digits) | `{day}` | `{tag}` | `{jour}` | `{giorno}` |

Note that the English `{month}` renders the month as a **number**. The German, French and Italian month tokens render the month **name**.

- **WHEN** `targetStructure` is `{year}/{Monat}` and the date is 2024-02-15
  **THEN** the path is `2024/Februar`
- **WHEN** `targetStructure` is `{year}/{Mois}` and the date is 2024-02-15
  **THEN** the path is `2024/Février`
- **WHEN** `targetStructure` is `{anno}/{mese}/{giorno}` and the date is 2024-02-15
  **THEN** the path is `2024/febbraio/15`

**Code:** `internal/metadata/targetpath.go`
**Tests:** `internal/metadata/metadata_test.go#TestResolveTargetPath`, `internal/organizer/organizer_test.go#TestOrganize_MultilingualTokens_German`, `internal/organizer/organizer_test.go#TestOrganize_MultilingualTokens_French`, `internal/organizer/organizer_test.go#TestOrganize_MultilingualTokens_Mixed`

### ORG-007 Case-sensitive rendering of month names
**Status:** implemented

The case of the token MUST determine the case of the rendered name: all lowercase → lowercase, all uppercase → uppercase, otherwise → first letter uppercase.

- **WHEN** the token is `{monat}` and the date is in February
  **THEN** the output is `februar`
- **WHEN** the token is `{Monat}`
  **THEN** the output is `Februar`
- **WHEN** the token is `{MONAT}`
  **THEN** the output is `FEBRUAR`

**Code:** `internal/metadata/targetpath.go`
**Tests:** `internal/metadata/metadata_test.go#TestResolveTargetPath`

### ORG-008 Numeric placeholders are always two digits
**Status:** implemented

`{month}`, `{day}`, `{monthNumber}`, `{dayNumber}` and their language aliases MUST render zero-padded two-digit numbers.

- **WHEN** `targetStructure` is `{year}/{monthNumber}/{dayNumber}` and the date is 2024-02-05
  **THEN** the path is `2024/02/05`
- **WHEN** `targetStructure` is `{year}/{monthNumber}-{Monat}/{dayNumber}` and the date is 2024-02-05
  **THEN** the path is `2024/02-Februar/05`

**Code:** `internal/metadata/targetpath.go`
**Tests:** `internal/metadata/metadata_test.go#TestResolveTargetPath`, `internal/metadata/metadata_test.go#TestResolveTargetPath_EdgeDates`

### ORG-009 Unknown tokens are rejected
**Status:** implemented

A `targetStructure` that contains an unknown or unclosed token MUST fail configuration validation at startup (see CFG-002). Token lookup ignores case, so `{Year}` is valid.

- **WHEN** `targetStructure` is `{year}/{invalid}` or `{year}/{month_nl}`
  **THEN** validation fails and names the unknown token

**Code:** `internal/config/validate.go`, `internal/metadata/targetpath.go`
**Tests:** `internal/metadata/metadata_test.go#TestResolveTargetPath_InvalidToken`, `internal/config/config_test.go#TestValidateInvalidTargetToken`
**Notes:** The token list exists twice, in `validTargetTokens` (`internal/config/validate.go`) and `validTokens` (`internal/metadata/targetpath.go`). Change both together.

### ORG-010 Per-source scheduling without overlap
**Status:** implemented

Each source MUST run on its own 5-field cron `schedule`, independently of other sources. A run MUST NOT overlap with itself: if the schedule fires while the previous run is still active, the new run is skipped and a warning is logged. Runs of *different* jobs wait for each other (RUN-002).

- **WHEN** source A has `0 2 * * *` and source B has `0 4 * * *`
  **THEN** A runs at 02:00 and B runs at 04:00, independently
- **WHEN** a run is still active and its schedule fires again
  **THEN** the new run is skipped with a warning

**Code:** `internal/scheduler/scheduler.go`, `cmd/phorga/main.go`
**Tests:** `internal/scheduler/scheduler_test.go#TestRegisterAndRun`, `internal/scheduler/scheduler_test.go#TestOverlapPrevention`

### ORG-011 Lightweight duplicate detection during organization
**Status:** implemented

When a file with the same name already exists at the target path, the organizer MUST decide whether it is a duplicate using `duplicates.detectionMode`. The organizer MUST NOT use content-based detection; that belongs to DUP.

Fields:
- `filename`: same base name
- `filedate`: same mtime
- `exif`: same **embedded** capture date (step 1 of ORG-002: EXIF in JPEG, RAW and HEIC; the creation time in MP4/MOV). Files without one never match, even if a date can be resolved from their filename, path or mtime

Fields can be combined with `+` (AND) or `|` (OR). Mixing `+` and `|` in one expression MUST be rejected at validation.

- **WHEN** `detectionMode` is `filename|filedate` and only the filename matches
  **THEN** it is a duplicate
- **WHEN** `detectionMode` is `filename+filedate` and the mtimes differ
  **THEN** it is not a duplicate
- **WHEN** `detectionMode` is `exif` and both files have no EXIF but the same filename date (`IMG_20240315_143000.jpg`)
  **THEN** it is not a duplicate
- **WHEN** `detectionMode` is `filename+filedate|exif`
  **THEN** validation fails

**Code:** `internal/organizer/organizer.go`, `internal/config/validate.go`
**Tests:** `internal/organizer/organizer_test.go#TestIsDuplicate_Filename`, `internal/organizer/organizer_test.go#TestIsDuplicate_UnknownField`, `internal/organizer/organizer_test.go#TestOrganize_FiledateDetection`, `internal/organizer/organizer_test.go#TestOrganize_ExifDetection`, `internal/organizer/organizer_test.go#TestOrganize_OrDetection`, `internal/organizer/organizer_test.go#TestOrganize_AndDetection_NotMatching`, `internal/config/config_test.go#TestValidateMixedOperators`, `internal/organizer/exif_detection_test.go#TestOrganize_ExifDetection_SameEXIFDateIsDuplicate`, `internal/organizer/exif_detection_test.go#TestOrganize_ExifDetection_NoEXIFNeverMatches`, `internal/organizer/dates_test.go#TestOrganize_ExifDetectionComparesVideoCreationTime`, `internal/metadata/embedded_test.go#TestExtractEmbeddedDate_HEICAndVideo`, `internal/config/detection_test.go#TestParseDetectionMode`
**Notes:** The expression is parsed once per run (`config.ParseDetectionMode`). Until 2026-11, `exif` compared the resolved date (incl. filename/path/mtime). It now requires real EXIF, see [ADR-0012](../decisions/0012-duplicate-checks-require-real-exif.md).

### ORG-012 Duplicate actions
**Status:** implemented

For a detected duplicate, the organizer MUST apply `duplicates.action.method`:

| Method | Effect |
|---|---|
| `replace` | Overwrite the existing target file. Only allowed with a `detectionMode` that compares `filedate` or `exif` (CFG-002) |
| `addSuffix` | Write as `<name>-<4 random [a-z0-9]>.<ext>`, e.g. `photo-a3f9.jpg`, using a name that is still free |
| `moveToFolder` | Put the file into `action.duplicateFolder` (required), without overwriting a file that is already there (ORG-020) |
| `skip` | Do not transfer, log the duplicate |

**Code:** `internal/organizer/organizer.go`, `internal/fileops/fileops.go`
**Tests:** `internal/organizer/organizer_test.go#TestOrganize_DuplicateReplace`, `internal/organizer/organizer_test.go#TestOrganize_DuplicateAddSuffix`, `internal/organizer/organizer_test.go#TestOrganize_DuplicateMoveToFolder`, `internal/organizer/organizer_test.go#TestOrganize_DuplicateSkip`, `internal/fileops/fileops_test.go#TestGenerateSuffix`, `internal/fileops/fileops_test.go#TestAddSuffix`, `internal/organizer/safety_test.go#TestOrganize_MoveToFolderNeverOverwrites`, `internal/fileops/noclobber_test.go#TestReplace_OverwritesExistingFile`, `internal/config/detection_test.go#TestValidateReplaceNeedsMoreThanTheFilename`

- **WHEN** `detectionMode` is `filename+filedate`, the method is `replace`, and a file with the same name and mtime arrives
  **THEN** it replaces the target file
- **WHEN** the same configuration sees a file with the same name but another mtime
  **THEN** the target stays and the new file gets a suffix (ORG-013)
- **WHEN** `detectionMode` is `filename` (or `filename|…`) and the method is `replace`
  **THEN** validation fails, because the organizer only checks files whose name is already taken at the target, so every different photo with the same name would be overwritten

**Notes:** The validation rule for `replace` was added in 2026-11. Before, the README warned about `filename` + `replace` as a known limit.

### ORG-013 Name collisions that are not duplicates
**Status:** implemented

- **WHEN** the target path exists, `duplicates` is configured, and the file is *not* a duplicate per `detectionMode`
  **THEN** the file is written with a random suffix (as in `addSuffix`), so nothing is overwritten
- **WHEN** the target path exists and no `duplicates` block is configured
  **THEN** the file is skipped and logged

**Code:** `internal/organizer/organizer.go`
**Tests:** `internal/organizer/organizer_test.go#TestOrganize_NoDuplicateConfig_ExistingTarget`, `internal/organizer/organizer_test.go#TestOrganize_AndDetection_NotMatching`, `internal/organizer/exif_detection_test.go#TestOrganize_ExifDetection_NoEXIFNeverMatches`
**Notes:** This requirement was not in openspec. It documents the existing behavior.

### ORG-014 Dry-run
**Status:** implemented

With `dryRun: true`, the organizer MUST log every planned action (transfer, fallback, duplicate handling) and MUST NOT change any file, including the state file.

- **WHEN** `dryRun` is `true`
  **THEN** source and target directories are unchanged and the planned actions are logged with a `[DRY-RUN]` prefix

**Code:** `internal/organizer/organizer.go`
**Tests:** `internal/organizer/organizer_test.go#TestOrganize_DryRun`, `internal/organizer/organizer_test.go#TestOrganize_DuplicateDryRun`, `internal/organizer/organizer_test.go#TestHandleNoDate_WithFallback_DryRun`, `internal/organizer/safety_test.go#TestOrganize_DryRunNeverSavesState`

### ORG-015 State persistence and resume
**Status:** implemented

The organizer MUST store which files it has processed (keyed by path, size and mtime) in a JSON state file per profile target and source under `stateDir`. On later runs it MUST skip files that are unchanged. A missing or corrupt state file MUST lead to a full re-scan, not a failure; a corrupt one with a warning. A state file that exists but cannot be read (e.g. it belongs to another user) MUST NOT lead to a re-scan, because every file would count as new: the run MUST NOT transfer anything, log an error and count one error (ORG-017, WEB-011, ntfy summary).

The state MUST be saved at the end of every run and also during the run, after every 200 processed files, so a crash or power cut loses at most that much progress. A dry run MUST never save the state (ORG-014).

- **WHEN** the daemon restarts after processing 50 of 100 files
  **THEN** the next run skips those 50 and processes the rest
- **WHEN** a run is killed after 1,000 of 5,000 files
  **THEN** the state file already lists those 1,000 files
- **WHEN** the state file is corrupt
  **THEN** a warning is logged and the run starts fresh
- **WHEN** the state file of a `copy` source belongs to another user and cannot be read
  **THEN** the run copies nothing and reports 1 error

**Code:** `internal/state/state.go`, `internal/organizer/organizer.go`, `cmd/phorga/main.go`
**Tests:** `internal/organizer/organizer_test.go#TestOrganize_Resume`, `internal/organizer/organizer_test.go#TestStatePath`, `internal/state/state_test.go#TestMarkAndCheck`, `internal/state/state_test.go#TestSaveAndLoad`, `internal/state/state_test.go#TestLoadMissing`, `internal/state/state_test.go#TestLoadCorrupt`, `internal/state/state_test.go#TestLoadUnreadableIsAnError`, `cmd/phorga/access_test.go#TestOrganizerJob_UnreadableStateTransfersNothing`, `internal/state/state_test.go#TestSaveAtomicity`, `internal/organizer/safety_test.go#TestOrganize_SavesStateDuringRun`, `internal/organizer/safety_test.go#TestOrganize_DryRunNeverSavesState`, `internal/organizer/safety_test.go#TestHandleNoDate_CopyIsRemembered`
**Notes:** Without the saves during a run, a crash in `copy` mode made the next run process the same files again. Files without EXIF then failed an `exif` duplicate check and got a second copy with a suffix.

### ORG-016 Supported file formats
**Status:** implemented

The system MUST process files with these extensions (case-insensitive): `jpg jpeg png heic raw cr2 nef arw mp4 mov`. Other files MUST be skipped silently.

- **WHEN** a source contains `notes.txt`
  **THEN** it is ignored and not counted as an error

**Code:** `internal/metadata/metadata.go`
**Tests:** `internal/metadata/metadata_test.go#TestIsSupported`, `internal/organizer/organizer_test.go#TestOrganize_SkipUnsupported`

### ORG-017 Per-file errors do not abort a run
**Status:** implemented

A failure on a single file (unreadable, transfer error, …) MUST be logged and counted in the run result. The run MUST continue with the next file.

- **WHEN** the target directory for one file cannot be created
  **THEN** that file counts as an error and the other files are still organized

**Code:** `internal/organizer/organizer.go`
**Tests:** `internal/organizer/errors_test.go#TestOrganize_FileErrorDoesNotAbortRun`

### ORG-018 Transfers keep the modification time
**Status:** implemented

A copied or moved file MUST have the same modification time (mtime) as its source. If the mtime cannot be set, the transfer MUST fail and the source MUST stay in place.

- **WHEN** a video with mtime `2019-07-15 14:30` and no other date is moved
  **THEN** the file in the target has mtime `2019-07-15 14:30`

**Code:** `internal/fileops/fileops.go`
**Tests:** `internal/fileops/safety_test.go#TestAtomicCopy_KeepsModTime`, `internal/fileops/safety_test.go#TestAtomicMove_KeepsModTime`, `internal/organizer/safety_test.go#TestOrganize_KeepsModTime`
**Notes:** For files without readable EXIF (HEIC, videos, screenshots), the mtime is often the only date. Before this requirement, a transfer set it to the time of the run, and `move` lost the original for good. It also means the `filedate` check (ORG-011) can now match a file that Phorga transferred itself.

### ORG-019 Verified, durable transfers
**Status:** implemented

Before a transferred file becomes visible under its final name, the system MUST flush it to disk and MUST check that it has the same size as the source. If the sizes differ, the transfer MUST fail, the temp file MUST be removed, and in `move` mode the source MUST NOT be deleted.

- **WHEN** a copy stops halfway without reporting an error
  **THEN** the transfer fails with `incomplete copy`, the source is untouched and nothing is left in the target directory

**Code:** `internal/fileops/fileops.go`
**Tests:** `internal/fileops/safety_test.go#TestAtomicMove_IncompleteCopyKeepsSource`
**Notes:** Extends ORG-004, see [ADR-0006](../decisions/0006-atomic-file-operations.md). The flush costs some speed per file. It cannot be tested against a real power cut; the test covers the size check.

### ORG-020 No silent overwrite
**Status:** implemented

The organizer MUST NOT overwrite an existing file, except with the duplicate action `replace` (ORG-012). When the destination in `fallbackFolder` (ORG-003) or in `action.duplicateFolder` (ORG-012) already exists, the file MUST be written with a random suffix, as in `addSuffix`.

This MUST also hold when a file appears at the destination after Phorga checked it, e.g. written by another program. The finished temp file therefore gets its final name through a hard link, which fails if the name is taken. The transfer then fails with `destination already exists` (ORG-017), the existing file and the source stay untouched, and the next run handles the file again. On filesystems without hard links (e.g. exFAT, some network mounts), Phorga falls back to a check followed by a rename, which leaves a short window ([ADR-0019](../decisions/0019-no-clobber-transfers-with-hard-links.md)).

- **WHEN** three different photos named `IMG_0001.jpg` arrive one after another and the action is `moveToFolder`
  **THEN** the first is organized and the duplicate folder holds the other two
- **WHEN** two files named `nodate.jpg` are moved to `fallbackFolder`
  **THEN** both are in `fallbackFolder` afterwards
- **WHEN** another program creates `2024/03/a.jpg` while Phorga copies a different `a.jpg` there
  **THEN** the transfer fails, the other program's file is unchanged, and no temp file is left

**Code:** `internal/fileops/fileops.go`, `internal/organizer/organizer.go`
**Tests:** `internal/fileops/safety_test.go#TestUniquePath`, `internal/fileops/noclobber_test.go#TestAtomicCopy_NeverReplacesExistingFile`, `internal/fileops/noclobber_test.go#TestAtomicCopy_FileThatAppearsDuringCopyIsKept`, `internal/fileops/noclobber_test.go#TestAtomicMove_NeverReplacesExistingFile`, `internal/fileops/noclobber_test.go#TestAtomicCopy_WithoutHardLinksStillWorks`, `internal/organizer/safety_test.go#TestOrganize_MoveToFolderNeverOverwrites`, `internal/organizer/safety_test.go#TestHandleNoDate_FallbackNeverOverwrites`
**Notes:** Before this requirement, both paths replaced the earlier file, and in `move` mode that file was lost. Duplicate detection has the same rule in DUP-005. Until 2026-11, the final rename replaced a file that appeared after the check; Phorga's own jobs could cause that by running at the same time, see RUN-002.

### ORG-021 Stopping a run on shutdown
**Status:** implemented

When Phorga shuts down (RUN-001), a running organizer run MUST stop before it starts the next file. The file in progress is finished, so no file is left half-transferred. The run MUST then save its state as at the end of a normal run (ORG-015, never in a dry run) and report that it stopped early.

- **WHEN** Phorga is told to stop after the first of 5 files was moved
  **THEN** the run ends, 4 files stay untouched in the source, and the state lists the 1 moved file
- **WHEN** the stop request comes before the first file
  **THEN** no file is changed

**Code:** `internal/organizer/organizer.go`
**Tests:** `internal/organizer/cancel_test.go#TestOrganize_StopsWhenCancelled`, `internal/organizer/cancel_test.go#TestOrganize_CancelledBeforeStartChangesNothing`

### ORG-022 System and hidden folders are always skipped
**Status:** implemented

While walking a source, the system MUST skip these entries and everything below them, without opening, counting or logging each file:

- folders named exactly `@eaDir` (Synology thumbnails and index data), `#recycle` (share recycle bin) and `#snapshot` (Btrfs snapshots), as DSM writes them
- every file or folder whose name starts with `.` (e.g. `.thumbnails`, `.Trash-1000`, macOS `._IMG_1234.jpg`)

The rule applies only to entries **below** the source path. A source path that itself is or contains such a name (e.g. `/photos/.import`) is still walked. The list is fixed and cannot be turned off. Skipped entries are not errors. Each skipped folder MUST be logged once at info level (`skipping folder`, with its path). The count for the progress display (WEB-014) skips the same entries, so they never appear in "x of y".

- **WHEN** a source holds `IMG_1234.JPG` and `@eaDir/IMG_1234.JPG/SYNO_PHOTO_THUMB_SM.jpg`
  **THEN** only `IMG_1234.JPG` is organized, and the run reports 1 file and 0 errors
- **WHEN** a source holds `._IMG_1234.jpg` next to `IMG_1234.jpg`
  **THEN** only `IMG_1234.jpg` is organized
- **WHEN** the source path is `/photos/.import` and holds `a.jpg`
  **THEN** `a.jpg` is organized

**Code:** `internal/exclude/exclude.go`, `internal/organizer/organizer.go`, `internal/metadata/metadata.go`
**Tests:** `internal/exclude/exclude_test.go#TestSkip_FixedEntries`, `internal/exclude/exclude_test.go#TestSkip_RootIsNeverSkipped`, `internal/organizer/exclude_test.go#TestOrganize_SkipsSystemAndHiddenEntries`, `internal/organizer/exclude_test.go#TestOrganize_HiddenSourceRootIsWalked`, `internal/metadata/count_test.go#TestCountSupported_SkipsExcludedEntries`
**Notes:** Found in a dry run on a Synology: `@eaDir` thumbnails were organized like photos. Duplicate detection uses the same rule (DUP-013). See [ADR-0016](../decisions/0016-fixed-skip-list-and-simple-exclude-patterns.md). The log line is not covered by a test.

### ORG-023 Exclude list per source
**Status:** implemented

`organizer[].sources[].exclude` is an optional list of patterns (CFG-007). Entries below the source that match a pattern MUST be skipped like the fixed list (ORG-022): not opened, not counted, not an error, one log line per skipped folder. A matching folder is skipped with everything below it.

Matching rules:

- A pattern **without** `/` matches the **name** of any file or folder at any depth, e.g. `Screenshots`, `*.tmp`, `*_edited.jpg`.
- A pattern **with** `/` matches the **path relative to the source root**, e.g. `2019/Scans` or `*/WhatsApp`. A leading `/` is not allowed; a trailing `/` matches folders only.
- Wildcards follow Go's `path.Match`: `*`, `?`, `[abc]`. `*` never crosses a `/`. There is no `**`.
- Matching ignores case.
- `exclude` adds to the fixed list (ORG-022); it cannot re-include a fixed entry.

- **WHEN** `exclude: [Screenshots]` and the source holds `2024/Screenshots/a.png` and `2024/b.jpg`
  **THEN** only `2024/b.jpg` is organized
- **WHEN** `exclude: ["*_edited.jpg"]` and the source holds `a.jpg` and `a_edited.jpg`
  **THEN** only `a.jpg` is organized
- **WHEN** `exclude: [2019/Scans]` and the source holds `2019/Scans/x.jpg` and `2020/Scans/y.jpg`
  **THEN** only `2020/Scans/y.jpg` is organized
- **WHEN** `exclude: [screenshots]` and the source holds `Screenshots/a.png`
  **THEN** `a.png` is skipped

**Code:** `internal/exclude/exclude.go`, `internal/organizer/organizer.go`, `internal/metadata/metadata.go`
**Tests:** `internal/exclude/exclude_test.go#TestSkip_Patterns`, `internal/organizer/exclude_test.go#TestOrganize_ExcludePatterns`, `internal/metadata/count_test.go#TestCountSupported_SkipsExcludedEntries`
**Notes:** Decided 2026-11-15: per source and per job (no global list), the simple name-or-path syntax, case ignored, one info line per skipped folder, see [ADR-0016](../decisions/0016-fixed-skip-list-and-simple-exclude-patterns.md). A `target`, `fallbackFolder` or `duplicateFolder` inside a source is not skipped automatically; that is left for a separate change.

### ORG-024 Files that are already in place
**Status:** implemented

When the target path of a file is the file itself (same file on disk, e.g. because the target is inside the source), the organizer MUST leave it alone: no duplicate check, no action, counted as skipped, and recorded in the state (not in a dry run). No transfer function may ever copy or move a file onto itself.

- **WHEN** source and target are both `/photos`, `/photos/2019/07/IMG_0001.jpg` has an mtime in July 2019, the mode is `move` and the duplicate action is `replace`, `addSuffix`, `moveToFolder` or `skip`
  **THEN** every run reports 1 skipped file, and `/photos/2019/07/IMG_0001.jpg` is the only file, unchanged

**Code:** `internal/organizer/organizer.go`, `internal/fileops/fileops.go`
**Tests:** `internal/organizer/dates_test.go#TestOrganize_FileAlreadyInPlaceStaysUntouched`, `internal/fileops/noclobber_test.go#TestTransferOntoItselfIsRefused`
**Notes:** Until 2026-11, `move` with `replace` deleted such a file (it was copied onto itself and then removed as the source), and `addSuffix` renamed it with a new suffix on every run. Skipping a `target` inside a source completely is still open (ORG-023 note).

### ORG-025 Moves within one volume keep the file
**Status:** implemented

In `move` mode, when source and destination are on the same volume, the file MUST get its new name without copying its data: a hard link at the destination, then the source name is removed. The moved file is the same file (inode), so its content, permissions and modification time are unchanged (ORG-018) and no size check is needed (ORG-019). When that is not possible (another volume, which includes separate Docker bind mounts, or a filesystem without hard links), the move MUST fall back to the verified copy of ORG-004 and ORG-019. With `replace`, a rename is used instead of the hard link. A source that is a symbolic link MUST NOT take this shortcut, because the hard link or rename would move the link itself, which may point nowhere from its new folder: its content is copied as in ORG-004 and ORG-019, and then only the link is removed. The file it points to stays untouched.

- **WHEN** a 4 GB video is moved from `/volume1/photos/incoming` to `/volume1/photos/organized` on the same volume and mount
  **THEN** no data is copied and the file is the same inode afterwards
- **WHEN** the target is on another volume
  **THEN** the file is copied, verified, and only then removed from the source
- **WHEN** the source holds `IMG_1.jpg`, a relative symbolic link to `../shared/IMG_1.jpg`, and it is moved to `2024/05/`
  **THEN** `2024/05/IMG_1.jpg` is a regular file with the photo's content, the link is gone, and `shared/IMG_1.jpg` is unchanged

**Code:** `internal/fileops/fileops.go`
**Tests:** `internal/fileops/noclobber_test.go#TestAtomicMove_SameVolumeKeepsTheFileWithoutCopying`, `internal/fileops/noclobber_test.go#TestAtomicMove_OtherVolumeCopiesAndRemovesSource`, `internal/fileops/noclobber_test.go#TestReplace_OverwritesExistingFile`, `internal/fileops/noclobber_test.go#TestAtomicMove_SymlinkIsMovedAsItsContent`
**Notes:** In Docker, source and target are only "the same volume" when they come from the same bind mount, e.g. mount `/volume1/photos` once instead of `incoming` and `organized` separately. See [ADR-0019](../decisions/0019-no-clobber-transfers-with-hard-links.md).
