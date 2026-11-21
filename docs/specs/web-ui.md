# Spec: Web UI (`WEB`)

> Human explanation: [product.md › Web UI](../product.md#web-ui) · Architecture: [arc42 §6 Web UI Request, §8 Configuration UI](../arc42/arc42.adoc) · Decisions: [ADR-0008](../decisions/0008-built-in-server-rendered-web-ui.md), [ADR-0009](../decisions/0009-ui-save-destination-and-no-hot-reload.md), [ADR-0013](../decisions/0013-folder-picker-limited-to-browse-roots.md), [ADR-0014](../decisions/0014-persisted-run-status-without-dry-runs.md) · Format: [README](README.md)

---

### WEB-001 Optional, built into the same process
**Status:** implemented

The web UI MUST be off by default. It MUST start in the same process as the scheduler when either `webUI.enabled: true` is set or the `--web-ui` flag is given. `--web-ui-addr` overrides the listen address; otherwise `webUI.address` or `127.0.0.1:8080` is used. On SIGINT/SIGTERM it MUST shut down gracefully together with the scheduler. If the HTTP server fails (e.g. the port is taken), the process MUST exit with a non-zero status.

- **WHEN** `--web-ui --web-ui-addr 127.0.0.1:9090` is given and the config has no `webUI` block
  **THEN** `GET http://127.0.0.1:9090/ui` answers 200, and after shutdown the port is closed and the exit code is 0
- **WHEN** `webUI.address` is set but `webUI.enabled` is `false` and `--web-ui` is not given
  **THEN** nothing listens on that address
- **WHEN** the UI address is already in use
  **THEN** the process exits with status 1

**Code:** `cmd/phorga/main.go`, `internal/webui/server.go`
**Tests:** `internal/webui/address_test.go#TestNewAddressPrecedence`, `cmd/phorga/main_test.go#TestRun_WebUIEnabledByFlag`, `cmd/phorga/main_test.go#TestRun_WebUIEnabledByConfig`, `cmd/phorga/main_test.go#TestRun_WebUIOffByDefault`, `cmd/phorga/main_test.go#TestRun_WebUIListenFailureExitsNonZero`

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

All UI text MUST come from server-side message catalogs for `de`, `en`, `it` and `fr`. The language is chosen from (1) the `lang` query/form parameter, (2) the `phorga_lang` cookie, (3) `Accept-Language`, or (4) English as the default. The chosen language is stored in the `phorga_lang` cookie. A missing key falls back to English, then to the key itself. The top bar of every UI page MUST offer a language picker with the current language selected; choosing another one reloads the page with `?lang=<code>`.

- **WHEN** `/ui?lang=fr` is requested
  **THEN** the heading is `Configuration Web Phorga`
- **WHEN** `/ui?lang=es` is requested
  **THEN** the English UI is shown
- **WHEN** `/ui/status?lang=it` is requested
  **THEN** the top bar shows the language picker with `Italiano` selected

**Code:** `internal/webui/i18n.go`, `internal/webui/server.go`, `internal/webui/templates/layout.html`
**Tests:** `internal/webui/server_test.go#TestLocalizationUsesRequestedLanguageAndFallback`, `internal/webui/usability_test.go#TestLanguagePickerOnEveryPage`

### WEB-006 Local/private use only
**Status:** implemented

The UI has no authentication. Its documentation MUST state that it is meant for local or trusted networks only. The default address MUST bind to loopback (`127.0.0.1`).

**Code:** `internal/config/loader.go`
**Tests:** `internal/config/defaults_test.go#TestLoadAppliesDefaults`, `internal/webui/address_test.go#TestNewAddressPrecedence`
**Notes:** In Docker, the in-container address must be `0.0.0.0:8080`, and the loopback restriction moves to the host port mapping (`127.0.0.1:8080:8080`). README, `docker-compose.yml` and `config.example.yaml` document this. HTMX is embedded in the binary (WEB-009), so the UI needs no internet access. Without JavaScript, the forms still work as plain POSTs and return full pages. The folder picker (WEB-010) can list directories, so its roots should be limited to the media folders.

### WEB-007 Choice fields and help texts
**Status:** implemented

Fields with a fixed set of values MUST be rendered as `<select>`. The allowed values MUST come from the same lists the validation uses (CFG-002):
- organizer source `mode`: `copy`, `move`
- organizer source duplicate `actionMethod`: `addSuffix`, `moveToFolder`, `replace`, `skip`
- duplicate detection `detectionMode`: `content`, `metadata`
- duplicate detection `actionMethod`: `moveToFolder`, `skip`

Each select also offers an empty "not set" option. A value from the file that is not in the list MUST stay selected, shown as `<value> (unknown)`, so validation can report it. The organizer duplicate `detectionMode` stays a text field, because it combines fields with `+` or `|`. Every editable field MUST show a short help text from the message catalogs (WEB-005), linked with `aria-describedby`.

- **WHEN** a source has `mode: copy`
  **THEN** the mode select has `copy` selected and also offers `move`
- **WHEN** a dedup job has `action.method: explode`
  **THEN** the select shows `explode (unknown)` as the selected option

**Code:** `internal/webui/templates/partials.html`, `internal/webui/templates/page.html`, `internal/webui/model.go`, `internal/webui/i18n_ui.go`, `internal/config/validate.go`
**Tests:** `internal/webui/usability_test.go#TestPageRendersSelectsWithCurrentValue`, `internal/webui/usability_test.go#TestUIMessagesExistInAllLanguages`, `internal/webui/usability_test.go#TestPageUsesOnlyKnownKeys`, `internal/config/browseroots_test.go#TestEnumAccessorsMatchValidation`

### WEB-008 Live previews and field errors
**Status:** implemented

While the user types (HTMX, 300 ms delay) and on page load, the UI MUST show:
- for each `targetStructure`, the path it produces for the fixed sample date 2024-03-15 (`POST /ui/preview/structure?field=<name>`). An empty value previews the default `{year}/{month}`. An invalid structure shows the validation error.
- for each `schedule`, the next 3 run times in server time (`POST /ui/preview/schedule?field=<name>`), parsed with the scheduler's 5-field rules. An invalid expression shows the parse error.

Previews MUST NOT write anything. When validate or save (WEB-003, WEB-004) finds problems, each problem that belongs to one field MUST also be shown next to that field: as out-of-band HTMX swaps (`hx-swap-oob`) in fragment responses, and inline in full-page responses. The summary list at the top stays.

- **WHEN** `targetStructure` is `{year}/{month}/{day}`
  **THEN** the preview shows `2024/03/15`
- **WHEN** `schedule` is `0 2 * * *` and it is 2026-11-11 10:00
  **THEN** the preview shows 2026-11-12, 2026-11-13 and 2026-11-14 at 02:00
- **WHEN** an HTMX validate request has an empty `organizer.0.target`
  **THEN** the response contains `id="err-f-organizer-0-target" hx-swap-oob="true"` with the message `organizer[0].target is required`

**Code:** `internal/webui/preview.go`, `internal/webui/model.go`, `internal/webui/server.go`, `internal/webui/templates/partials.html`, `internal/scheduler/scheduler.go`, `internal/config/validate.go`
**Tests:** `internal/webui/usability_test.go#TestPreviewStructure`, `internal/webui/usability_test.go#TestPreviewSchedule`, `internal/webui/usability_test.go#TestValidateReturnsFieldErrorsOutOfBand`, `internal/webui/usability_test.go#TestFieldForProblem`, `internal/scheduler/scheduler_test.go#TestNextTimes`

### WEB-009 Embedded assets
**Status:** implemented

HTMX (1.9.12) and the stylesheet MUST be embedded in the binary and served from `/ui/static/htmx.min.js` and `/ui/static/phorga.css` with a `Cache-Control` header. UI pages MUST NOT load scripts, styles or fonts from external hosts.

- **WHEN** `/ui` or `/ui/status` is rendered
  **THEN** it loads `/ui/static/htmx.min.js` and contains no `unpkg.com` reference

**Code:** `internal/webui/server.go`, `internal/webui/static/htmx.min.js`, `internal/webui/static/phorga.css`, `internal/webui/templates/layout.html`
**Tests:** `internal/webui/usability_test.go#TestHTMXIsServedFromBinary`

### WEB-010 Folder picker
**Status:** implemented

When `webUI.browseRoots` (CFG-006) is set, every path field MUST offer autocomplete suggestions and a "Browse…" button that opens a folder dialog. The picker MUST:
- list only directories, sorted by name, without hidden (dot) folders, and only below one of the roots after `..` cleanup and symlink resolution
- answer `403` for `GET /ui/fs/list?path=<p>` when `<p>` is relative, not a directory, or outside the roots
- for `GET /ui/fs/list?start=<field value>`, open the nearest existing allowed folder, or the list of roots
- for `GET /ui/fs/suggest?field=<name>`, return at most 20 `<option>` completions of the typed path
- never create, change or delete anything on disk
- use the roots loaded at startup (CFG-005)

When `browseRoots` is not set, the picker controls MUST be hidden and both endpoints MUST answer `404`.

- **WHEN** the root contains `albums/`, `archive/`, `.hidden/` and `notes.txt`
  **THEN** the listing shows `albums` and `archive` only
- **WHEN** `path` is `<root>/albums/../..` or a symlink inside the root that points outside
  **THEN** the response is `403`
- **WHEN** the field value is `<root>/albums/2025/new`, which does not exist yet
  **THEN** the dialog opens at `<root>/albums`

**Code:** `internal/webui/fsbrowse.go`, `internal/webui/templates/partials.html`, `internal/webui/templates/page.html`
**Tests:** `internal/webui/fsbrowse_test.go#TestPickerListsOnlyVisibleSubfolders`, `internal/webui/fsbrowse_test.go#TestPickerRejectsPathsOutsideRoots`, `internal/webui/fsbrowse_test.go#TestPickerStartsAtNearestAllowedFolder`, `internal/webui/fsbrowse_test.go#TestPickerSuggestCompletesPrefix`, `internal/webui/fsbrowse_test.go#TestPickerDisabledWithoutRoots`, `internal/webui/fsbrowse_test.go#TestPickerNeverWrites`
**Notes:** The symlink case is skipped where the OS does not allow creating symlinks (Windows without developer mode). See [ADR-0013](../decisions/0013-folder-picker-limited-to-browse-roots.md).

### WEB-011 Status page
**Status:** implemented

`GET /ui/status` MUST show one row per registered job (each organizer source, each enabled dedup job) with: kind and label (`<source> → <target>`, or the dedup sources), cron schedule, next planned run, running/waiting/idle ("waiting for another job" while the job is due but another job runs, RUN-002), and the last run's start, duration, counters and a dry-run badge. Jobs that have not run show "not run yet". Above the table, the page MUST show how many jobs there are and how many are running, waiting and idle, counting each job once (running before waiting). The table and the summary MUST refresh themselves every 10 s from `GET /ui/status/table` (fragment). Both endpoints answer `405` for other methods.

The last result of each job MUST be recorded after every run. Results of non-dry runs MUST be persisted atomically in `<stateDir>/run-status.json` and loaded at startup. Results of dry runs MUST be kept in memory only, so a dry run never writes a file. A failure to write the status file MUST be logged and never fail the job.

- **WHEN** an organizer run (not dry) finishes and Phorga restarts
  **THEN** the status page still shows that run with its counters
- **WHEN** a dedup job with `dryRun: true` finishes
  **THEN** the page shows it with a "dry run" badge, and `run-status.json` is not created or changed
- **WHEN** a dedup job is due while an organizer run is busy
  **THEN** its row shows "waiting for another job"
- **WHEN** one job runs, one waits and two are idle
  **THEN** the summary shows 4 jobs, 1 running, 1 waiting and 2 idle

**Code:** `internal/runstatus/runstatus.go`, `internal/webui/status.go`, `internal/webui/templates/status.html`, `internal/scheduler/scheduler.go`, `internal/fileops/fileops.go`, `cmd/phorga/main.go`
**Tests:** `internal/runstatus/runstatus_test.go#TestFinishPersistsAndReloads`, `internal/runstatus/runstatus_test.go#TestDryRunIsNotWritten`, `internal/runstatus/runstatus_test.go#TestWriteErrorDoesNotPanic`, `internal/scheduler/scheduler_test.go#TestNextAndRunning`, `internal/webui/usability_test.go#TestStatusTableShowsJobs`, `internal/webui/usability_test.go#TestStatusTableShowsWaitingJob`, `internal/webui/usability_test.go#TestStatusPageWithoutSource`, `internal/webui/usability_test.go#TestStatusSummaryCountsEachJobOnce`, `cmd/phorga/status_test.go#TestStatusBoardReportsRecordedRun`, `cmd/phorga/status_test.go#TestRun_StatusPageIsServed`
**Notes:** See [ADR-0014](../decisions/0014-persisted-run-status-without-dry-runs.md).

### WEB-012 Theme follows system color scheme
**Status:** implemented

By default, all UI pages MUST follow the light or dark setting of the operating system or browser. A manual override is possible (WEB-013). Pages MUST declare `<meta name="color-scheme" content="light dark">` and load the embedded stylesheet (WEB-009). The stylesheet MUST define its colors as custom properties, declare `color-scheme: light dark` so native controls follow the theme, and override the colors in `@media (prefers-color-scheme: dark)`. Text and status colors (errors, success, badges) MUST stay readable in both themes.

- **WHEN** the system uses a dark theme and `/ui` or `/ui/status` is opened
  **THEN** the page, form controls and folder picker dialog are shown in dark colors
- **WHEN** `/ui/static/phorga.css` is requested
  **THEN** it answers 200 with `text/css` and contains a `prefers-color-scheme: dark` block

**Code:** `internal/webui/static/phorga.css`, `internal/webui/templates/layout.html`
**Tests:** `internal/webui/usability_test.go#TestThemeFollowsSystemColorScheme`

### WEB-013 Manual light/dark switch
**Status:** implemented

The top bar of every UI page MUST offer a switch with three choices: System (follow WEB-012), Light and Dark. The labels MUST come from the message catalogs (WEB-005). The choice MUST be stored in the browser only (`localStorage` key `phorga-theme`), so it never reaches the server or the config file, and it MUST be applied before the first paint, so a stored choice does not flicker. A forced choice sets `data-theme="light"` or `data-theme="dark"` on `<html>`, and the stylesheet MUST let that win over `prefers-color-scheme`. "System" removes the attribute and the stored value. Without JavaScript, the switch is hidden and the page follows the system setting.

- **WHEN** the system is dark and the user picks Light
  **THEN** the page turns light at once and stays light on reload and on the other UI page
- **WHEN** the user picks System again
  **THEN** the stored value is removed and the page follows the system setting

**Code:** `internal/webui/templates/layout.html`, `internal/webui/static/phorga.css`, `internal/webui/i18n_ui.go`
**Tests:** `internal/webui/usability_test.go#TestThemeSwitchIsRendered`
**Notes:** The switch behavior itself runs in the browser and is checked manually. The Go test covers the markup, the translations and the CSS override selectors.

### WEB-014 Live progress of running jobs
**Status:** implemented

For a running job, the status page (WEB-011) MUST show under the "running" badge what the run is doing right now:

- the phase, from the message catalogs (WEB-005): counting files, organizing files (organizer), scanning files, comparing files, applying actions (duplicate detection)
- once the amount of work is known: `<done> / <total> (<percent> %)` and a progress bar
- how long the run has been going

Before working, each run MUST count the supported files without opening them, so the total is known. The progress MUST come from the job itself while it runs, MUST be kept in memory only (never written to disk, also not in a dry run), and MUST disappear when the run ends. It is updated by the table's normal refresh every 10 s. A job that is not running shows no progress.

- **WHEN** a duplicate detection run has scanned 1,500 of 7,402 files for 3 minutes, in German
  **THEN** the state column shows `Dateien prüfen: 1500 / 7402 (20 %)`, a progress bar and `läuft seit 3m0s`
- **WHEN** the run is still counting
  **THEN** only the phase is shown, without numbers or bar
- **WHEN** the run ends (finished or stopped, RUN-001)
  **THEN** its progress is removed and the status page shows the last result as before

**Code:** `internal/runstatus/runstatus.go`, `internal/metadata/metadata.go`, `internal/organizer/organizer.go`, `internal/dedup/dedup.go`, `cmd/phorga/main.go`, `internal/webui/status.go`, `internal/webui/templates/status.html`, `internal/webui/i18n_ui.go`, `internal/webui/static/phorga.css`
**Tests:** `internal/runstatus/progress_test.go#TestProgressIsKeptInMemoryOnly`, `internal/runstatus/progress_test.go#TestReportReachesReporterInContext`, `internal/metadata/count_test.go#TestCountSupported`, `internal/dedup/progress_test.go#TestDedup_ReportsProgress`, `internal/organizer/progress_test.go#TestOrganize_ReportsProgress`, `internal/webui/usability_test.go#TestStatusTableShowsProgress`, `cmd/phorga/status_test.go#TestProgressIsShownWhileRunningAndClearedAfter`
**Notes:** Jobs report through the run's `context.Context` (`runstatus.Report`), the same way they receive the stop request (RUN-001), so `organizer.Run` and `dedup.Run` need no extra parameter. Counting walks the folders once more; on a local disk this took a few milliseconds for 7,950 files.
