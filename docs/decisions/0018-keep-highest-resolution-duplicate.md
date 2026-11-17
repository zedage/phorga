# ADR-0018: Keep the highest-resolution file of a duplicate group

- **Status:** accepted
- **Date:** 2026-11-17
- **Specs:** DUP-004, DUP-012

## Context
Duplicate detection kept the file it found first and moved all others (DUP-004). "First" means source order, then walk order, so in practice it means the file name. A content run on a real library moved 3370 files. In 740 pairs, it moved the camera original and kept a WhatsApp copy, for example `IMG20220920182841.jpg` (3.4 MB) in favor of `IMG-20220920-WA0004.jpg` (211 KB). `IMG-2022…` sorts before `IMG2022…`, and `IMG…` sorts before `PXL…` and `Screenshot…`. WhatsApp copies are scaled down, re-compressed and have no EXIF. ADR-0016 had fixed the same problem for Synology thumbnails, but only by skipping their folders.

## Decision
Each duplicate group keeps the image with the most pixels (width × height, read from the image header). On a tie, it keeps the larger file, and then the file found first. A file whose resolution cannot be read counts as 0 pixels. The resolution is read only for files in a group and is stored in the fingerprint cache (DUP-012).

How groups form does not change: the first file not yet in a group starts one, and every later matching file joins. Only the kept file changes. All other files are reported against it.

## Consequences
- Messenger copies, thumbnails and downscaled exports lose against the original, whatever their names are.
- The `canonical` in the log can now be any file of the group, not only the first.
- In content mode, the kept file and a duplicate both matched the file that started the group, but were not necessarily compared with each other.
- Reading a header costs one small read per group member. Later runs take it from the cache.
- If a heavily edited copy has a higher resolution than the original (e.g. upscaled), the copy is kept.

## Alternatives considered
- **Largest file only:** needs no new data, but a re-saved PNG or a copy saved at higher quality beats the original, and in metadata mode both files have the same size anyway.
- **Never keep WhatsApp names (`IMG-YYYYMMDD-WAxxxx`):** fixes exactly this case, but it is a name heuristic and misses Telegram, Signal, screenshots and edited exports.
- **Prefer files with EXIF:** WhatsApp strips EXIF, but screenshots and many originals have none, so this does not decide the pair on its own.
