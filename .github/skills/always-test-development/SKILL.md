---
name: always-test-development
description: Enforce a test-first, test-always workflow for software development. Use this skill whenever writing, changing, refactoring, or reviewing code so tests are created, updated, and run alongside the implementation.
---

# Always Test Development

## Purpose

When developing software, prioritize tests as a normal part of the work, not an optional final step. Prefer writing, updating, and running tests alongside any code change.

## Core rules

- Always consider tests before, during, and after implementation.
- Do not consider a task complete until relevant tests exist and pass.
- Prefer small, focused, deterministic tests.
- Keep tests fast so they can run frequently in local development and CI.
- Test behavior through public interfaces where possible.
- Use mocks or stubs for external dependencies when isolation is needed.
- Include edge cases, invalid inputs, and failure paths.
- Update tests whenever behavior changes.
- If a change cannot be tested easily, explicitly explain why and suggest a safer design.

## Workflow

1. Identify the behavior to change.
2. Determine which tests should fail before the code change.
3. Write or update tests first when practical.
4. Implement the minimum code needed to satisfy the tests.
5. Run the relevant test subset.
6. Run the broader test suite if the change could affect other areas.
7. Fix failing tests or code until all relevant checks pass.

## Expectations by task type

- New feature: add unit tests and, when needed, integration tests.
- Bug fix: add a regression test that reproduces the bug.
- Refactor: run the existing suite and preserve behavior.
- API change: update contract tests and downstream tests if applicable.
- CI-related work: ensure tests are executable in automation and remain stable.

## Test quality guidelines

- Each test should check one behavior.
- Use descriptive test names.
- Keep setup minimal and readable.
- Avoid flaky timing, network, or shared-state dependence.
- Prefer Arrange-Act-Assert style when useful.
- Clean up test data and reset state after each test.

## Default response behavior

When helping with code:
- Mention the tests that should be added or updated.
- Prefer showing the test first when the task is testable.
- If code is requested without tests, still include tests unless the user explicitly says not to.
- If tests are skipped, explain the tradeoff clearly.

## Examples

- For a new function, create unit tests for normal, edge, and error cases.
- For a bug fix, add a failing test that proves the bug exists, then fix the code.
- For a refactor, preserve behavior by running the existing tests before and after the change.

## Reminder

Tests are part of the implementation, not a separate optional step.