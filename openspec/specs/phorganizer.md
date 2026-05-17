# Phorganizer - The other photo organizer
Phorganizer is a photo organizer running in the background, which automatically organizes photos in folders by year and month by their metadata.

## Features
Phorganizer runs as a daemon in the background and wakes up automatically according to the configuration.

### Organization
- Automatically organize photos in folders by year and month by their metadata.
- Support for various photo and video formats, including JPEG, PNG, and RAW or video formats like MP4 and MOV.
- Configurable settings for organizing photos, such as the ability to exclude certain folders or file types.
- As configuration, you can specify a target directory where the organized photos will be stored, and several source directories where the photos to be organized are located.
- Several source directories per target are supported.
- Several target directories are supported, so that photos can be organized into different locations based on their source.
- Support for multiple platforms, including Windows, macOS, Linux, and Synology NAS.
- The tool has no user interface, it runs in the background and can be configured through a yaml or json configuration file.
- The tool runs as a docker container, making it easy to deploy and run on various platforms.
- The tool can be scheduled to run at specific intervals per source, such as daily or weekly, to ensure that photos are organized regularly.
- The tool can be configured to move or copy photos, depending on the user's preference.
- The tool can run in dry mode, which allows users to see what actions would be taken without actually moving or copying any files, which can help to prevent accidental data loss.
- The tool remembers the last organized photo, so that it can continue organizing from where it left off in case of interruptions or restarts.
- The organizer includes only lightweight duplicate handling (filename/filedate/exif) during organize operations.

#### Organizer MUST/SHOULD
- The Organizer feature MUST focus on organizing files into the configured target structure.
- The Organizer feature MUST support `move` and `copy` modes per source.
- The Organizer feature MUST support `dryRun` without changing files.
- The Organizer feature MUST support per-source scheduling.
- The Organizer feature MUST support lightweight duplicate checks using only `filename`, `filedate`, and `exif`.
- The Organizer feature MUST NOT perform advanced content-based duplicate detection.
- Lightweight duplicate handling SHOULD only apply within the organizer run scope and its configured action.
- The Organizer configuration MUST remain independent from advanced Duplicate Detection configuration.

#### Configuration
- The configuration file can be in yaml or json format, and it allows users to specify various settings for organizing photos, such as the target directory, source directories, and lightweight duplicate handling.
```yaml
organizer:
  - dryRun: true # if true, the tool will only print the actions it would take without actually moving or copying any files
    ntfy:
      url: https://ntfy.sh/your-topic # optional, if you want to receive notifications about the status of the tool
    target: /path/to/organized/photos
    targetStructure: "{year}/{month}" # {year} 4-digit, {month} 2-digit, {monthWithName} 01-january, {day} 2-digit day of month (01-31)
    fallbackFolder: /path/to/fallback # optional, if metadata is missing or invalid
    sources:
      - path: /path/to/photos1
        mode: move # move or copy
        duplicates:
          detectionMode: filename # filename, filedate, exif; '+' or '|' allowed, but not mixed in one expression
          action:
            method: replace # replace, addSuffix, moveToFolder, skip
            duplicateFolder: /path/to/duplicates # required if method is moveToFolder
        schedule: 0 2 * * *
```
 
### Duplicate Detection
- Duplicate Detection is a separate feature from the Organizer.
- It scans configured source directories for duplicate photos and videos.
- Detection can be based on metadata (`fileSize` + `exifDateTimeOriginal`) or content (perceptual hash / content hash).
- Duplicates are detected across all configured sources (cross-source matching).
- Detected duplicates can be moved to a separate folder or skipped.
- It can run in dry mode to preview actions without modifying any files.
- It runs on its own schedule, independent of the Organizer.

#### Duplicate Detection MUST/SHOULD (V1)
- Duplicate Detection MUST be configured and executed independently of Organizer.
- Duplicate Detection MUST define its own `sources`, `schedule`, `detectionMode`, and `action`.
- Duplicate Detection MUST NOT reuse Organizer duplicate settings implicitly.
- Duplicate Detection MUST support `dryRun` without modifying any files.
- Duplicate Detection MUST support `detectionMode: metadata` using `fileSize` + `exifDateTimeOriginal` as a fixed heuristic.
- Duplicate Detection MUST support `detectionMode: content` using a fixed content-hash approach (no tuning parameters in V1).
- Duplicate Detection MUST match duplicates across all configured sources.
- Duplicate Detection MUST support `action.method: moveToFolder` with a required `duplicateFolder`.
- Duplicate Detection MUST support `action.method: skip` (log only, no file changes).
- Duplicate Detection SHOULD send a summary notification via ntfy when configured.
- A failure on one file MUST NOT abort the entire detection run.

#### Configuration
```yaml
duplicateDetection:
  - enabled: true
    dryRun: true
    schedule: "0 4 * * *"
    sources:
      - /path/to/photos1
      - /path/to/photos2
    detectionMode: content # content | metadata
    action:
      method: moveToFolder # moveToFolder | skip
      duplicateFolder: /path/to/duplicates # required if method=moveToFolder
    ntfy:
      url: https://ntfy.sh/your-topic # optional
```

## Technology
- use Go with libs like goexif or images4 or go-phash for metadata extraction and duplicate detection.
- use ntfy.sh for notifications to send status messages to the user, such as when photos are organized or when duplicates are detected.
