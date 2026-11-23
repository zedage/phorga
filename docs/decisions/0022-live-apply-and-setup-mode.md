# ADR-0022: Apply UI changes live, start without a config

- **Status:** accepted
- **Date:** 2026-11-21
- **Specs:** CFG-008, CFG-009, CFG-002, CFG-003, WEB-001, WEB-004
- **Supersedes:** the "no hot reload" part of [ADR-0009](0009-ui-save-destination-and-no-hot-reload.md)

## Context
Phorga is meant to be set up in the browser, typically in a container on a NAS. Two things got in the way. A missing or empty config stopped Phorga at startup, so there was no UI to set it up with. And every change saved in the UI took effect only after restarting the container. ADR-0009 had put off hot reload because of running jobs: a run can take hours, and stopping it for a settings change is not acceptable.

## Decision
- **Setup mode.** A missing config file is not an error. Phorga starts with no jobs and the web UI on, and the first save creates the file. A config without jobs is valid, and the web UI always starts then. The Docker image sets `PHORGA_WEB_UI_ADDR=0.0.0.0:8080`, so the UI is reachable in a fresh container without any config.
- **Live apply.** Saving to the active file validates, writes the file, reads it back and swaps the scheduled jobs: all old cron entries are removed and the new ones added. A running job is **not** interrupted; it finishes with its old settings. Overlap protection is per job name and survives the swap, and jobs still run one at a time.
- **Restart only for process-wide settings.** `stateDir` (state files, run status and fingerprint cache stay where they were opened), `webUI.address` and turning the UI off take effect after a restart, and the save result names them.
- **The file stays the source of truth.** Live apply only happens through the UI save, after the file was written. Phorga does not watch the file for edits made elsewhere.
- Saving to a separate file stays possible and never applies anything (ADR-0009).

## Consequences
- A fresh container can be set up completely in the browser, and changes apply at once.
- A mistyped `--config` path starts an empty Phorga instead of failing. The log and the UI show which path is in use.
- For a while after a save, a job may still be running with the old settings. Its next run uses the new ones.
- Job names are positions, so removing a profile renames the jobs after it; their last results are dropped from the status page instead of being shown under the wrong folder.
- The config folder must be writable by Phorga's user. In Docker the folder has to be mounted, not the file alone, because a file mounted on its own cannot be replaced atomically.

## Alternatives considered
- **Stop running jobs on apply**, like a shutdown. Rejected: a long organizer or duplicate run would restart from scratch for an unrelated change, such as an ntfy URL.
- **Restart the whole process from inside.** Rejected: it would stop running jobs and drop the UI connection, and in Docker it depends on the restart policy.
- **Watch the config file** (fsnotify or polling). Rejected for now: half-written files from editors and network shares make it unreliable, and it adds a dependency. A restart still picks up manual edits.
- **Keep failing on a missing config** and ship a sample config in the image. Rejected: the sample would need paths that match the user's mounts, which is the very thing the UI should help with.
