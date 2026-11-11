# ADR-0014: Persisted run status, dry runs kept in memory only

- **Status:** accepted
- **Date:** 2026-11-11
- **Specs:** WEB-011

## Context
The web UI status page shows the last result of each job. If the results lived only in memory, a restart (which CFG-005 requires after every config change) would empty the page until each job ran again. That can take a day for a nightly schedule. Writing a file, however, conflicts with the safety rule that `dryRun` never changes a file, including state files.

## Decision
- A new package `internal/runstatus` keeps the last record per job: start, duration, counters, and the dry-run flag.
- Records of **non-dry** runs are written to `<stateDir>/run-status.json` with an atomic write (`fileops.WriteFileAtomic`) after every run, and loaded at startup. This follows the JSON flat-file approach of [ADR-0005](0005-json-state-files.md).
- Records of **dry** runs stay in memory only. The page shows them with a "dry run" badge until the next restart.
- A failure to write the status file is logged and never affects the job (error resilience).
- "Running" and "next run" come from the scheduler at request time and are not stored.

## Consequences
- After a restart, the status page shows the last real run of each job right away.
- A dry run never creates or changes `run-status.json`, so the dryRun rule holds without exceptions.
- After a restart, a dry run's result is gone, and the page may show an older real run for that job instead.
- The file only holds the last record per job. There is no history.

## Alternatives considered
In-memory only was rejected, because the page would be empty after every config change. Persisting dry runs as well was rejected, because it would need an exception to the dryRun safety rule. A history log was deferred as not needed for "what happened last".
