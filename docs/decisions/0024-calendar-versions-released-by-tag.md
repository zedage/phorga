# ADR-0024: Calendar versions, released by a Git tag

- **Status:** accepted
- **Date:** 2026-11-23
- **Specs:** RUN-004

## Context
Phorga had no versions. The Docker workflow pushed `:latest` after every merge to `main`, so whoever pulled `:latest` on a NAS got whatever was merged last, including breaking changes such as the switch to a non-root image. The only way to pin a build was `sha-<commit>`, which says nothing about age or order, and a running Phorga could not tell which build it was.

## Decision
- **Calendar versions `YYYY.MM.MICRO`**, e.g. `2026.11.0`, then `2026.11.1`, then `2026.12.0`. `MICRO` counts the releases within a month from 0. The month is zero-padded, so versions sort as text.
- **A release is a Git tag `vYYYY.MM.MICRO` on `main`**, pushed by hand. The Docker workflow checks the format and that the tag is on `main`, builds the image with the version baked in (`-X main.version`), pushes `:2026.11.0`, `:2026.11` and `:latest`, and creates a GitHub release with generated notes.
- **A merge to `main` is not a release.** It pushes `:edge` and `:sha-<commit>`; the binary reports `dev+<commit>`.
- The version is shown by `phorga --version`, in the start log and in the web UI's top bar (RUN-004).

## Consequences
- `:latest` changes only when someone releases. A NAS that pulls `:latest` gets released builds, and `:2026.11` gives the newest fix of a month.
- Whoever wants every merge pulls `:edge`. This changes the meaning of `:latest` for existing users once: it now stays on the last release.
- Somebody must cut releases. Nothing happens automatically, so a fix sits in `:edge` until the next tag.
- The zero-padded month (`2026.01.0`) is not strict SemVer. The workflow uses `type=match` instead of `type=semver`, and tools that sort SemVer may not accept these versions.

## Alternatives considered
- **A release on every merge to `main`:** no manual step, but every merge would reach `:latest`, which is what this ADR is meant to stop.
- **SemVer (`1.4.2`):** says something about compatibility, but for an application without an API, deciding major vs. minor is guesswork; the release date tells users more.
- **`YYYY.MM.DD`:** shows the day, but needs a fourth part for two releases on one day, and the numbers are long.
- **`YYYY.M.MICRO` without padding:** strict SemVer, but sorts wrongly as text (`2026.10.0` before `2026.9.0`).
