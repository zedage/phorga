## ADDED Requirements

### Requirement: YAML configuration support
The system SHALL accept a YAML configuration file for all settings.

#### Scenario: Valid YAML config
- **WHEN** a valid YAML config file is provided at startup
- **THEN** the system SHALL parse it and start successfully

#### Scenario: JSON configuration support
- **WHEN** a valid JSON config file is provided at startup
- **THEN** the system SHALL parse it and start successfully

### Requirement: Fail-fast configuration validation
The system SHALL validate the entire configuration at startup and fail immediately on invalid values.

#### Scenario: Missing required field
- **WHEN** `target` is missing from an organizer profile
- **THEN** the system SHALL exit with a non-zero status and a descriptive error message

#### Scenario: Invalid cron expression
- **WHEN** a `schedule` value is not a valid 5-field cron expression
- **THEN** the system SHALL exit with a non-zero status and identify the invalid expression

#### Scenario: Invalid mode value
- **WHEN** `mode` is set to something other than `move` or `copy`
- **THEN** the system SHALL exit with a non-zero status

#### Scenario: moveToFolder without duplicateFolder
- **WHEN** `action.method` is `moveToFolder` but `duplicateFolder` is not set
- **THEN** the system SHALL exit with a non-zero status

#### Scenario: Invalid targetStructure token
- **WHEN** `targetStructure` contains an unknown placeholder
- **THEN** the system SHALL exit with a non-zero status

### Requirement: Config file path resolution
The system SHALL accept the config file path via CLI argument or environment variable.

#### Scenario: CLI argument
- **WHEN** the binary is started with `--config /path/to/config.yaml`
- **THEN** the system SHALL use that file

#### Scenario: Environment variable fallback
- **WHEN** no `--config` argument is given but `PHORGA_CONFIG` is set
- **THEN** the system SHALL use the path from the environment variable

#### Scenario: Default path
- **WHEN** neither CLI argument nor environment variable is set
- **THEN** the system SHALL look for `config.yaml` in the current working directory

### Requirement: Effective configuration can be displayed in the UI
The system SHALL expose the currently effective configuration values to the web UI for read-only display before changes are saved.

#### Scenario: Current config values are shown in UI
- **WHEN** the web UI loads after a valid configuration has been read at startup
- **THEN** the system SHALL display the effective current values from the loaded configuration model

### Requirement: UI-submitted configuration is validated before save
The system SHALL validate configuration changes submitted through the web UI before writing them to disk.

#### Scenario: Invalid UI update is rejected
- **WHEN** a user submits configuration changes that violate existing config rules
- **THEN** the system SHALL reject the save and return validation feedback without writing a configuration file

#### Scenario: Valid UI update passes validation
- **WHEN** a user submits configuration changes that satisfy existing config rules
- **THEN** the system SHALL accept the configuration for persistence

### Requirement: Configuration can be saved to an alternate file
The system SHALL support saving validated configuration to either the active config file or a separate file selected by the user.

#### Scenario: Save to primary config file
- **WHEN** the user selects the primary loaded config file as the save target
- **THEN** the system SHALL overwrite that file with the validated configuration using the supported config format

#### Scenario: Save to separate config file
- **WHEN** the user selects a separate config file path as the save target
- **THEN** the system SHALL write the validated configuration to the separate file without requiring the active config path to change immediately

