## ADDED Requirements

### Requirement: Date-based photo organization
The system SHALL organize photos and videos from configured source directories into a target directory, structured by date extracted from file metadata.

#### Scenario: Organize photo with valid EXIF date
- **WHEN** a source directory contains a JPEG with EXIF DateTimeOriginal "2024-03-15 14:30:00"
- **THEN** the file SHALL be placed in `<target>/<targetStructure>` resolved to `2024/03`

#### Scenario: Fallback to file modification time
- **WHEN** a photo has no EXIF date but has file modification time "2023-12-01"
- **THEN** the file SHALL be organized using `2023/12` as the date

#### Scenario: No date available with fallbackFolder configured
- **WHEN** a file has no EXIF date and no parseable modification time AND `fallbackFolder` is configured
- **THEN** the file SHALL be moved/copied to the `fallbackFolder`

#### Scenario: No date available without fallbackFolder
- **WHEN** a file has no usable date AND `fallbackFolder` is not configured
- **THEN** the file SHALL be skipped and a warning SHALL be logged

### Requirement: Move and copy modes
The system SHALL support `move` and `copy` modes per source directory.

#### Scenario: Copy mode preserves source
- **WHEN** mode is `copy` and the file is successfully written to the target
- **THEN** the source file SHALL remain unchanged

#### Scenario: Move mode removes source after success
- **WHEN** mode is `move` and the file is successfully written to the target
- **THEN** the source file SHALL be deleted only after the target write is confirmed

### Requirement: Target structure configuration
The system SHALL support a configurable `targetStructure` with placeholder tokens.

#### Scenario: Default target structure
- **WHEN** `targetStructure` is not specified
- **THEN** the system SHALL use `{year}/{month}` as the default

#### Scenario: Custom target structure with day
- **WHEN** `targetStructure` is `{year}/{monthWithName}/{day}`
- **THEN** a photo from 2024-01-15 SHALL be placed in `2024/01-january/15`

#### Scenario: Invalid token in target structure
- **WHEN** `targetStructure` contains an unknown token like `{invalid}`
- **THEN** configuration validation SHALL fail at startup

### Requirement: Per-source scheduling
The system SHALL execute each source on its own cron schedule, independently of other sources.

#### Scenario: Two sources with different schedules
- **WHEN** source A has schedule `0 2 * * *` and source B has schedule `0 4 * * *`
- **THEN** source A SHALL run at 02:00 and source B SHALL run at 04:00 independently

#### Scenario: Overlapping run prevention
- **WHEN** a source run is still in progress and its schedule triggers again
- **THEN** the new run SHALL be skipped and a warning SHALL be logged

### Requirement: Lightweight duplicate handling during organization
The system SHALL detect duplicates during organization using lightweight methods only (`filename`, `filedate`, `exif`). Content-based detection is NOT supported in the organizer.

#### Scenario: Duplicate detected by filename
- **WHEN** `detectionMode` is `filename` and a file with the same name already exists in the target
- **THEN** the configured `action.method` SHALL be applied

#### Scenario: Duplicate action replace
- **WHEN** `action.method` is `replace`
- **THEN** the existing file in the target SHALL be overwritten

#### Scenario: Duplicate action addSuffix
- **WHEN** `action.method` is `addSuffix`
- **THEN** a 4-character random suffix SHALL be appended to the filename (e.g., `photo-a3f9.jpg`)

#### Scenario: Duplicate action moveToFolder
- **WHEN** `action.method` is `moveToFolder` and `duplicateFolder` is set
- **THEN** the duplicate SHALL be placed in the configured `duplicateFolder`

#### Scenario: Duplicate action skip
- **WHEN** `action.method` is `skip`
- **THEN** the file SHALL not be copied/moved and a log entry SHALL be written

### Requirement: Dry-run mode
The system SHALL support a `dryRun` mode that simulates all actions without modifying the filesystem.

#### Scenario: Dry-run output
- **WHEN** `dryRun` is `true`
- **THEN** the system SHALL log each planned action (copy/move, source, destination) without writing any files

### Requirement: State persistence and resume
The system SHALL persist processing state to survive restarts and continue from where it left off.

#### Scenario: Resume after restart
- **WHEN** the daemon restarts after processing 50 of 100 files
- **THEN** the system SHALL skip the 50 already-processed files and continue with the remaining 50

#### Scenario: Corrupt state file
- **WHEN** the state file is corrupted or unreadable
- **THEN** the system SHALL log a warning and start a full re-scan of the source

### Requirement: Supported file formats
The system SHALL process common photo and video formats.

#### Scenario: Supported formats
- **WHEN** files with extensions jpg, jpeg, png, heic, raw, cr2, nef, arw, mp4, mov are present
- **THEN** the system SHALL process all of them

#### Scenario: Unsupported format
- **WHEN** a file with an unsupported extension (e.g., `.txt`) is in the source
- **THEN** the file SHALL be skipped silently

## ADDED Requirements (Multilingual Tokens)

### Requirement: Multilingual target structure with language-specific tokens
The system SHALL support language-specific month and day name tokens in `targetStructure` for German, French, and Italian, in addition to English.

#### Scenario: German month name token
- **WHEN** `targetStructure` is `{year}/{Monat}` and a photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/Februar`

#### Scenario: French month name token
- **WHEN** `targetStructure` is `{year}/{Mois}` and a photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/Février`

#### Scenario: Italian day name token
- **WHEN** `targetStructure` is `{year}/{mese}/{giorno}` and a photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/febbraio/15`

#### Scenario: Case-sensitive month name rendering
- **WHEN** `targetStructure` is `{year}/{monat}` (lowercase m) and photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/februar` (all lowercase)

#### Scenario: Case-sensitive month name with capital first letter
- **WHEN** `targetStructure` is `{year}/{Monat}` (capital M) and photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/Februar` (first letter capitalized)

### Requirement: Numeric month and day placeholders
The system SHALL support numeric placeholder tokens that render dates as zero-padded two-digit numbers.

#### Scenario: Month number placeholder
- **WHEN** `targetStructure` is `{year}/{monthNumber}` and photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/02`

#### Scenario: Day number placeholder
- **WHEN** `targetStructure` is `{year}/{monthNumber}/{dayNumber}` and photo date is 2024-02-05
- **THEN** the file SHALL be placed in `2024/02/05`

#### Scenario: Numeric placeholders are always two digits
- **WHEN** processing dates with single-digit day (e.g., 2024-02-05) or single-digit month (if applicable)
- **THEN** the dayNumber and monthNumber SHALL be zero-padded (e.g., `05`)

### Requirement: Language-specific year token support
The system SHALL support language-specific year tokens in addition to English `{year}`.

#### Scenario: German year token
- **WHEN** `targetStructure` is `{jahr}/{monat}` and photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/februar`

#### Scenario: French year token
- **WHEN** `targetStructure` is `{année}/{mois}` and photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/février`

#### Scenario: Italian year token
- **WHEN** `targetStructure` is `{anno}/{mese}` and photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/febbraio`

## MODIFIED Requirements

### Requirement: Target structure configuration (extended)
The system SHALL support a configurable `targetStructure` with placeholder tokens (English and multilingual).

#### Scenario: Default target structure
- **WHEN** `targetStructure` is not specified
- **THEN** the system SHALL use `{year}/{month}` as the default

#### Scenario: Custom target structure with day
- **WHEN** `targetStructure` is `{year}/{monthWithName}/{day}`
- **THEN** a photo from 2024-01-15 SHALL be placed in `2024/01-january/15`

#### Scenario: Multilingual German target structure
- **WHEN** `targetStructure` is `{year}/{Monat}/{Tag}`
- **THEN** a photo from 2024-01-15 SHALL be placed in `2024/Januar/15`

#### Scenario: Mixed token formats
- **WHEN** `targetStructure` is `{year}/{monthNumber}-{Monat}/{dayNumber}`
- **THEN** a photo from 2024-02-05 SHALL be placed in `2024/02-Februar/05`

#### Scenario: Invalid token in target structure
- **WHEN** `targetStructure` contains an unknown token like `{invalid}`
- **THEN** configuration validation SHALL fail at startup

#### Scenario: Unrecognized language token
- **WHEN** `targetStructure` contains a language token from an unsupported language (e.g., `{month_nl}`)
- **THEN** configuration validation SHALL fail at startup with an error indicating unknown token

