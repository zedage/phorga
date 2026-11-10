# ADR-0012: Duplicate checks on dates require real EXIF

- **Status:** accepted
- **Date:** 2026-11-10
- **Specs:** ORG-011, DUP-002

## Context
The organizer detection mode `exif` and the dedup mode `metadata` (`fileSize` + capture date) used `metadata.ExtractDate`. That function falls back to the filename, the path and finally mtime ([ADR-0003](0003-date-resolution-order.md)). As a result, two files without EXIF but with the same size and the same copy time, or with the same date in their name, were reported as duplicates. This contradicts the names of the modes and leads to false positives, especially after bulk copies, where many files share an mtime.

## Decision
Both modes compare only the real EXIF capture date (`metadata.ExtractEXIFDate`: `DateTimeOriginal`, falling back to the EXIF `DateTime`). Files without EXIF never match in these modes. The date resolution for **sorting** files (ADR-0003) is unchanged.

## Consequences
- Fewer false positives. The modes do what their names say.
- Files without EXIF (screenshots, many videos, messenger images) are no longer detected by these modes. Use `filename`/`filedate` (organizer) or `content` (dedup) for them.
- This is a behavior change for existing configs. Users who relied on mtime matching will see fewer duplicates.

## Alternatives considered
Keeping the resolved date was rejected because of the false positives. A separate mode such as `resolvedDate` was not added, since nobody needs it yet.
