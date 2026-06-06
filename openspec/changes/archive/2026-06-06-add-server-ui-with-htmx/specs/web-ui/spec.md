## ADDED Requirements

### Requirement: Server-rendered configuration UI
The system SHALL provide an optional server-rendered web UI for viewing and editing Phorga configuration.

#### Scenario: UI main page shows configuration sections
- **WHEN** the web UI is enabled and a user opens the configuration page
- **THEN** the system SHALL render a browser-accessible HTML page that shows the current configuration sections for organizer, duplicate detection, notifications, and state settings

#### Scenario: HTMX is used for interactive updates
- **WHEN** a user submits configuration changes or requests a partial update from the UI
- **THEN** the system SHALL respond with server-rendered HTML fragments suitable for HTMX-driven updates without requiring a full SPA frontend

### Requirement: Multilingual UI localization
The system SHALL provide the configuration UI in German, English, Italian, and French.

#### Scenario: German UI rendering
- **WHEN** the selected UI language is German
- **THEN** the system SHALL render labels, actions, and validation messages in German

#### Scenario: French UI rendering
- **WHEN** the selected UI language is French
- **THEN** the system SHALL render labels, actions, and validation messages in French

#### Scenario: Unsupported language fallback
- **WHEN** a request asks for an unsupported UI language
- **THEN** the system SHALL fall back to a supported default language and continue rendering the UI successfully

### Requirement: UI save destination selection
The system SHALL allow users to choose the configuration file destination when saving UI changes.

#### Scenario: Save to active config file
- **WHEN** a user edits configuration in the UI and selects the active config file as the save destination
- **THEN** the system SHALL persist the validated configuration to the active config file path

#### Scenario: Save to separate config file
- **WHEN** a user edits configuration in the UI and selects a separate output file path
- **THEN** the system SHALL persist the validated configuration to that separate file path and report the location back to the user

