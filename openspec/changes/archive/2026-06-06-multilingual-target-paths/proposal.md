## Why

Phorga currently organizes photos into target folders rigid using month numbers (e.g., `2024/02`). Many users organize their photos in natural language folders with month names in their preferred language (German, French, Italian, or English). This feature enables users to define target paths using multilingual tokens without losing the ability to sort and organize by numeric formats.

## What Changes

- Add support for language-specific placeholders in `targetStructure`: `{monat}` (German), `{mois}` (French), `{mese}` (Italian), in addition to the existing `{month}` (English)
- Add numeric placeholders for month and day: `{monthNumber}`, `{dayNumber}` (always two digits, zero-padded)
- Support case-sensitive placeholders: `{monat}` → `februar`, `{Monat}` → `Februar`, `{MONAT}` → `FEBRUAR`
- Allow free formatting with separators: `{year}/{monthNumber}-{Monat}`, `{year}_{Monat}`, etc.
- Extend date-based organization to support natural language folder structures while maintaining numeric precision

## Capabilities

### New Capabilities

<!-- None - this is an extension to existing functionality -->

### Modified Capabilities

- `photo-organizer`: targetStructure resolution now supports multilingual month/day tokens with case-sensitive formatting and numeric alternatives

## Impact

- **Organizer package** (`internal/organizer/`): Extend targetStructure parsing to recognize language-specific tokens
- **Metadata package** (`internal/metadata/targetpath.go`): Add language-aware placeholder resolution
- **Configuration** (`config.yaml`): No config changes needed - language is inferred from token choice
- **Tests**: Add test cases for all supported languages and token combinations
- **Documentation** (`README.md`, `arc42.adoc`, `phorga.md`): Document new placeholders and language support

