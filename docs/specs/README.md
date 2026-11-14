# Phorga Specifications

These files are the **verifiable source of truth for Phorga's behavior**. Each requirement has a stable ID, testable scenarios, and pointers to the code and tests that implement it.

They are written mainly so that AI agents (and reviewers) can change code safely. For the human-oriented explanation of *what* Phorga does and *why*, see [product.md](../product.md). For architecture, see [arc42](../arc42/arc42.adoc). For decisions, see [decisions/](../decisions/).

| File | Prefix | Covers |
|---|---|---|
| [organizer.md](organizer.md) | `ORG` | Date-based organization, target structure tokens, lightweight duplicates, state |
| [duplicate-detection.md](duplicate-detection.md) | `DUP` | Independent cross-source duplicate detection |
| [configuration.md](configuration.md) | `CFG` | Config loading, path resolution, validation, saving |
| [notifications.md](notifications.md) | `NTF` | ntfy.sh run summaries |
| [web-ui.md](web-ui.md) | `WEB` | Optional built-in configuration UI, folder picker and status page |
| [runtime.md](runtime.md) | `RUN` | Process-wide behavior such as shutdown |

## Requirement format

```markdown
### ORG-010 Per-source scheduling
**Status:** implemented

The system MUST run each source on its own cron schedule.

- **WHEN** a run is still active and its schedule fires again
  **THEN** the new run is skipped and a warning is logged

**Code:** `internal/scheduler/scheduler.go`
**Tests:** `internal/scheduler/scheduler_test.go#TestOverlapPrevention`
```

Rules:

- **IDs are stable.** Never renumber and never reuse an ID. To retire a requirement, set `**Status:** removed` and add one line explaining why. Leave it in place.
- **Status** is one of:
  - `proposed`: agreed but not built yet. `Code`/`Tests` may be `none`.
  - `implemented`: the code behaves as described, and the listed tests cover it.
  - `removed`: no longer valid. Kept for history.
- **Keywords:** MUST / MUST NOT / SHOULD are used as in RFC 2119.
- **Scenarios** use `WHEN … THEN …` with concrete values, so they can be turned into tests directly.
- **Code** lists repo-relative paths in backticks.
- **Tests** lists `path#TestName` entries in backticks. Use `none` if no automated test exists yet, and add a `**Gap:**` line explaining what is missing.
- **Notes** (optional) record known deviations or open questions. A note is not a requirement.
- Each test that covers a requirement carries a `// Spec: <ID>[, <ID>]` comment above the test function. This keeps `grep -rn ORG-010` useful in both directions.

`docs/docs_test.go` enforces the mechanical parts: unique IDs, valid statuses, existing code paths and test functions, `// Spec:` comments that reference known IDs, and Markdown links that resolve.

## Changing specs

The workflow is described in [AGENTS.md](../../AGENTS.md#change-workflow). In short: **spec first, then code and tests, then human docs.** A behavior change without a spec change is incomplete.
