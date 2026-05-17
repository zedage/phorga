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

