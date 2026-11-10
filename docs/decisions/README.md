# Architecture Decision Records

Short records of decisions that shaped Phorga: what was decided, why, and what it costs. They explain the "why" behind the [specs](../specs/README.md) and the [arc42 documentation](../arc42/arc42.adoc).

| ADR | Decision | Date | Status |
|---|---|---|---|
| [0001](0001-single-binary-independent-features.md) | One binary, organizer and duplicate detection kept independent | 2026-05-17 | accepted |
| [0002](0002-in-process-cron-scheduling.md) | In-process cron scheduling with `robfig/cron/v3` | 2026-05-17 | accepted |
| [0003](0003-date-resolution-order.md) | Date resolution: EXIF → filename → path → mtime | 2026-05-17 | accepted |
| [0004](0004-perceptual-hash-for-content-dedup.md) | Perceptual hash with fixed threshold for content dedup | 2026-05-17 | accepted |
| [0005](0005-json-state-files.md) | JSON flat files for state, no database | 2026-05-17 | accepted |
| [0006](0006-atomic-file-operations.md) | Atomic file operations (temp file + rename) | 2026-05-17 | accepted |
| [0007](0007-rename-to-phorga.md) | Rename the project from Phorganizer to Phorga | 2026-05-30 | accepted |
| [0008](0008-built-in-server-rendered-web-ui.md) | Optional built-in, server-rendered web UI with HTMX | 2026-06-06 | accepted |
| [0009](0009-ui-save-destination-and-no-hot-reload.md) | UI saves to active or separate file, no hot reload | 2026-06-06 | accepted |
| [0010](0010-multilingual-tokens-by-name.md) | Language chosen by token name, case controls rendering | 2026-06-06 | accepted |
| [0011](0011-specs-and-agents-md-instead-of-openspec.md) | Specs in `docs/specs` + `AGENTS.md` instead of openspec | 2026-11-10 | accepted |

## Writing a new ADR

Add an ADR when a change makes a real trade-off: something a future reader might want to undo without knowing why it was done. Copy the structure of an existing ADR, use the next number, add a row above, and link it from the affected spec requirement and from arc42 §9. Do not edit accepted ADRs in substance. If a decision changes, write a new ADR and set the old one to `superseded by NNNN`.
