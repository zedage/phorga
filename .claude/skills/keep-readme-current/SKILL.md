---
name: keep-readme-current
description: Keep the repository README accurate, concise, and up to date for a technically experienced audience. Use this skill whenever code, behavior, deployment, configuration, or project structure changes.
---

# Keep README Current

## Purpose

The README must stay aligned with the current state of the codebase. Treat README updates as part of the same change as the implementation, not as a separate cleanup task.

## Core rules

- Update the README whenever behavior, setup, configuration, deployment, dependencies, or project structure changes.
- If code changes affect how someone uses, builds, runs, tests, or deploys the project, update the README in the same task.
- Keep the README concise and high-signal.
- Write for readers who are already comfortable with software and technical terminology.
- Avoid beginner-friendly filler, overexplaining, and generic onboarding prose.
- Prefer precise wording, short sections, and directly useful examples.
- Remove stale instructions, outdated assumptions, and duplicated content.
- If the README becomes too large, keep the root README focused and link to deeper docs.

## Audience

Assume the reader:
- Knows common development workflows.
- Understands technical terms without hand-holding.
- Wants the fastest path to understanding what changed, how to use it, and what matters operationally.

## What to keep current

- Project purpose and scope.
- Build, run, and test commands.
- Configuration and environment variables.
- Deployment or release notes that affect users.
- Architectural decisions that affect usage.
- API or CLI behavior.
- Known constraints or compatibility notes.
- Links to related docs when the README should stay brief.

## What the README should do

- Explain what the project is.
- Show the essential commands.
- Document the minimal necessary setup.
- Highlight important usage patterns.
- Call out non-obvious operational details.
- Stay current with the codebase.

## Writing style

- Be concise.
- Use technical language where appropriate.
- Prefer short sections with clear headings.
- Use examples only when they add real value.
- Avoid marketing language.
- Avoid long tutorials or step-by-step beginner walkthroughs.
- Do not repeat information that is already obvious from the code or repo structure.

## Workflow

1. Review the change for user-facing impact.
2. Identify which README sections are now stale.
3. Update the README together with the code change.
4. Remove outdated instructions or assumptions.
5. Check that commands, file paths, and names still match reality.
6. Keep the final wording lean and technical.

## Update triggers

Always review the README when changing:
- Installation or bootstrap steps.
- Runtime behavior.
- Build or test commands.
- Environment variables.
- Service endpoints, ports, or protocols.
- Deployment or release workflows.
- Directory layout or major module boundaries.
- External dependencies or compatibility requirements.

## Default response behavior

When helping with code:
- Mention whether the README needs updating.
- Point to the exact sections likely affected.
- Prefer making the README change directly if the task already touches related functionality.
- If the README is not updated, explain why it is still accurate.
- If information is missing, add a short placeholder or TODO rather than leaving stale content.

## Quality bar

A good README for this project:
- Is accurate.
- Is short enough to scan quickly.
- Reflects the current state of the repo.
- Helps an experienced engineer act immediately.
- Does not contain outdated or redundant material.

## Phorga specifics

- The README is for **users**: install, configure, run. The product explanation lives in `docs/product.md`, exact behavior in `docs/specs/`, and contributor guidance in `AGENTS.md`. Link to those instead of duplicating them.
- Config changes must update the README configuration section **and** `config.example.yaml` together.
- `docs/product.md` links to README anchors (`#configuration`, `#targetstructure-tokens`). Keep those headings stable, or update the links.

## Reminder

If the code changed, the README should be checked. If the README changed, it should describe the code as it exists now.