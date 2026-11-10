# ADR-0010: Language chosen by token name, case controls rendering

- **Status:** accepted
- **Date:** 2026-06-06
- **Specs:** ORG-006, ORG-007, ORG-008

## Context
German, French and Italian users want month names in their language in folder names (`2024/März`).

## Decision
- The language is chosen by the **token name** (`{monat}`, `{mois}`, `{mese}`). There is no `language` config field.
- The **case of the token** controls the output: `{monat}` → `märz`, `{Monat}` → `März`, `{MONAT}` → `MÄRZ`.
- Numeric tokens are always two digits (`02`, `05`).
- The token set is fixed: year, month, day in EN/DE/FR/IT, plus `monthWithName`, `monthNumber` and `dayNumber`. Unknown tokens fail validation.

## Consequences
- The template alone shows the result. Tokens of different languages can be mixed.
- English has no month-name token except `{monthWithName}` (`03-march`), because `{month}` is numeric for backwards compatibility.
- A new language requires a code change. The token list exists in both `internal/config/validate.go` and `internal/metadata/targetpath.go`.

## Alternatives considered
A `language:` config field was rejected because it adds a config dimension. Format modifiers (`{monat|upper}`) were rejected because they add syntax.
