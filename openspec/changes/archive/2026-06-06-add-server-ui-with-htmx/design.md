## Context

Phorga currently starts from `cmd/phorga/main.go`, resolves and loads a YAML/JSON config via `internal/config`, validates it, and then runs organizer and duplicate-detection jobs on a schedule. The current architecture and documentation describe the system as headless and file-configured.

This change introduces a small browser-based UI running on the server and implemented with server-rendered HTML plus HTMX. The UI must display the currently effective configuration values, support German, English, Italian, and French, and allow users to save edited settings either back to `config.yaml` or to a separate file.

The design must preserve Phorga's simplicity: one binary, no SPA frontend, no mandatory extra service, and no loss of the existing file-based configuration workflow.

## Goals / Non-Goals

**Goals:**
- Add an optional built-in HTTP server for configuration management
- Use server-rendered HTML templates with HTMX for lightweight interactions
- Display the current effective configuration values from the loaded config model
- Allow editing, validation, and persistence of configuration through the UI
- Support UI localization for German, English, Italian, and French
- Let the user choose whether to save changes to `config.yaml` or to a separate file
- Keep existing headless startup behavior intact when the UI is disabled

**Non-Goals:**
- Building a SPA or introducing a frontend build pipeline
- Adding public-internet-ready authentication and authorization in this change
- Adding operational dashboards, job history, or live scheduler control
- Replacing file-based config as the primary source of truth
- Supporting every future config extension on day one

## Decisions

### 1. Built-in HTTP server in the existing binary
- **Decision**: Extend the existing `phorga` process with an optional HTTP server instead of creating a separate admin service.
- **Rationale**: One binary keeps deployment simple for Docker, NAS, and direct local execution, and it keeps configuration and runtime behavior in one place.
- **Alternative considered**: A separate admin binary or sidecar service. Rejected because it adds operational complexity and splits configuration logic.

### 2. Server-rendered templates plus HTMX
- **Decision**: Render HTML on the server and use HTMX for partial page updates and form submissions.
- **Rationale**: This satisfies the HTMX requirement while avoiding a JavaScript-heavy frontend stack and keeping the codebase aligned with Go's strengths.
- **Alternative considered**: SPA with JSON API. Rejected as too heavy for the requested scope.

### 3. Separate effective-config display from save destination
- **Decision**: The UI will always show the currently loaded effective configuration, but saving requires an explicit destination choice: overwrite the active config file or write a separate file.
- **Rationale**: The user explicitly wants a choice of save target. Separating “what is loaded now” from “where to save” avoids ambiguity.
- **Alternative considered**: Always overwrite the loaded config. Rejected because it removes the requested flexibility.

### 4. Add structured config serialization for writes
- **Decision**: Extend `internal/config` with explicit save/export support for structured YAML output and atomic file writes.
- **Rationale**: The current config package loads and validates configuration but does not provide a stable write path. Structured serialization is safer than patching text files.
- **Alternative considered**: Line-based or text-template rewriting of the original config file. Rejected because it is brittle and hard to validate.

### 5. Introduce a form/view-model layer
- **Decision**: Use dedicated form models between HTML form data and `internal/config.Config`.
- **Rationale**: Nested config structures, optional sections, and localized validation feedback are easier to manage when the domain structs are not bound directly to HTTP form parsing.
- **Alternative considered**: Binding forms directly to config structs. Rejected because it makes validation, partial updates, and UI error reporting harder.

### 6. Server-side localization with message catalogs
- **Decision**: Keep translations in server-side message catalogs keyed by symbolic message IDs, with support for `de`, `en`, `it`, and `fr`.
- **Rationale**: This keeps templates reusable across languages and avoids duplication of template files per locale.
- **Alternative considered**: Separate template per language. Rejected because it creates unnecessary duplication.

### 7. Private/local deployment assumption for v1
- **Decision**: Scope the first version of the UI for local or trusted-network use and document that it should not be exposed publicly without additional protection.
- **Rationale**: Security hardening and authentication would significantly expand the scope beyond the requested simple configuration UI.
- **Alternative considered**: Mandatory authentication in the first change. Deferred to keep the first iteration small and implementable.

## Risks / Trade-offs

- **[Risk] UI concerns couple too tightly with runtime startup** → Keep HTTP concerns isolated in a dedicated package and keep scheduler logic unchanged when UI is disabled.
- **[Risk] Saving configuration rewrites formatting or removes comments** → Use structured serialization and document that UI saves preserve config semantics, not original comments/layout.
- **[Risk] Users may confuse effective runtime config with a newly saved separate file** → Clearly label the current loaded file, selected save target, and whether a restart or different startup path is needed.
- **[Risk] No authentication in the first version** → Bind to local/private interfaces by configuration and document deployment expectations prominently.
- **[Risk] Nested config forms become unwieldy** → Group the UI into sections and use HTMX partials for focused edits and validation feedback.

## Migration Plan

- Introduce the HTTP server as an optional feature so existing headless usage remains unchanged
- Keep `--config` and current file-based config resolution behavior intact
- Add config write support behind the UI workflow without requiring changes for existing users
- Update documentation to explain the new optional UI mode, save behavior, and local/private deployment expectations
- Roll back by disabling the UI mode; file-based startup continues to work as before

## Open Questions

- Should the UI be enabled through config, CLI flag, or both?
- Should separate-file saves support only YAML in v1, or also JSON?
- Should a successful save offer config reload/apply behavior, or only persist for next startup?
- How should the selected language persist across requests: query parameter, cookie, or both?

