# ADR-0006: Atomic file operations (temp file + rename)

- **Status:** accepted
- **Date:** 2026-05-17 (Phorga v1)
- **Specs:** ORG-004, ORG-015, CFG-004

## Context
Phorga moves users' photos unattended, often on NAS hardware that can lose power. A half-written target file followed by deleting the source would lose data.

## Decision
All writes go to a temp file in the target directory, followed by a rename. This covers media files, state files and config files. In `move` mode, the source is deleted only after the rename succeeded.

## Consequences
- A crash leaves either the old state or the new one, never a partial file. At worst, a leftover temp file remains.
- Moving across filesystems is a copy plus delete, so it costs I/O.
