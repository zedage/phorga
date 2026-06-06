## Why

Phorga is currently configured only through YAML or JSON files. That keeps the runtime simple, but it makes setup and updates unnecessarily hard for users who run Phorga on a NAS or server and want a small browser-based administration page.

A simple server-rendered UI using HTMX would make configuration easier without introducing a heavy frontend stack. It should show the current effective configuration, support German, English, Italian, and French, and let users decide whether changes are saved back to `config.yaml` or to a separate file.

## What Changes

- Add an optional built-in web UI served by the Phorga process
- Render the UI server-side and use HTMX for lightweight partial updates and form submissions
- Show the current effective configuration values from the loaded config model
- Allow users to edit and validate configuration through the UI
- Let users choose whether changes are written to `config.yaml` or to a separate file
- Add UI localization for German, English, Italian, and French
- Document the new UI mode, save behavior, and deployment expectations for private/local server usage

## Capabilities

### New Capabilities
- `web-ui`: browser-based configuration interface for viewing, editing, validating, and saving Phorga settings

### Modified Capabilities
- `config-management`: configuration can be displayed, edited, validated, and saved through the web UI in addition to file-based editing
- `phorga`: the runtime can optionally expose a built-in HTTP interface for configuration management

## Impact

- **Entry point**: `cmd/phorga/main.go` must start and stop the optional HTTP server alongside the existing scheduler lifecycle
- **Configuration**: `internal/config/` must expose effective values for rendering and support writing updated config to a selected output file
- **New web/UI modules**: HTTP handlers, templates, HTMX endpoints, and localization support must be added
- **Documentation**: `README.md`, `docs/arc42/arc42.adoc`, and `openspec/specs/phorga.md` must be updated because the system is no longer strictly headless when the UI is enabled
- **Tests**: handler, persistence, validation, and localization coverage must be added

