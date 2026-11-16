# ADR-0017: Detail check after the pHash match in content mode

- **Status:** accepted
- **Date:** 2026-11-16
- **Specs:** DUP-003, DUP-012, DUP-014

## Context
Content mode treated every pair with a pHash distance ≤ 5 as duplicates (ADR-0004). A real library showed that burst shots taken one second apart match too: one person with eyes closed, another one who moved. pHash scales the image to 32×32 and keeps only its coarse layout, so these pairs reached distance 0–4. A lower threshold does not help, because one of the pairs had distance 0.

Measurements on the user's photos, with the worst 4×4-pixel tile of a 64×64 thumbnail:

| Pair | pHash | Flipped steps, worst tile |
|---|---|---|
| burst shots (3 pairs) | 0–4 | 0.22–0.63 |
| identical file | 0 | 0 |
| copy resized to 25 %, JPEG q60 | 0 | 0 |
| copy resized to 10 %, JPEG q40 | 0 | ≤ 0.03 |
| copy resized to 5 %, JPEG q30 | 0 | ≤ 0.07 |
| copy with gamma 0.8 or 1.25 | – | 0 |

Comparing the brightness of the tiles instead (after normalising the whole image) also separated the burst shots, but scored gamma-edited copies at 0.16–0.21, which overlaps with real changes.

## Decision
Content mode becomes two stages. pHash ≤ 5 only makes a pair a candidate. The candidates are then compared by a detail fingerprint: a 64×64 grayscale thumbnail, split into 16×16 tiles. For each tile, we count the neighbouring pixel pairs whose brightness step points in a different direction; pairs flat in both images are ignored. A pair is a duplicate only when no tile exceeds 12 % flipped steps. Like the pHash threshold, this one is fixed.

The fingerprint (4 KB) is computed only for candidates and stored in the existing fingerprint cache (DUP-012), so an unchanged library is not read again.

## Consequences
- Burst shots and similar series are no longer moved as duplicates.
- Re-encoded, resized and tone-edited copies are still found, because the direction of brightness steps survives these edits.
- Candidates are decoded a second time in the run that first sees them. Libraries with many burst series take longer on that run.
- `dedup-cache.json` grows by about 5.5 KB (base64) per candidate file, not per file.
- Very small changes (a few pixels in a large group photo) can stay below the threshold. Cropped or rotated copies were not found before and are still not found.

## Alternatives considered
- **Lower pHash threshold or a 256-bit pHash:** still global; the burst pairs reached distance 0 (64 bit) and 10 (256 bit), too close to copies (0–2).
- **EXIF veto (different capture dates are never duplicates):** cheap, but copies from messengers often lose EXIF, and it would mix metadata rules into content mode. Not needed with the detail check.
- **Combining modes (`content,metadata`):** `metadata` requires the same file size, which no resized copy has.
- **Storing the thumbnail of every file:** would let the comparison skip pHash, but makes the cache many times larger for no gain.
