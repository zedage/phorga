## ADDED Requirements

### Requirement: Independent duplicate detection
The system SHALL provide a standalone Duplicate Detection feature that runs independently of the Organizer, with its own configuration, schedule, and sources.

#### Scenario: Duplicate Detection runs without Organizer
- **WHEN** only `duplicateDetection` is configured (no `organizer` block)
- **THEN** the system SHALL run duplicate detection on its own schedule without errors

### Requirement: Metadata-based detection mode
The system SHALL detect duplicates by metadata when `detectionMode` is `metadata`, using `fileSize` + `exifDateTimeOriginal` as a fixed heuristic.

#### Scenario: Same size and EXIF date
- **WHEN** two files across sources have identical file size and identical EXIF DateTimeOriginal
- **THEN** they SHALL be classified as duplicates

#### Scenario: Same size but different EXIF date
- **WHEN** two files have identical size but different EXIF dates
- **THEN** they SHALL NOT be classified as duplicates

### Requirement: Content-based detection mode
The system SHALL detect duplicates by content when `detectionMode` is `content`, using perceptual hashing with a fixed threshold.

#### Scenario: Visually identical images with different filenames
- **WHEN** two images are visually identical (pHash Hamming distance ≤ 8) but have different filenames
- **THEN** they SHALL be classified as duplicates

#### Scenario: Similar but not identical images
- **WHEN** two images have a pHash Hamming distance > 8
- **THEN** they SHALL NOT be classified as duplicates

### Requirement: Cross-source duplicate matching
The system SHALL check for duplicates across all configured sources.

#### Scenario: Duplicate across two sources
- **WHEN** `/photos1/img.jpg` and `/photos2/copy.jpg` are content-identical
- **THEN** they SHALL be detected as duplicates

### Requirement: Duplicate action moveToFolder
The system SHALL move detected duplicates to a configured `duplicateFolder` when `action.method` is `moveToFolder`.

#### Scenario: Move duplicate to folder
- **WHEN** a duplicate is detected and method is `moveToFolder`
- **THEN** the duplicate file SHALL be moved to `duplicateFolder`

#### Scenario: duplicateFolder is required
- **WHEN** `action.method` is `moveToFolder` and `duplicateFolder` is not set
- **THEN** configuration validation SHALL fail at startup

### Requirement: Duplicate action skip
The system SHALL skip duplicates and log them when `action.method` is `skip`.

#### Scenario: Skip duplicate
- **WHEN** a duplicate is detected and method is `skip`
- **THEN** no files SHALL be modified and a log entry SHALL record the duplicate pair

### Requirement: Dry-run mode for duplicate detection
The system SHALL support `dryRun` mode for duplicate detection.

#### Scenario: Dry-run lists duplicates without acting
- **WHEN** `dryRun` is `true`
- **THEN** the system SHALL log all detected duplicate pairs and planned actions without modifying any files

### Requirement: Independent scheduling
Duplicate Detection SHALL run on its own cron schedule.

#### Scenario: Scheduled run
- **WHEN** `schedule` is `0 4 * * *`
- **THEN** duplicate detection SHALL execute at 04:00 daily, independent of any organizer schedule

