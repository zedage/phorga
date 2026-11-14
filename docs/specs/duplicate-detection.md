# Spec: Duplicate Detection (`DUP`)

> Human explanation: [product.md › Duplicate detection](../product.md#duplicate-detection) · Architecture: [arc42 §6 Duplicate Detection Run](../arc42/arc42.adoc) · Decisions: [ADR-0001](../decisions/0001-single-binary-independent-features.md), [ADR-0004](../decisions/0004-perceptual-hash-for-content-dedup.md) · Format: [README](README.md)

Duplicate detection is a standalone feature, separate from the organizer's lightweight duplicate handling (ORG-011). It scans several sources together and acts on duplicates it finds across them. Each `duplicateDetection[]` entry is one job.

---

### DUP-001 Independent of the organizer
**Status:** implemented

A duplicate detection job MUST have its own `sources`, `schedule`, `detectionMode`, `action`, `dryRun` and `ntfy`. It MUST NOT reuse organizer settings and MUST work without any `organizer` block. Jobs with `enabled: false` MUST NOT be scheduled.

- **WHEN** only `duplicateDetection` is configured
  **THEN** the config validates and the job runs on its schedule

**Code:** `internal/dedup/dedup.go`, `internal/config/validate.go`, `cmd/phorga/main.go`
**Tests:** `internal/config/config_test.go#TestValidateDuplicateDetection`

### DUP-002 Metadata mode
**Status:** implemented

With `detectionMode: metadata`, two files MUST count as duplicates when they have the same file size **and** the same **EXIF** capture date (`DateTimeOriginal`). Files without EXIF MUST never match in this mode. Dates from the filename, path or mtime are not used.

- **WHEN** two files have the same size and the same capture date `2024-07-15 10:00:00`
  **THEN** they are duplicates
- **WHEN** two files have the same size but different capture dates
  **THEN** they are not duplicates
- **WHEN** two files have the same size and mtime but no EXIF
  **THEN** they are not duplicates

**Code:** `internal/dedup/dedup.go`
**Tests:** `internal/dedup/dedup_test.go#TestDedup_MetadataMode_SameSize`, `internal/dedup/metadata_mode_test.go#TestDedup_MetadataMode_SameSizeAndEXIFDate`, `internal/dedup/metadata_mode_test.go#TestDedup_MetadataMode_DifferentEXIFDate`, `internal/dedup/metadata_mode_test.go#TestDedup_MetadataMode_NoEXIFNeverMatches`
**Notes:** Until 2026-11, the code used the resolved date (incl. mtime), so files without EXIF could match. It now requires real EXIF, see [ADR-0012](../decisions/0012-duplicate-checks-require-real-exif.md).

### DUP-003 Content mode (perceptual hash)
**Status:** implemented

With `detectionMode: content`, the system MUST compute a perceptual hash (pHash) per image. Two images MUST count as duplicates when the Hamming distance of their hashes is **≤ 5**. The threshold is fixed and not configurable. Files that cannot be decoded as images (e.g. videos, HEIC, RAW) are skipped for content comparison.

- **WHEN** two images are visually identical but have different names
  **THEN** they are duplicates
- **WHEN** two images differ visibly (distance > 5)
  **THEN** they are not duplicates

**Code:** `internal/dedup/dedup.go`
**Tests:** `internal/dedup/dedup_test.go#TestDedup_ContentMode_IdenticalImages`, `internal/dedup/dedup_test.go#TestDedup_ContentMode_DifferentImages`
**Notes:** The old spec said "≤ 8". The code (`maxHammingDistance = 5`) and the arc42 glossary say 5, so the spec now matches the code. See [ADR-0004](../decisions/0004-perceptual-hash-for-content-dedup.md).

### DUP-004 Cross-source matching
**Status:** implemented

Every file of every source in a job MUST be compared with every other file. The file found first (in source order, then walk order) is kept as the original. Each other file MUST be reported as a duplicate **at most once**, against the first earlier non-duplicate file it matches. A group of N matching files therefore yields N-1 pairs that share one original.

- **WHEN** `/photos1/img.jpg` and `/photos2/copy.jpg` are duplicates
  **THEN** the pair is reported and `/photos2/copy.jpg` is treated as the duplicate
- **WHEN** three sources each contain the same image and the method is `moveToFolder`
  **THEN** 2 pairs are reported, 2 files are moved, the first stays in place, and there are 0 errors

**Code:** `internal/dedup/dedup.go`
**Tests:** `internal/dedup/dedup_test.go#TestDedup_CrossSource`, `internal/dedup/actions_test.go#TestDedup_GroupOfThree_MovesTwoWithoutErrors`
**Notes:** Comparison is pairwise O(n²), see arc42 §10. Until 2026-11, groups of 3+ produced every pair (A-B, A-C, B-C), and moving the already-moved file counted as an error. That is now fixed.

### DUP-005 Action `moveToFolder`
**Status:** implemented

With `action.method: moveToFolder`, the duplicate MUST be moved into `action.duplicateFolder`. That field is required for this method (CFG-002). A file that is already in `duplicateFolder` MUST NOT be overwritten; the duplicate is then written with a random suffix (as in ORG-020). The move keeps the modification time and is verified (ORG-018, ORG-019).

- **WHEN** a duplicate is found and the method is `moveToFolder`
  **THEN** the duplicate is moved into `duplicateFolder` and the original stays in place
- **WHEN** `duplicateFolder` already holds a `photo.jpg` and another duplicate named `photo.jpg` is found
  **THEN** both files are in `duplicateFolder` afterwards

**Code:** `internal/dedup/dedup.go`, `internal/config/validate.go`
**Tests:** `internal/dedup/dedup_test.go#TestDedup_MoveToFolder`, `internal/config/config_test.go#TestValidateMoveToFolderRequiresDuplicateFolder`, `internal/dedup/actions_test.go#TestDedup_GroupOfThree_MovesTwoWithoutErrors`, `internal/dedup/actions_test.go#TestDedup_MoveToFolderNeverOverwrites`

### DUP-006 Action `skip`
**Status:** implemented

With `action.method: skip`, the system MUST NOT modify any file. It MUST log each duplicate pair.

**Code:** `internal/dedup/dedup.go`
**Tests:** `internal/dedup/actions_test.go#TestDedup_SkipLeavesFilesUntouched`

### DUP-007 Dry-run
**Status:** implemented

With `dryRun: true`, the system MUST log every detected pair and the action it would take, and MUST NOT modify any file.

**Code:** `internal/dedup/dedup.go`
**Tests:** `internal/dedup/dedup_test.go#TestDedup_DryRunNoChanges`

### DUP-008 Own schedule
**Status:** implemented

Each enabled job MUST run on its own 5-field cron `schedule`, independent of organizer schedules. Overlap prevention from ORG-010 applies as well.

- **WHEN** `schedule` is `0 4 * * *`
  **THEN** the job runs daily at 04:00

**Code:** `cmd/phorga/main.go`, `internal/scheduler/scheduler.go`
**Tests:** `internal/scheduler/scheduler_test.go#TestRegisterAndRun`, `internal/scheduler/scheduler_test.go#TestOverlapPrevention`

### DUP-009 Per-file errors do not abort a run
**Status:** implemented

Errors while scanning a file or applying an action MUST be logged and counted. The run MUST continue.

- **WHEN** `duplicateFolder` is not a directory and two duplicates are found
  **THEN** both moves fail, `errors` is 2, and all files stay in place

**Code:** `internal/dedup/dedup.go`
**Tests:** `internal/dedup/actions_test.go#TestDedup_ActionErrorDoesNotAbortRun`

### DUP-010 Stopping a run on shutdown
**Status:** implemented

When Phorga shuts down (RUN-001), a running duplicate detection run MUST stop before it scans the next file. It MUST NOT compare files or apply any further action after the stop request, and it MUST report that it stopped early.

- **WHEN** Phorga is told to stop while the run is scanning
  **THEN** the scan ends after the current file and no file is moved
- **WHEN** the stop request comes while actions are applied
  **THEN** the action in progress is finished and no further file is moved

**Code:** `internal/dedup/dedup.go`
**Tests:** `internal/dedup/cancel_test.go#TestDedup_CancelledRunMovesNothing`

### DUP-011 Scan progress in the log
**Status:** implemented

While scanning, a duplicate detection run MUST log its progress every 500 scanned files, with the number of files scanned so far and the source folder.

- **WHEN** a source holds 1,200 supported files
  **THEN** the log has a progress line after 500 and after 1,000 files

**Code:** `internal/dedup/dedup.go`
**Tests:** `internal/dedup/cancel_test.go#TestDedup_LogsScanProgress`
**Notes:** Content mode reads every image completely, so a large library takes many minutes. Without progress lines, such a run looked like a hang.
