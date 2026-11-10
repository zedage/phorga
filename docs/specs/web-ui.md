# Spec: Web UI (`WEB`)

> Human explanation: [product.md › Web UI](../product.md#web-ui) · Architecture: [arc42 §6 Web UI Request, §8 Configuration UI](../arc42/arc42.adoc) · Decisions: [ADR-0008](../decisions/0008-built-in-server-rendered-web-ui.md), [ADR-0009](../decisions/0009-ui-save-destination-and-no-hot-reload.md) · Format: [README](README.md)

---

### WEB-001 Optional, built into the same process
**Status:** implemented

The web UI MUST be off by default. It MUST start in the same process as the scheduler when either `webUI.enabled: true` is set or the `--web-ui` flag is given. `--web-ui-addr` overrides the listen address; otherwise `webUI.address` or `127.0.0.1:8080` is used. On SIGINT/SIGTERM it MUST shut down gracefully together with the scheduler. If the HTTP server fails, the process MUST exit with a non-zero status.

**Code:** `cmd/phorga/main.go`, `internal/webui/server.go`
**Tests:** `internal/webui/address_test.go#TestNewAddressPrecedence`
**Gap:** The address precedence is tested. The enable/shutdown wiring in `main.go` is only verified manually.

### WEB-002 Server-rendered configuration page
**Status:** implemented

`GET /` MUST redirect to `/ui`. `GET /ui` MUST render an HTML page showing the configuration **loaded at startup** as an editable form, with sections for general settings (`stateDir`, web UI), organizer profiles with their sources, and duplicate detection jobs. Other HTTP methods on `/ui` MUST return 405.

- **WHEN** the UI is opened after startup with a valid config
  **THEN** the page shows the loaded values (e.g. the organizer `target`)

**Code:** `internal/webui/server.go`, `internal/webui/model.go`, `internal/webui/templates/page.html`
**Tests:** `internal/webui/server_test.go#TestMainPageRendersCurrentConfiguration`

### WEB-003 Validate via HTMX fragments
**Status:** implemented

`POST /ui/validate` MUST convert the form into a configuration, validate it with the same rules as startup (CFG-002), and return the result. For requests with `HX-Request: true`, the result MUST be an HTML fragment (`flash.html`) instead of a full page. It MUST NOT write anything.

- **WHEN** an invalid form is submitted with `HX-Request: true`
  **THEN** the response is an error fragment listing the problems, and no file is written

**Code:** `internal/webui/server.go`, `internal/webui/templates/flash.html`
**Tests:** `internal/webui/server_test.go#TestValidateReturnsHTMXErrorFragment`

### WEB-004 Save to active or separate file
**Status:** implemented

`POST /ui/save` MUST validate as in WEB-003 and, only if the form is valid, write the configuration (CFG-004) to:
- the active config file when `saveTarget` is not `separate`
- `separatePath` when `saveTarget=separate` (this field is then required)

On success it MUST report the written path and send the header `HX-Trigger: {"configSaved":true}`. The running process does not reload (CFG-005).

- **WHEN** `saveTarget=separate` and `separatePath=/tmp/new.yaml`
  **THEN** `/tmp/new.yaml` is written, the active config is unchanged, and the response names `/tmp/new.yaml`
- **WHEN** the form is invalid
  **THEN** nothing is written

**Code:** `internal/webui/server.go`, `internal/config/saver.go`
**Tests:** `internal/webui/server_test.go#TestSaveWritesSeparateFile`, `internal/webui/server_test.go#TestSaveWritesActiveConfigFile`

### WEB-005 Localization (de, en, it, fr)
**Status:** implemented

All UI text MUST come from server-side message catalogs for `de`, `en`, `it` and `fr`. The language is chosen from (1) the `lang` query/form parameter, (2) the `phorga_lang` cookie, (3) `Accept-Language`, or (4) English as the default. The chosen language is stored in the `phorga_lang` cookie. A missing key falls back to English, then to the key itself.

- **WHEN** `/ui?lang=fr` is requested
  **THEN** the heading is `Configuration Web Phorga`
- **WHEN** `/ui?lang=es` is requested
  **THEN** the English UI is shown

**Code:** `internal/webui/i18n.go`, `internal/webui/server.go`
**Tests:** `internal/webui/server_test.go#TestLocalizationUsesRequestedLanguageAndFallback`

### WEB-006 Local/private use only
**Status:** implemented

The UI has no authentication. Its documentation MUST state that it is meant for local or trusted networks only. The default address MUST bind to loopback (`127.0.0.1`).

**Code:** `internal/config/loader.go`
**Tests:** `internal/config/defaults_test.go#TestLoadAppliesDefaults`, `internal/webui/address_test.go#TestNewAddressPrecedence`
**Notes:** The page loads HTMX from `unpkg.com`. Without internet access, the forms still work as plain POSTs and return full pages.
