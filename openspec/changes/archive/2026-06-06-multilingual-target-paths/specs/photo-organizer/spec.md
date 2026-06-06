## ADDED Requirements

### Requirement: Multilingual target structure with language-specific tokens
The system SHALL support language-specific month and day name tokens in `targetStructure` for German, French, and Italian, in addition to English.

#### Scenario: German month name token
- **WHEN** `targetStructure` is `{year}/{Monat}` and a photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/Februar`

#### Scenario: French month name token
- **WHEN** `targetStructure` is `{year}/{Mois}` and a photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/Février`

#### Scenario: Italian day name token
- **WHEN** `targetStructure` is `{year}/{mese}/{giorno}` and a photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/febbraio/15`

#### Scenario: Case-sensitive month name rendering
- **WHEN** `targetStructure` is `{year}/{monat}` (lowercase m) and photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/februar` (all lowercase)

#### Scenario: Case-sensitive month name with capital first letter
- **WHEN** `targetStructure` is `{year}/{Monat}` (capital M) and photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/Februar` (first letter capitalized)

### Requirement: Numeric month and day placeholders
The system SHALL support numeric placeholder tokens that render dates as zero-padded two-digit numbers.

#### Scenario: Month number placeholder
- **WHEN** `targetStructure` is `{year}/{monthNumber}` and photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/02`

#### Scenario: Day number placeholder
- **WHEN** `targetStructure` is `{year}/{monthNumber}/{dayNumber}` and photo date is 2024-02-05
- **THEN** the file SHALL be placed in `2024/02/05`

#### Scenario: Numeric placeholders are always two digits
- **WHEN** processing dates with single-digit day (e.g., 2024-02-05) or single-digit month (if applicable)
- **THEN** the dayNumber and monthNumber SHALL be zero-padded (e.g., `05`)

### Requirement: Language-specific year token support
The system SHALL support language-specific year tokens in addition to English `{year}`.

#### Scenario: German year token
- **WHEN** `targetStructure` is `{jahr}/{monat}` and photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/februar`

#### Scenario: French year token
- **WHEN** `targetStructure` is `{année}/{mois}` and photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/février`

#### Scenario: Italian year token
- **WHEN** `targetStructure` is `{anno}/{mese}` and photo date is 2024-02-15
- **THEN** the file SHALL be placed in `2024/febbraio`

## MODIFIED Requirements

### Requirement: Target structure configuration
The system SHALL support a configurable `targetStructure` with placeholder tokens (English and multilingual).

#### Scenario: Default target structure
- **WHEN** `targetStructure` is not specified
- **THEN** the system SHALL use `{year}/{month}` as the default

#### Scenario: Custom target structure with day
- **WHEN** `targetStructure` is `{year}/{monthWithName}/{day}`
- **THEN** a photo from 2024-01-15 SHALL be placed in `2024/01-january/15`

#### Scenario: Multilingual German target structure
- **WHEN** `targetStructure` is `{year}/{Monat}/{Tag}`
- **THEN** a photo from 2024-01-15 SHALL be placed in `2024/Januar/15`

#### Scenario: Mixed token formats
- **WHEN** `targetStructure` is `{year}/{monthNumber}-{Monat}/{dayNumber}`
- **THEN** a photo from 2024-02-05 SHALL be placed in `2024/02-Februar/05`

#### Scenario: Invalid token in target structure
- **WHEN** `targetStructure` contains an unknown token like `{invalid}`
- **THEN** configuration validation SHALL fail at startup

#### Scenario: Unrecognized language token
- **WHEN** `targetStructure` contains a language token from an unsupported language (e.g., `{month_nl}`)
- **THEN** configuration validation SHALL fail at startup with an error indicating unknown token

