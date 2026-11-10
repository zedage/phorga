# ADR-0007: Rename the project from Phorganizer to Phorga

- **Status:** accepted
- **Date:** 2026-05-30
- **Specs:** none (project-wide constraint)

## Context
The project started as "Phorganizer". The repository and the public name became "Phorga".

## Decision
The name is `Phorga` in prose, `phorga` in code, paths, the binary and the Docker image, and `PHORGA_` for environment variables (e.g. `PHORGA_CONFIG`). The Go module is `github.com/zedage/phorga` and the entry point is `cmd/phorga`. The rename was done in a single pass with case-sensitive replacements. No aliases were kept, since there were no external users yet.

## Consequences
- New code, docs and config must not use the old name.
- Lesson learned: the `.gitignore` pattern `phorga` (meant for the binary) also ignored `cmd/phorga/`, so `main.go` went missing from the repo until PR #4. Ignore patterns for binaries must be anchored (`/phorga`).
