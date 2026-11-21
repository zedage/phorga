# ADR-0020: Run one job at a time

- **Status:** accepted
- **Date:** 2026-11-21
- **Specs:** RUN-002, ORG-010, DUP-008, WEB-011

## Context
Each organizer source and each duplicate detection job has its own cron schedule. The scheduler only prevented a job from overlapping with itself, so different jobs ran in parallel. Typical configurations overlap on disk: duplicate detection often scans the organizer's target, and several sources of one profile write into the same target folders. In parallel, one job can move a file while another job is reading, comparing or writing next to it. arc42 §10 claimed a lock file per source that did not exist.

## Decision
The scheduler runs at most one job at a time. A job that is due while another one runs waits for it and starts afterwards. It still never overlaps with itself: if it fires again while running or waiting, that run is skipped. Waiting jobs do not start after a stop request. The status page shows a waiting job as "waiting for another job".

## Consequences
- Jobs never see each other's half-finished work, so the safety rules (ORG-020, DUP-005) only have to deal with other programs.
- Total wall-clock time can grow when schedules overlap. On a NAS with spinning disks, parallel jobs mostly competed for the same disk, so the loss is small.
- A long duplicate detection run delays the organizer, and the reverse. Users who want a predictable start can spread the schedules.
- A job that hangs blocks all others. Jobs already check their context between files (RUN-001), so a hang would be a bug anyway.

## Alternatives considered
- **Locks per folder:** keeps parallelism for unrelated folders, but needs path-overlap logic (nested sources, targets inside sources, symlinks) and still lets jobs interleave on shared folders.
- **Skip instead of wait:** simpler, but a job that always collides with a long one would never run.
- **A configurable worker count:** more knobs for little gain, given the disk is the bottleneck.
