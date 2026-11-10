# ADR-0011: Specs in `docs/specs` + `AGENTS.md` instead of openspec

- **Status:** accepted
- **Date:** 2026-11-10
- **Specs:** all (process decision)

## Context
Phorga used openspec, with propose/apply/archive changes and delta specs. The tooling added ceremony. The main specs still contained delta headings, had drifted from the code (date fallback, pHash threshold), and were not linked to code or tests.

## Decision
- **AI-first specs** live in `docs/specs/*.md`. Each requirement has a stable ID (`ORG-001` …), a status, WHEN/THEN scenarios, and `Code`/`Tests` references. Tests carry `// Spec: <ID>` comments.
- **Human docs** are `docs/product.md` (what and why), `docs/arc42/` (architecture) and `docs/decisions/` (ADRs). The archived openspec designs became ADRs 0001–0010.
- **`AGENTS.md`** is the shared entry point for humans and AI. `CLAUDE.md` imports it. The workflow is: spec first, then code and tests, then human docs.
- `docs/docs_test.go` checks links, IDs, and code/test references in `go test ./...`.

## Consequences
- No external tool is needed. Specs are plain Markdown next to the code.
- Drift is caught mechanically for references, but not for semantics. Reviews still have to compare scenarios with the behavior.
- History of earlier changes is in git and in the ADRs. The openspec archive was removed.
