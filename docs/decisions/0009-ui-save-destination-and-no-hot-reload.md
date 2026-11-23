# ADR-0009: UI saves to active or separate file, no hot reload

- **Status:** superseded in part by [ADR-0022](0022-live-apply-and-setup-mode.md) (hot reload)
- **Date:** 2026-06-06
- **Specs:** WEB-004, CFG-004, CFG-005

## Context
Users want to try out config changes without risking the running config. Rewriting the original YAML text in place is fragile.

## Decision
- The UI always shows the config **loaded at startup**. Saving requires an explicit choice: overwrite the active file, or write a separate file.
- Saving serializes the config model (`config.Save`), with format by extension and an atomic write. It does not patch the original text.
- The running process does **not** reload. Changes take effect after a restart.

## Consequences
- Saving is safe and predictable, and a separate file allows trying out changes.
- Comments and formatting in a hand-written YAML file are lost when it is overwritten from the UI.
- After saving, the UI and the running jobs can differ until the next restart.

## Alternatives considered
Always overwriting the active file was rejected because it removes the requested choice. Text-based patching was rejected as brittle. Hot reload was deferred because of its complexity around running jobs.
