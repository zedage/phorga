## Context

Currently, `ResolveTargetPath()` in `internal/metadata/targetpath.go` uses a fixed set of English token placeholders:
- `{year}` → `2024`
- `{month}` → `02` (numeric)
- `{monthWithName}` → `02-february`
- `{day}` → `15`

Users want to organize photos using natural language month/day names in different languages (German, French, Italian) with case-sensitive formatting. The current hardcoded English-only approach and limited token set prevent this use case.

## Goals / Non-Goals

**Goals:**
- Support language-specific month/day name placeholders in targetStructure
- Support numeric placeholders (`monthNumber`, `dayNumber`) for flexible date formatting
- Enable case-sensitive placeholder values (e.g., `Februar`, `februari`, `FEBRUAR`)
- Allow any combination of numeric and name tokens with arbitrary separators
- Validate only recognized tokens; ignore unrecognized placeholders
- Maintain backward compatibility with existing English tokens

**Non-Goals:**
- Weekday names (out of scope for v1, can be added later)
- Locale detection from system - language explicitly chosen via token choice
- Custom date/time formatting beyond month/day names
- Recursive or conditional token replacement

## Decisions

**1. Language Specification via Token Naming (not Config Field)**
- **Decision**: Use language-specific token names (`{monat}`, `{mois}`, `{mese}`) rather than a separate `language` config field
- **Rationale**: Single source of truth - language is evident in the targetStructure itself. Users can mix tokens (though not recommended). No extra config dimension.
- **Alternatives**: 
  - Separate `language: "de"` field would require parsing and validation, adds config complexity
  - Central locale file would be inflexible for users who want different formats in different organizers

**2. Case-Sensitive Placeholder Rendering**
- **Decision**: Token case determines output case: `{monat}` → `februar`, `{Monat}` → `Februar`, `{MONAT}` → `FEBRUAR`
- **Rationale**: Intuitive for end users - they write what they want to see. Minimizes config overhead.
- **Alternatives**:
  - Separate formatting modifiers (`{monat|uppercase}`) would add syntax complexity
  - Fixed case per language would remove user flexibility

**3. Numeric Placeholders Always Two Digits**
- **Decision**: `{monthNumber}` and `{dayNumber}` always produce zero-padded two-digit output (e.g., `02`, `07`, `15`)
- **Rationale**: Standard for photo organization; non-configurable simplifies logic
- **Trade-off**: Users cannot have `2` or `7` (single digit); must use `02`, `07`

**4. Token Validation with String Replacement**
- **Decision**: Extend current `strings.NewReplacer` approach to validate tokens before replacement
- **Rationale**: Keeps logic simple, reuses existing pattern. Validation catches typos in targetStructure
- **Alternative**: Use regex could be more complex; NewReplacer is sufficient

**5. Language Token Inventory**
- **Decision**: Define all 16 month/day tokens upfront (4 month variants + 4 day variants):
  - Month: `{month}`, `{monat}`, `{mois}`, `{mese}`
  - MonthNumber: `{monthNumber}` (all languages)
  - Day: `{day}`, `{tag}`, `{jour}`, `{giorno}`
  - DayNumber: `{dayNumber}` (all languages)
  - Year: `{year}`, `{jahr}`, `{année}`, `{anno}`
- **Rationale**: Explicit, validates unknown tokens, allows future extension
- **Trade-off**: Fixed token set; adding new languages requires code change

**6. Multilingual Month/Day Maps**
- **Decision**: Create separate lookup maps for each language (German, English, French, Italian) mapped by month/day
- **Rationale**: Clean separation, easy to extend, supports case transformation
- **Structure**:
  ```go
  monthNameDE, monthNameEN, monthNameFR, monthNameIT // maps[time.Month]string
  dayNameDE, dayNameEN, dayNameFR, dayNameIT         // maps[time.Weekday]string
  ```

## Risks / Trade-offs

**[Risk] User Mistakes with Mixed Languages**
- Mitigation: Clear documentation with examples showing recommended vs allowed patterns (e.g., mixing `{monat}` and `{day}` is allowed but may look odd)

**[Risk] Weekday Names Not Supported in V1**
- Rationale: Out of scope; month/day names solve the core use case. Weekdays can be added in v2 without breaking changes

**[Risk] Token String Length Variability**
- German: `Januar` (6), `Februar` (7), `März` (5), `September` (9)
- French: `janvier` (7), `février` (7), `décembre` (8)
- Italian: `gennaio` (7), `febbraio` (8), `settembre` (9)
- Mitigation: Document that folder names will have varying lengths; users should be aware when planning path structure

**[Trade-off] No Configuration Simplification**
- Keeping language implicit in token choice means no single `language` field, but users must remember token names per organizer
- Mitigation: Config examples in documentation will be comprehensive

## Migration Plan

- No breaking changes - new tokens are additive
- Existing configs using `{year}/{month}` continue to work unchanged
- Gradual rollout: v1 adds all 16 tokens; users opt into multilingual paths voluntarily

## Open Questions

- Should error messages mention "unrecognized token" for tokens that don't match any known placeholder?
- Should we normalize `{Month}` (different case) to `{month}` or reject it as unknown?
- For future: Weekday names - should we reserve `{weekday}`, `{wochentag}`, etc. now or add later?

