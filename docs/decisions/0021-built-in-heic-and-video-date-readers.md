# ADR-0021: Built-in date readers for HEIC and MP4/MOV

- **Status:** accepted
- **Date:** 2026-11-21
- **Specs:** ORG-002, ORG-011, DUP-002, DUP-012

## Context
Phorga read capture dates only through `goexif`, which handles JPEG and TIFF-based RAW. iPhones store photos as HEIC and videos as MOV, and Android videos are MP4. For those files, the organizer fell back to the filename, the folder or the modification time, and the mtime is often just the day of the copy to the NAS. So for the most common input of a phone library, the main feature guessed. Duplicate detection in metadata mode never matched these files at all.

## Decision
Phorga reads the dates itself, in pure Go and without new dependencies:

- **HEIC/HEIF:** walk the ISO base media boxes `meta` → `iinf` (find the item of type `Exif`) → `iloc` (find its bytes, in the file or in `idat`), then pass the EXIF block to `goexif`. The date is EXIF `DateTimeOriginal`, the same as for JPEG.
- **MP4/MOV:** read the creation time from `moov/mvhd` (versions 0 and 1). It is seconds since 1904 in UTC and is converted to the local zone (`TZ`), because EXIF dates are local wall-clock times. 0 and implausible values count as no date.

Both readers limit what they read into memory to a few MB, and treat any malformed box as "no date". The duplicate checks treat these dates as "real EXIF" (ADR-0012). The fingerprint cache records which reader version found "no date", so old entries for HEIC and videos are read again once.

## Consequences
- HEIC photos and most videos are now sorted by their capture date, and metadata duplicate detection can match them.
- The readers are about 350 lines Phorga has to maintain. Tests use synthetic files plus one HEIC written by macOS ImageIO, checked manually.
- Videos from cameras that write local time into `mvhd` are shifted by the zone offset. Apple's local creation date with time zone (`com.apple.quicktime.creationdate`) is not read yet.
- Content duplicate detection still cannot decode HEIC images. That needs an HEVC decoder, which does not exist in pure Go.

## Alternatives considered
- **`github.com/evanoberholster/imagemeta` or `dsoprea/go-exif` + `go-heic-exif-extractor`:** cover more formats, but add a large dependency tree for two small box walks, and Phorga must stay without CGO for ARM NAS builds.
- **Calling `exiftool`:** accurate and complete, but needs Perl in the image and a process per file.
- **Leaving it to the filename:** iPhone names (`IMG_4711.HEIC`) contain no date.
