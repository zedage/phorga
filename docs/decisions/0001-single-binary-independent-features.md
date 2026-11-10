# ADR-0001: One binary, organizer and duplicate detection kept independent

- **Status:** accepted
- **Date:** 2026-05-17 (Phorga v1)
- **Specs:** DUP-001, ORG-011

## Context
Phorga has two features: organizing files by date, and finding duplicates across folders. They could ship as separate tools or as one.

## Decision
A single Go binary runs both features as independent cron jobs in one process with one config file. They do not share settings: the organizer has only lightweight duplicate handling (`filename`/`filedate`/`exif`), while content-based detection exists only in `duplicateDetection`, which has its own sources, schedule and actions.

## Consequences
- One container and one config file are simple to deploy on a NAS.
- Users can run either feature alone.
- There are two kinds of "duplicate" settings, which can confuse users. The docs must keep them clearly apart.

## Alternatives considered
Separate binaries were rejected because they double the Docker and config effort with no functional benefit.
