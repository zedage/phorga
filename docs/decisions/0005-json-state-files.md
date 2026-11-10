# ADR-0005: JSON flat files for state, no database

- **Status:** accepted
- **Date:** 2026-05-17 (Phorga v1)
- **Specs:** ORG-015

## Context
The organizer must remember which files it has already processed, so restarts do not redo work. Phorga must cross-compile to ARM for Synology NAS.

## Decision
State is a JSON file per organizer profile target and source in `stateDir`, keyed by path, size and mtime. It is written atomically. A missing or corrupt file means a full re-scan, not an error.

## Consequences
- No CGO and no external service. The files are easy to inspect and to delete for a reset.
- The whole state is loaded into memory per run. That is fine for typical libraries but not designed for many millions of files.

## Alternatives considered
SQLite was rejected because it needs CGO, which complicates ARM cross-compilation.
