---
name: keep-arc42-up-to-date
description: Keep the small arc42 architecture documentation in AsciiDoc accurate and current, including diagrams. Use this skill whenever architecture, structure, runtime behavior, deployment, interfaces, or major decisions change.
---

# Keep arc42 Up to Date

## Purpose

The arc42 documentation is part of the system’s source of truth. Keep the AsciiDoc content and diagrams aligned with the current architecture at all times.

## Scope

This applies to the small arc42 set used for the project, typically focused on the most relevant chapters:
- Introduction and goals.
- Constraints.
- Context and scope.
- Solution strategy.
- Building block view.
- Runtime view.
- Deployment view.
- Crosscutting concepts.
- Architectural decisions.
- Risks and technical debt.
- Glossary.

## Core rules

- Update the arc42 docs whenever the architecture changes.
- Update diagrams whenever the underlying structure, runtime flow, deployment, or boundaries change.
- Keep the documentation small, relevant, and current.
- Prefer schematic diagrams over detailed ones.
- Document architecture at the right level of abstraction, not source-code detail.
- Remove outdated diagrams, obsolete decisions, and stale assumptions.
- Use AsciiDoc consistently for text, structure, and embedded diagrams.
- Treat documentation changes as part of the same change as the code or infrastructure change.

## What must trigger an update

Review and update arc42 when changing:
- Service boundaries or module structure.
- External systems or integration points.
- Key runtime flows or interaction patterns.
- Deployment topology, environments, or infrastructure.
- Communication protocols or APIs.
- Major design decisions or tradeoffs.
- Cross-cutting concerns such as security, observability, or resilience.
- Known risks, constraints, or technical debt.

## Writing style

- Keep the text concise and technical.
- Assume the reader is experienced with software architecture.
- Avoid tutorial-style explanations.
- Focus on what changed, why it matters, and how the system is currently shaped.
- Use precise terminology.
- Prefer short sections with direct statements.
- Do not over-document implementation details that belong in code.

## Diagram rules

- Use diagrams to clarify architecture, not to duplicate code.
- Keep diagrams schematic and high-level.
- Show only the elements needed to understand the architecture.
- Update diagram labels, relationships, and boundaries when reality changes.
- Prefer simple, readable diagrams over dense ones.
- If a diagram becomes too detailed, simplify it.

## Workflow

1. Review the proposed code or infrastructure change.
2. Identify which arc42 sections are affected.
3. Update the relevant AsciiDoc sections and diagrams.
4. Remove stale content at the same time.
5. Verify that the documentation matches the current implementation.
6. Keep the final result small and useful.

## Quality bar

A good arc42 doc for this project:
- Reflects the current architecture.
- Explains the important decisions and boundaries.
- Uses diagrams that are easy to scan.
- Stays short enough to maintain.
- Avoids stale or overly detailed content.
- Helps experienced engineers understand the system quickly.

## Default response behavior

When helping with development work:
- Mention which arc42 sections need updating.
- Prefer updating the documentation directly if the change affects architecture.
- If the docs are not updated, explain why they still remain valid.
- If information is missing, add a clear placeholder or TODO rather than leaving outdated text.

## Phorga specifics

- The file is `docs/arc42/arc42.adoc`.
- §1 "Requirements Overview" stays a short summary that links to `docs/specs/`. Do not copy requirements into arc42.
- §9 "Architectural Decisions" is a list of links to `docs/decisions/`. Write the ADR first, then add one row.
- §10 "Risks and Technical Debt" should mirror the open `**Notes:**`/`**Gap:**` items from the specs that matter architecturally.
- Spec files link to arc42 sections by name (e.g. "§6 Organizer Run"). If you rename a section, update those links.

## Reminder

If the architecture changes, the arc42 docs should change with it. If the diagrams change, the text should still match them.