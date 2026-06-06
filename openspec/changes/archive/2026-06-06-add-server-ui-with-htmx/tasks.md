## 1. Runtime and HTTP Server Setup

- [x] 1.1 Decide how the web UI is enabled in startup flow (`config`, CLI flag, or both)
- [x] 1.2 Add runtime settings for HTTP bind address and port
- [x] 1.3 Start the optional HTTP server from `cmd/phorga/main.go`
- [x] 1.4 Stop the HTTP server during graceful shutdown
- [x] 1.5 Verify existing headless scheduler behavior still works when the UI is disabled

## 2. Web UI Foundation

- [x] 2.1 Create a new internal package for HTTP routing, handlers, and template rendering
- [x] 2.2 Add the main configuration page handler
- [x] 2.3 Add HTMX-compatible endpoints for partial rendering and form submission
- [x] 2.4 Add a shared layout and reusable UI fragments for configuration sections
- [x] 2.5 Embed or package templates so the UI ships with the binary

## 3. Configuration Read and View Models

- [x] 3.1 Define view models for rendering the effective loaded configuration
- [x] 3.2 Map organizer profiles and nested sources into UI sections
- [x] 3.3 Map duplicate detection settings into UI sections
- [x] 3.4 Show the currently loaded config file path and effective values in the UI

## 4. Localization

- [x] 4.1 Define translation keys for labels, actions, help text, and validation messages
- [x] 4.2 Add German translations
- [x] 4.3 Add English translations
- [x] 4.4 Add Italian translations
- [x] 4.5 Add French translations
- [x] 4.6 Implement language selection and supported-language fallback behavior

## 5. Editing and Validation Flow

- [x] 5.1 Define form models for editable config sections
- [x] 5.2 Parse submitted form data into form models
- [x] 5.3 Convert valid form models back into `internal/config.Config`
- [x] 5.4 Reuse existing validation logic from `internal/config` for UI submissions
- [x] 5.5 Return field-level validation feedback through HTMX responses

## 6. Config Persistence

- [x] 6.1 Add structured YAML serialization support for config writes
- [x] 6.2 Add atomic file-write behavior for saved config files
- [x] 6.3 Implement save-to-active-config behavior
- [x] 6.4 Implement save-to-separate-file behavior
- [x] 6.5 Show a clear success response that includes the actual save destination

## 7. Testing

- [x] 7.1 Add handler tests for the main configuration page
- [x] 7.2 Add HTMX submission tests for valid changes
- [x] 7.3 Add HTMX submission tests for invalid changes
- [x] 7.4 Add config write/read roundtrip tests
- [x] 7.5 Add tests for alternate save-target behavior
- [x] 7.6 Add localization tests for `de`, `en`, `it`, and `fr`
- [x] 7.7 Add startup tests covering UI enabled and disabled modes

## 8. Documentation and Spec Sync

- [x] 8.1 Update `README.md` with web UI purpose, startup, and save behavior
- [x] 8.2 Update `docs/arc42/arc42.adoc` to describe the optional built-in web UI
- [x] 8.3 Update `openspec/specs/phorga.md` to reflect the optional UI capability
- [x] 8.4 Sync approved spec changes back into the main specs after implementation
- [x] 8.5 Document that the UI is intended for local/private deployment unless additional protection is added

## 9. Verification

- [x] 9.1 Verify the UI displays the current effective configuration values
- [x] 9.2 Verify saving to the active config file updates the expected file
- [x] 9.3 Verify saving to a separate file writes the expected config output
- [x] 9.4 Verify all four UI languages render correctly
- [x] 9.5 Verify Phorga still runs correctly without the UI enabled

