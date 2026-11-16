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

With `detectionMode: content`, the system MUST compute a perceptual hash (pHash) per image. Two images with a Hamming distance of **≤ 5** are duplicate *candidates*; they count as duplicates only when the detail check (DUP-014) confirms them. The threshold is fixed and not configurable. Files that cannot be decoded as images (e.g. videos, HEIC, RAW) are skipped for content comparison.

- **WHEN** two images are visually identical but have different names
  **THEN** they are duplicates
- **WHEN** an image and a resized, re-compressed copy of it are scanned
  **THEN** they are duplicates
- **WHEN** two images differ visibly (distance > 5)
  **THEN** they are not duplicates

**Code:** `internal/dedup/dedup.go`
**Tests:** `internal/dedup/dedup_test.go#TestDedup_ContentMode_IdenticalImages`, `internal/dedup/dedup_test.go#TestDedup_ContentMode_DifferentImages`, `internal/dedup/detail_test.go#TestDedup_ContentMode_ResizedRecompressedCopyIsADuplicate`
**Notes:** The old spec said "≤ 8". The code (`maxHammingDistance = 5`) and the arc42 glossary say 5, so the spec now matches the code. See [ADR-0004](../decisions/0004-perceptual-hash-for-content-dedup.md). Until 2026-11, a pHash match alone was enough, so burst shots counted as duplicates, see DUP-014.

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

With `dryRun: true`, the system MUST log every detected pair and the action it would take, and MUST NOT modify, move or delete any photo or video. The only file a dry run may write is the fingerprint cache (DUP-012).

**Code:** `internal/dedup/dedup.go`
**Tests:** `internal/dedup/dedup_test.go#TestDedup_DryRunNoChanges`, `internal/dedup/cache_test.go#TestCache_DryRunWritesCacheButNoPhoto`
**Notes:** Writing the cache in dry runs is a deliberate exception, see [ADR-0015](../decisions/0015-dedup-fingerprint-cache-written-in-dry-runs.md).

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

### DUP-012 Fingerprint cache
**Status:** implemented

The system MUST remember what a scan computed for each file, so later runs do not read unchanged files again:

- The cache is one file, `<stateDir>/dedup-cache.json`, shared by all duplicate detection jobs. It is separate from the organizer's state files.
- An entry is keyed by path and is valid only while the file's size and modification time are unchanged. A changed file MUST be computed again.
- `content` mode stores the perceptual hash, or the fact that the file cannot be decoded (e.g. videos, HEIC), and the detail fingerprint of files that were a pHash candidate (DUP-014). `metadata` mode stores the EXIF date, or the fact that there is none. Read errors MUST NOT be cached.
- The cache MUST be saved atomically every 500 newly computed files and at the end of each scan, also when the run was stopped (DUP-010), and also in dry runs (DUP-007).
- After a complete scan, entries below the job's own sources whose files are gone MUST be removed. Entries of other folders MUST be kept.
- Jobs running at the same time MUST NOT lose each other's entries.
- A missing or corrupt cache file MUST lead to a warning and a full scan, not a failure. A failed write MUST be logged and never fail the job.

- **WHEN** a library was scanned once and nothing changed
  **THEN** the next run reads no image and finds the same duplicates
- **WHEN** one file's modification time changed
  **THEN** only that file is computed again
- **WHEN** a dry run scans 3 images
  **THEN** `dedup-cache.json` holds 3 entries and no photo was moved
- **WHEN** a run is stopped after 2 of 3 files
  **THEN** the saved cache holds those 2 entries
- **WHEN** a file of job A is deleted and job A runs again
  **THEN** its entry is removed, and the entries of job B's folders stay

**Code:** `internal/dedup/cache.go`, `internal/dedup/dedup.go`, `cmd/phorga/main.go`
**Tests:** `internal/dedup/cache_test.go#TestCache_SecondRunUsesCache`, `internal/dedup/detail_test.go#TestCache_DetailFingerprintsAreCached`, `internal/dedup/cache_test.go#TestCache_MetadataModeUsesCache`, `internal/dedup/cache_test.go#TestCache_ChangedFileIsRecomputed`, `internal/dedup/cache_test.go#TestCache_PersistsAndReloads`, `internal/dedup/cache_test.go#TestCache_DryRunWritesCacheButNoPhoto`, `internal/dedup/cache_test.go#TestCache_RemembersUndecodableFiles`, `internal/dedup/cache_test.go#TestCache_PruneKeepsOtherJobsEntries`, `internal/dedup/cache_test.go#TestCache_CorruptFileStartsFresh`, `internal/dedup/cache_test.go#TestCache_InterruptedRunKeepsComputedFingerprints`, `internal/dedup/cache_test.go#TestCache_SharedByConcurrentJobs`
**Notes:** See [ADR-0015](../decisions/0015-dedup-fingerprint-cache-written-in-dry-runs.md). Deleting `dedup-cache.json` forces a full scan, e.g. after editing photos with a tool that keeps the modification time.

### DUP-013 Skipped folders and exclude list
**Status:** implemented

The scan MUST skip the fixed system and hidden entries of ORG-022, and the entries matching the job's optional `duplicateDetection[].exclude` list, with the matching rules of ORG-023. A relative pattern matches against the path below each of the job's sources. Skipped files are not scanned, not compared, not counted and never acted on. After a complete scan, their cache entries are removed like those of vanished files (DUP-012).

- **WHEN** a source holds `IMG_1234.JPG` and its thumbnail `@eaDir/IMG_1234.JPG/SYNO_PHOTO_THUMB_SM.jpg`, the mode is `content` and the method is `moveToFolder`
  **THEN** no pair is reported and `IMG_1234.JPG` stays in place
- **WHEN** an earlier scan cached the thumbnail and the job runs again
  **THEN** the thumbnail's cache entry is removed

**Code:** `internal/dedup/dedup.go`, `internal/exclude/exclude.go`, `internal/metadata/metadata.go`
**Tests:** `internal/dedup/exclude_test.go#TestDedup_SynologyThumbnailIsNotADuplicate`, `internal/dedup/exclude_test.go#TestDedup_ExcludePatternsSkipFiles`, `internal/dedup/exclude_test.go#TestDedup_ExcludedFilesLeaveTheCache`
**Notes:** Without this, a thumbnail is found before its original (`@` sorts before letters), becomes the kept file, and the real photo is moved as the duplicate (DUP-004). See [ADR-0016](../decisions/0016-fixed-skip-list-and-simple-exclude-patterns.md). The cache test removes the entry through an `exclude` pattern; the fixed list uses the same pruning path.

### DUP-014 Detail check for content candidates
**Status:** implemented

In `content` mode, every pair that passes the pHash stage (DUP-003) MUST pass a detail check before it counts as a duplicate:

- Each candidate image is scaled to a 64×64 grayscale thumbnail (its detail fingerprint).
- The thumbnail is split into 16×16 tiles. For each tile, the system counts the neighbouring pixel pairs whose brightness step goes in a different direction in the two images. Pairs that are nearly flat in both images are ignored.
- The pair is a duplicate only when no tile has more than **12 %** flipped steps. The threshold is fixed and not configurable.
- Re-encoded, resized and brightness- or gamma-edited copies MUST still count as duplicates. Burst shots in which a part of the picture changed (a face, closed eyes, a person who moved) MUST NOT.
- A candidate whose fingerprint cannot be computed is logged, counted as an error, and never a duplicate.
- The check reads only candidate images; their fingerprints are cached (DUP-012). The run checks for shutdown before each image it reads (DUP-010).

- **WHEN** two burst shots have a pHash distance ≤ 5 but differ in one small area
  **THEN** they are not duplicates
- **WHEN** a copy of an image was made brighter (gamma 0.8)
  **THEN** it is a duplicate of the original
- **WHEN** a library with candidates was scanned once and nothing changed
  **THEN** the next run computes no detail fingerprint

**Code:** `internal/dedup/verify.go`, `internal/dedup/dedup.go`, `internal/dedup/cache.go`
**Tests:** `internal/dedup/detail_test.go#TestDedup_ContentMode_BurstShotWithLocalChangeIsNotADuplicate`, `internal/dedup/detail_test.go#TestDedup_ContentMode_BrightnessEditedCopyIsADuplicate`, `internal/dedup/detail_test.go#TestDedup_ContentMode_ResizedRecompressedCopyIsADuplicate`, `internal/dedup/detail_test.go#TestCache_DetailFingerprintsAreCached`, `internal/dedup/detail_test.go#TestDetailDiff_TileThatDiffersDecides`
**Notes:** pHash alone sees only the coarse layout, so burst shots of the same scene reached distance 0–4. Measured on real photos: copies ≤ 0.07, burst shots ≥ 0.22. See [ADR-0017](../decisions/0017-detail-check-after-phash.md). Very small changes (a few pixels in a large group photo) can stay below the threshold.
