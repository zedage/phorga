## ADDED Requirements

### Requirement: Optional ntfy.sh notifications
The system SHALL send notifications via ntfy.sh when a `ntfy.url` is configured.

#### Scenario: Run summary on success
- **WHEN** an organizer or duplicate detection run completes successfully and `ntfy.url` is set
- **THEN** the system SHALL send a summary notification including file count and action type

#### Scenario: Run summary on failure
- **WHEN** a run encounters errors and `ntfy.url` is set
- **THEN** the system SHALL send a notification including error count and affected files

#### Scenario: No ntfy configured
- **WHEN** `ntfy.url` is not set
- **THEN** no notification SHALL be sent and no error SHALL occur

### Requirement: Notification failure resilience
Notification delivery failures SHALL NOT affect the organizer or duplicate detection run.

#### Scenario: ntfy.sh unreachable
- **WHEN** the ntfy.sh endpoint is unreachable during a notification attempt
- **THEN** the system SHALL log a warning and continue the run without retrying

