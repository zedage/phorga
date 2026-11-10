# ADR-0002: In-process cron scheduling with `robfig/cron/v3`

- **Status:** accepted
- **Date:** 2026-05-17 (Phorga v1)
- **Specs:** ORG-010, DUP-008

## Context
Each organizer source and each duplicate detection job needs its own schedule, and Phorga runs as a long-lived container.

## Decision
Phorga uses `github.com/robfig/cron/v3` inside the process with standard 5-field expressions. The scheduler wrapper (`internal/scheduler`) skips a run if the previous run of the same job is still active.

## Consequences
- No OS cron is needed in the container, and state handling stays in one process.
- Schedules are validated at startup.
- Jobs run only while the daemon is running. A missed run is not caught up later.

## Alternatives considered
OS-level cron with one-shot runs was rejected because it complicates the Docker setup and state handling.
