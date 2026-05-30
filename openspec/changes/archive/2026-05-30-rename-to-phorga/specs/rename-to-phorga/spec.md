## ADDED Requirements

### Requirement: Project uses Phorga as its name

The project SHALL use "Phorga" (title case) and "phorga" (lowercase) as its name in all code, configuration, documentation, Docker artifacts, and binary output. No references to the previous name "Phorganizer" / "phorganizer" SHALL remain.

#### Scenario: Go module path uses phorga
- **WHEN** inspecting `go.mod`
- **THEN** the module path MUST be `github.com/zedage/phorga`

#### Scenario: Binary is named phorga
- **WHEN** building the project with `go build ./cmd/phorga`
- **THEN** the output binary MUST be named `phorga` (or `phorga.exe` on Windows)

#### Scenario: All import paths use phorga
- **WHEN** searching the codebase for `phorganizer`
- **THEN** zero matches SHALL be found

#### Scenario: Docker image uses phorga
- **WHEN** building the Docker image
- **THEN** the image name and references MUST use `phorga`

#### Scenario: Environment variable uses PHORGA prefix
- **WHEN** configuring via environment variables
- **THEN** the prefix MUST be `PHORGA_` (e.g., `PHORGA_CONFIG`)

