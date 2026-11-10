# ADR-0003: Date resolution EXIF → filename → path → mtime

- **Status:** accepted
- **Date:** 2026-05-17 (Phorga v1)
- **Specs:** ORG-002, ORG-003

## Context
Many files have no EXIF date: screenshots, messenger images, videos, and exported or re-saved images. Their mtime is often the copy date, not the capture date. Phone and messenger filenames (`IMG_20240715_…`, `IMG-20240715-WA0001`) and existing folder layouts (`/2019/07/`) often carry a better date.

## Decision
`metadata.ExtractDate` tries EXIF `DateTimeOriginal` (via `rwcarlsen/goexif`, pure Go, no CGO), then filename patterns, then date patterns in the directory path, and finally mtime. It returns the method that succeeded.

## Consequences
- Most files end up in a plausible folder, and only files that cannot be stat'ed go to `fallbackFolder`.
- Anything that compares "EXIF dates" (organizer `exif` detection mode, dedup `metadata` mode) actually compares this resolved date. That is a known open question in the specs.
- Filename patterns can misfire on arbitrary numbers. Patterns are kept strict (valid month/day ranges).

## Alternatives considered
The first v1 design used EXIF → mtime only. That was extended during v1 implementation because mtime is unreliable after copying. `dsoprea/go-exif` is more feature-rich, but heavier, and was not needed.
