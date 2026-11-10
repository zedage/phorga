# ADR-0004: Perceptual hash with fixed threshold for content dedup

- **Status:** accepted
- **Date:** 2026-05-17 (Phorga v1)
- **Specs:** DUP-003

## Context
Duplicate photos are often re-encoded, resized or re-saved copies. Byte hashes miss them.

## Decision
Content mode uses `github.com/corona10/goimagehash` pHash. Two images are duplicates if the Hamming distance is **≤ 5**. The threshold is fixed and not configurable.

## Consequences
- Re-encoded and resized copies are caught.
- Only decodable images are compared. Videos, HEIC and RAW are not compared by content.
- Pairwise comparison is O(n²) and decoding costs memory. Images are processed one at a time (see arc42 §10).
- A fixed threshold keeps the config simple, but cannot be tuned.

## Alternatives considered
SHA-256 of the file bytes only catches exact copies. The v1 design document proposed a threshold of 8. The implementation chose 5 to reduce false positives, and the specs now follow the code.
