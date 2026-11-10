---
name: keep-specs-current
description: Keep Phorga's requirement specs in docs/specs/ in sync with the code, tests and human docs. Use this skill whenever a change adds, alters or removes behavior (features, bug fixes that change behavior, config fields, defaults, validation rules, UI endpoints), when planning a feature, or when code and a spec seem to disagree.
---

# Keep Specs Current

## Purpose

`docs/specs/*.md` is the verifiable source of truth for Phorga's behavior. Every behavior change starts and ends there. The human docs (`docs/product.md`, arc42, README, ADRs) are kept in sync and link to the specs.

The full workflow is in `AGENTS.md` (section "Change workflow"). The spec format is in `docs/specs/README.md`. Read both if you have not done so in this session.

## Workflow

1. **Locate.** Find the affected requirement(s): `grep -rn "<keyword>" docs/specs/`. Read the whole requirement, including its Notes and Gap lines.
2. **Propose.** For new behavior, add a requirement with the next free ID of that file's prefix (`ORG`, `DUP`, `CFG`, `NTF`, `WEB`). For larger or unclear changes, set `**Status:** proposed` and confirm with the user before coding. If you change existing behavior, edit the requirement text and scenarios. Never renumber or reuse IDs. To retire one, use `**Status:** removed` plus a one-line reason.
3. **Implement and test.** Turn each WHEN/THEN scenario into a test (see the `always-test-development` skill). Add `// Spec: <ID>` above each covering test function.
4. **Close.** Set `**Status:** implemented` and update `**Code:**` (repo-relative paths) and `**Tests:**` (`path#TestName`). If something is not tested, write `none` plus a `**Gap:**` line.
5. **Sync the human docs:**
   - `docs/product.md` if users would notice the change (prose, no MUST lists, link to the spec file)
   - arc42 if structure, runtime flow, deployment or risks changed (`keep-arc42-up-to-date`)
   - README and `config.example.yaml` if usage or config changed (`keep-readme-current`)
   - a new ADR in `docs/decisions/` if a real trade-off was made, linked from the spec and from arc42 §9
6. **Verify.** Run `go test ./...`. `docs/docs_test.go` fails on duplicate or unknown IDs, missing code or test references, and broken links.

## Drift: code and spec disagree

Do not silently pick one side.
- If the intended behavior is clear (e.g. the spec describes a bug), fix the code and add a regression test.
- If the spec is outdated, update the spec and add a `**Notes:**` line saying what changed.
- If it is unclear, ask the user. Meanwhile, record it as an open question in `**Notes:**`.

Always mention drift fixes in the final summary and in the PR description.

## Writing rules

- One requirement = one testable behavior. Split it if the scenarios test unrelated things.
- Use MUST / MUST NOT / SHOULD, with concrete values in the scenarios (paths, dates, config snippets).
- Describe observable behavior, not implementation. Code details belong in `Code:` paths and comments.
- Keep each spec file's header links (product.md section, arc42 section, ADRs) valid.

## Done when

- [ ] Every changed behavior has an up-to-date requirement with status `implemented`
- [ ] `Code:`/`Tests:` lines are accurate, and tests carry `// Spec:` comments
- [ ] product.md, arc42, README and ADRs are updated where affected
- [ ] `go test ./...` is green
