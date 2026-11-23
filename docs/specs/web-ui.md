# Spec: Web UI (`WEB`)

> Human explanation: [product.md › Web UI](../product.md#web-ui) · Architecture: [arc42 §6 Web UI Request, §8 Configuration UI](../arc42/arc42.adoc) · Decisions: [ADR-0008](../decisions/0008-built-in-server-rendered-web-ui.md), [ADR-0009](../decisions/0009-ui-save-destination-and-no-hot-reload.md), [ADR-0013](../decisions/0013-folder-picker-limited-to-browse-roots.md), [ADR-0014](../decisions/0014-persisted-run-status-without-dry-runs.md), [ADR-0022](../decisions/0022-live-apply-and-setup-mode.md) · Format: [README](README.md)

---

### WEB-001 Optional, built into the same process
**Status:** implemented

The web UI MUST be off by default. It MUST start in the same process as the scheduler when `webUI.enabled: true` is set, the `--web-ui` flag is given, or the configuration has no jobs (setup mode, CFG-009). The listen address is, in this order: `--web-ui-addr`, the environment variable `PHORGA_WEB_UI_ADDR`, `webUI.address`, `127.0.0.1:8080`. The Docker image sets `PHORGA_WEB_UI_ADDR=0.0.0.0:8080`. On SIGINT/SIGTERM it MUST shut down gracefully together with the scheduler. If the HTTP server fails (e.g. the port is taken), the process MUST exit with a non-zero status.

- **WHEN** `--web-ui --web-ui-addr 127.0.0.1:9090` is given and the config has no `webUI` block
  **THEN** `GET http://127.0.0.1:9090/ui` answers 200, and after shutdown the port is closed and the exit code is 0
- **WHEN** `webUI.address` is set but `webUI.enabled` is `false` and `--web-ui` is not given
  **THEN** nothing listens on that address
- **WHEN** the UI address is already in use
  **THEN** the process exits with status 1
- **WHEN** `PHORGA_WEB_UI_ADDR=127.0.0.1:9191` and `webUI.address: 127.0.0.1:8080`
  **THEN** the UI listens on `127.0.0.1:9191`; with `--web-ui-addr` given as well, the flag wins
- **WHEN** the config has no jobs and no `webUI` block
  **THEN** the UI starts

**Code:** `cmd/phorga/main.go`, `internal/webui/server.go`, `Dockerfile`
**Tests:** `internal/webui/address_test.go#TestNewAddressPrecedence`, `cmd/phorga/main_test.go#TestRun_WebUIEnabledByFlag`, `cmd/phorga/main_test.go#TestRun_WebUIEnabledByConfig`, `cmd/phorga/main_test.go#TestRun_WebUIOffByDefault`, `cmd/phorga/main_test.go#TestRun_WebUIListenFailureExitsNonZero`, `cmd/phorga/main_test.go#TestRun_WebUIAddrFromEnv`, `cmd/phorga/main_test.go#TestWebUIAddressPrecedence`, `cmd/phorga/main_test.go#TestRun_StartsWebUIWithoutJobs`
**Notes:** With the environment variable, Docker users no longer have to set `webUI.address: 0.0.0.0:8080`; which interfaces can reach the UI is decided by the port mapping.

### WEB-002 Server-rendered configuration page
**Status:** implemented

`GET /` MUST redirect to `/ui`. `GET /ui` MUST render an HTML page showing the configuration **in effect** (as loaded at startup or last applied, CFG-008) as an editable form, with sections in this order: Synology Photos users (SYN-003), organizer profiles with their sources, duplicate detection jobs, general settings (`stateDir`), web UI, and where to save. Profiles and jobs are collapsible cards (WEB-016). The page header shows the config path and a link to edit the file as text (WEB-018). Other HTTP methods on `/ui` MUST return 405.

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

On success it MUST report the written path and send the header `HX-Trigger: {"configSaved":true}`.

Saving to the active file MUST also apply the configuration without a restart (CFG-008). The result then reads "Saved and applied" and, as a warning, names the changed settings that still need a restart. If applying fails, the error is shown and the page keeps the configuration in effect. Saving to a separate file MUST NOT apply anything; the result says that the running configuration is unchanged.

- **WHEN** `saveTarget=separate` and `separatePath=/tmp/new.yaml`
  **THEN** `/tmp/new.yaml` is written, the active config and the running jobs are unchanged, and the response names `/tmp/new.yaml`
- **WHEN** the form is invalid
  **THEN** nothing is written
- **WHEN** the active file is saved with a new target and new `browseRoots`
  **THEN** the page shows the new target and the folder picker uses the new roots, without a restart

**Code:** `internal/webui/server.go`, `internal/config/saver.go`, `cmd/phorga/engine.go`
**Tests:** `internal/webui/server_test.go#TestSaveWritesSeparateFile`, `internal/webui/server_test.go#TestSaveWritesActiveConfigFile`, `internal/webui/apply_test.go#TestSaveAppliesThroughApplyFunc`, `internal/webui/apply_test.go#TestSaveShowsApplyError`, `internal/webui/apply_test.go#TestSaveToSeparateFileDoesNotApply`, `cmd/phorga/main_test.go#TestRun_SaveAppliesWithoutRestartAndPersists`

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

The UI has no user accounts. It can be protected with one password (WEB-020), and it rejects cross-site writes (WEB-021). Its documentation MUST state that it is meant for local or trusted networks only, never for the internet. The default address MUST bind to loopback (`127.0.0.1`).

**Code:** `internal/config/loader.go`
**Tests:** `internal/config/defaults_test.go#TestLoadAppliesDefaults`, `internal/webui/address_test.go#TestNewAddressPrecedence`
**Notes:** In Docker, the in-container address must be `0.0.0.0:8080`, and the loopback restriction moves to the host port mapping (`127.0.0.1:8080:8080`). To reach the UI from other devices, the guide (WEB-022) uses a reverse proxy on the NAS (HTTPS, port mapping stays on loopback) or publishes the port on the home network, in both cases with a password. README, `docker-compose.yml` and `config.example.yaml` document this. HTMX is embedded in the binary (WEB-009), so the UI needs no internet access. Without JavaScript, the forms still work as plain POSTs and return full pages. The folder picker (WEB-010) can list directories, so its roots should be limited to the media folders. Changed 2026-11-21: optional password and cross-site protection ([ADR-0023](../decisions/0023-optional-web-ui-password.md)).

### WEB-007 Choice fields and help texts
**Status:** implemented

Fields with a fixed set of values MUST be rendered as `<select>`. The allowed values MUST come from the same lists the validation uses (CFG-002):
- organizer source `mode`: `copy`, `move`
- organizer source duplicate `actionMethod`: `addSuffix`, `moveToFolder`, `replace`, `skip`
- duplicate detection `detectionMode`: `content`, `metadata`
- duplicate detection `actionMethod`: `moveToFolder`, `skip`

Each option MUST show a plain-language label from the message catalogs that starts with its value, e.g. `copy – keep the original` (WEB-017). Each select also offers an empty "not set" option. A value from the file that is not in the list MUST stay selected, shown as `<value> (unknown)`, so validation can report it. The organizer duplicate `detectionMode` is edited with checkboxes (WEB-017). Every editable field MUST show a short help text from the message catalogs (WEB-005), linked with `aria-describedby`.

- **WHEN** a source has `mode: copy`
  **THEN** the mode select has `copy – keep the original` selected and also offers `move – remove the original after copying`
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
- use the roots in effect, which change when a save is applied (CFG-008)

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

`GET /ui/status` MUST show one row per registered job (each organizer source, each enabled dedup job) with: the profile's or job's name (CFG-011), or its kind when it has none, and the label (`<source> → <target>`, or the dedup sources), cron schedule, next planned run, running/waiting/idle ("waiting for another job" while the job is due but another job runs, RUN-002), and the last run's start, duration, counters and a dry-run badge. Jobs that have not run show "not run yet". Above the table, the page MUST show how many jobs there are and how many are running, waiting and idle, counting each job once (running before waiting). The table and the summary MUST refresh themselves every 10 s from `GET /ui/status/table` (fragment). Both endpoints answer `405` for other methods.

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

### WEB-015 Add and remove entries
**Status:** implemented

The configuration page MUST offer buttons to add an organizer profile, a source to a profile, a duplicate detection job and a source to a job, and to remove each profile, source and job. Each button posts the whole form to `POST /ui/form` with `op=<operation>` (`addProfile`, `removeProfile:<i>`, `addSource:<i>`, `removeSource:<i>:<j>`, `addDedup`, `removeDedup:<i>`, `addDedupSource:<i>`, `removeDedupSource:<i>:<j>`, plus `addUsers` and `refreshUsers` from SYN-003). The response MUST be the page with the change made, keeping every value typed so far, and MUST name the added or changed entry so the browser scrolls to it. With HTMX the page area is swapped in place; without JavaScript it is a plain POST. An edit MUST NOT write any file or change the running configuration; that happens on save (WEB-004). Unknown operations and indexes out of range change nothing. Other methods than POST answer `405`.

New entries start safe:

| Entry | Values |
|---|---|
| organizer profile | `dryRun: true`, `targetStructure: {year}/{month}`, one source |
| organizer source | `mode: copy`, `schedule: 0 2 * * *` |
| duplicate detection job | `enabled: true`, `dryRun: true`, `schedule: 0 4 * * *`, `detectionMode: metadata`, `action.method: skip`, one empty source |

- **WHEN** `op=addSource:0` is posted with a changed but unsaved `organizer.0.target`
  **THEN** the page has two sources in profile 0, still shows the typed target, and no file was written
- **WHEN** `op=removeProfile:7` is posted and there is one profile
  **THEN** nothing changes

**Code:** `internal/webui/edit.go`, `internal/webui/server.go`, `internal/webui/templates/page.html`, `internal/webui/templates/partials.html`, `internal/webui/static/phorga.css`
**Tests:** `internal/webui/edit_test.go#TestFormOpsAddAndRemoveEntries`, `internal/webui/edit_test.go#TestFormOpKeepsTypedValuesAndNeverWrites`

### WEB-016 Collapsed cards with a flow summary
**Status:** implemented

Each organizer profile and duplicate detection job MUST be a collapsible card (`<details>`). Its always visible summary MUST show the number, the name (CFG-011) or "Organizer profile <n>" / "Duplicate job <n>" counted from 1, the dry-run and disabled badges, and a flow from left to right:
- organizer profile: each source folder with its mode and schedule → the target folder with its `targetStructure`
- duplicate detection job: each compared folder → the detection mode, schedule, action and duplicates folder

The summary MUST follow the fields while the user types. Unset paths show "not set". The card body holds all fields; rarely used ones are in a "More options" block that is also collapsed: per profile `fallbackFolder`, `ignoreModTime`, ntfy URL; per source `exclude`; per job `exclude` and ntfy URL. The sections general, web UI and save target are collapsed too, with a short summary in their heading (state folder, address, save target). The Synology section starts open in setup mode or when a user can be added.

A collapsed card or block MUST start open when it contains a field with a validation problem, or when it was just added or changed by an edit (WEB-015). After an HTMX edit, the cards that were open stay open. After an HTMX validate or save, cards with field errors open. Collapsed fields MUST still be submitted, so saving never drops them. Without JavaScript the cards work as plain `<details>`.

- **WHEN** the page is opened with one profile reading `/photos/incoming` (copy, `0 2 * * *`) into `/photos/organized`
  **THEN** its card is collapsed and its summary shows `/photos/incoming`, `copy`, `0 2 * * *` → `/photos/organized`, `{year}/{month}`
- **WHEN** validate reports a bad `exclude` pattern of source 0 of profile 0
  **THEN** that card and the source's "More options" are open, and the other cards stay collapsed
- **WHEN** a duplicate detection job is added
  **THEN** its card is open

**Code:** `internal/webui/templates/page.html`, `internal/webui/templates/partials.html`, `internal/webui/model.go`, `internal/webui/static/phorga.css`, `internal/webui/i18n_ui.go`
**Tests:** `internal/webui/overview_test.go#TestCardsStartCollapsedWithFlowSummary`, `internal/webui/overview_test.go#TestCardOpensForErrorsAndNewEntries`
**Notes:** Keeping cards open across edits and opening them for HTMX errors runs in the browser and is checked manually; the Go tests cover the server-rendered `open` state.

### WEB-017 Plain-language choices for duplicates
**Status:** implemented

The organizer duplicate `detectionMode` MUST be edited as one checkbox per field, each with a short explanation: "Same file name" (`filename`), "Same modification date" (`filedate`), "Same capture date" (`exif`). With two or more ticked, a choice "All ticked must match" (`+`) / "One match is enough" (`|`) MUST appear. The server builds `detectionMode` from the ticked fields in the order `filename`, `filedate`, `exif`, so mixing `+` and `|` cannot happen. No ticked field gives an empty value, which validation reports next to the checkboxes when duplicate handling is on (CFG-002). A value from the file that the checkboxes cannot show (e.g. `filename+exif|filedate`) MUST be shown as text with a note, with its known fields ticked. Requests without the checkbox marker (`<source>.detectionUI`) still accept `detectionMode` as text.

The select options of mode and actions MUST have plain-language labels (WEB-007), e.g. `skip – leave the new file in the source` (organizer) and `skip – only report them` (duplicate detection).

- **WHEN** a source has `detectionMode: filedate|exif`
  **THEN** "Same modification date" and "Same capture date" are ticked and "One match is enough" is selected
- **WHEN** `exif` and `filename` are ticked with "All ticked must match"
  **THEN** the saved value is `filename+exif`
- **WHEN** duplicate handling is on and no field is ticked
  **THEN** validate shows `detectionMode is required` next to the checkboxes

**Code:** `internal/webui/model.go`, `internal/webui/templates/partials.html`, `internal/webui/templates/page.html`, `internal/webui/i18n_ui.go`, `internal/config/detection.go`
**Tests:** `internal/webui/overview_test.go#TestDetectionModeAsCheckboxes`, `internal/webui/overview_test.go#TestDetectionCheckboxesBuildTheMode`, `internal/webui/overview_test.go#TestNoDetectionFieldTickedIsReported`, `internal/webui/overview_test.go#TestInvalidDetectionModeFromFileIsShown`, `internal/webui/usability_test.go#TestPageRendersSelectsWithCurrentValue`

### WEB-018 Edit the config file as text
**Status:** implemented

The configuration page header MUST link to `GET /ui/yaml` ("Edit YAML", or "Edit JSON" for a `.json` file). That page MUST show the active config file's text as it is, comments included, in a monospace editor. In setup mode, before the file exists, it shows the configuration in effect as it would be written, with a note that saving creates the file.

`POST /ui/yaml` with `content` and `action`:
- `validate` parses the text in the file's format and validates it with the startup rules (CFG-002); it writes nothing
- `save` does the same and, only if valid, writes the text **as it is** (line endings normalized to LF) atomically to the active config file and applies it without a restart (CFG-008). The result names settings that need a restart. The configuration page then shows the applied configuration.

Parse errors and problems MUST be listed above the editor, and the edited text MUST stay in the editor. Invalid text MUST NOT be written or applied. Tab indents with two spaces; Esc, then Tab, leaves the editor. Other methods answer `405`.

- **WHEN** the file starts with `# my notes`
  **THEN** the editor shows `# my notes`, and after a save the file still has it
- **WHEN** `organizer: [` is saved
  **THEN** a YAML parse error is shown, the text stays, and the file is unchanged
- **WHEN** there is no config file yet
  **THEN** the editor shows the configuration in effect and saving creates the file

**Code:** `internal/webui/rawedit.go`, `internal/webui/templates/rawedit.html`, `internal/webui/server.go`, `internal/config/loader.go`, `internal/config/saver.go`, `cmd/phorga/engine.go`
**Tests:** `internal/webui/overview_test.go#TestRawEditorShowsFileWithComments`, `internal/webui/overview_test.go#TestRawEditorRejectsInvalidText`, `internal/webui/overview_test.go#TestRawEditorSavesTextAsItIs`, `cmd/phorga/engine_test.go#TestEngineApplyRawKeepsCommentsAndSchedules`
**Notes:** Saving the form (WEB-004) still rewrites the file from the data model, without comments (CFG-004); the editor page says so.

### WEB-019 Jump from a job to its schedule
**Status:** implemented

On the status page, each job's name, label and schedule MUST link to the configuration page at the field of its schedule: `/ui#f-organizer-<i>-sources-<j>-schedule` for an organizer source, `/ui#f-duplicateDetection-<i>-schedule` for a duplicate detection job. When the configuration page is opened with such a fragment, it MUST open the collapsed card around the field, scroll it into view, focus it, and highlight it briefly.

- **WHEN** the status page lists `organizer-0-source-1`
  **THEN** its row links to `/ui#f-organizer-0-sources-1-schedule`, and that id exists on the configuration page
- **WHEN** the configuration page is opened with that link
  **THEN** profile 1's card opens and the schedule field of source 2 is focused and highlighted

**Code:** `internal/webui/templates/status.html`, `internal/webui/status.go`, `internal/webui/templates/page.html`, `internal/webui/static/phorga.css`, `cmd/phorga/main.go`
**Tests:** `internal/webui/overview_test.go#TestStatusJobsLinkToTheirSchedule`, `cmd/phorga/status_test.go#TestStatusBoardReportsRecordedRun`
**Notes:** Opening, scrolling and highlighting run in the browser and are checked manually. Indexes are positions: a link opened after the configuration changed may point at another job.

### WEB-020 Optional password
**Status:** implemented

When the environment variable `PHORGA_WEB_UI_PASSWORD` is set and not empty, every request to the web UI, static files included, MUST carry HTTP Basic credentials with that password; the user name is not checked. Without them, or with a wrong password, the UI MUST answer `401` with `WWW-Authenticate: Basic realm="Phorga", charset="UTF-8"` and show nothing else. The password is compared in constant time. It is read only from the environment at startup, never from or into the config file, so neither the form nor the YAML editor (WEB-018) shows it, and changing it needs a restart.

Without a password, the UI works as before. If it then listens on an address other than loopback, Phorga MUST log a warning at startup that anyone who can reach the port can change the configuration.

- **WHEN** `PHORGA_WEB_UI_PASSWORD=s3cret` and `/ui` is requested without credentials
  **THEN** the answer is `401` with the `WWW-Authenticate` header
- **WHEN** the same request carries user `anyone` and password `s3cret`
  **THEN** the configuration page is shown
- **WHEN** it carries password `wrong`, or `/ui/static/phorga.css` is requested without credentials
  **THEN** the answer is `401`
- **WHEN** no password is set and the UI listens on `0.0.0.0:8080`
  **THEN** the start-up log has a warning; on `127.0.0.1:8080` it has none

**Code:** `internal/webui/auth.go`, `internal/webui/server.go`, `cmd/phorga/main.go`
**Tests:** `internal/webui/auth_test.go#TestPasswordProtectsEveryPage`, `internal/webui/auth_test.go#TestNoPasswordNoPrompt`, `cmd/phorga/main_test.go#TestWebUIPasswordFromEnvironment`, `cmd/phorga/main_test.go#TestOpenWithoutPassword`
**Notes:** Basic auth needs no session or login page and works behind any reverse proxy. Over plain HTTP the password crosses the network readable, so the guide (WEB-022) puts HTTPS in front for access from other devices. There is no lockout after failed attempts; the guide asks for a long password. Decision: [ADR-0023](../decisions/0023-optional-web-ui-password.md).

### WEB-021 Reject cross-site writes
**Status:** implemented

The UI MUST reject requests other than `GET`, `HEAD` and `OPTIONS` that a browser marks as coming from another site, with `403` and without changing anything. A request counts as cross-site when its `Sec-Fetch-Site` header is neither `same-origin` nor `none`, or, without that header, when its `Origin` header names another host than the request's `Host`. Requests with neither header (such as `curl`) are allowed.

- **WHEN** a page on another site posts a form to `/ui/yaml` (`Sec-Fetch-Site: cross-site`)
  **THEN** the answer is `403` and the config file is unchanged
- **WHEN** the UI's own page posts to `/ui/yaml` (`Sec-Fetch-Site: same-origin`)
  **THEN** the request is handled
- **WHEN** a request without `Sec-Fetch-Site` has `Origin: http://evil.example` and `Host: nas:8080`
  **THEN** the answer is `403`

**Code:** `internal/webui/server.go`
**Tests:** `internal/webui/auth_test.go#TestCrossSiteWritesAreRejected`
**Notes:** Without this, any website open in a browser on the same network could post a new configuration to the UI, which applies at once (CFG-008). A password alone does not help, because the browser sends stored Basic credentials along. Uses Go's `http.CrossOriginProtection`. Decision: [ADR-0023](../decisions/0023-optional-web-ui-password.md).

### WEB-022 Help page with the setup guide
**Status:** implemented

Every UI page's top bar MUST link to `GET /ui/help` ("Help"). The setup hint shown while there are no jobs (CFG-009) MUST link there too. The help page MUST show, in the chosen language (WEB-005, English as fallback):
1. **This installation:** the config file path and whether it exists, the Synology homes folder (CFG-010) with the number of Synology Photos users found or "not found" (SYN-001), whether a password is set (WEB-020), and the address the UI listens on
2. the step-by-step guide for installing on a Synology NAS with Container Manager, and on another computer (Mac mini, NUC, server) that reaches the NAS over the network, each with its `docker-compose.yml`
3. setting Phorga up in the UI: pick users, check the dry run, switch the dry run off
4. opening the UI from other devices: password, Synology reverse proxy with HTTPS, or the port on the home network
5. troubleshooting

The two `docker-compose.yml` examples MUST be the same in the help page and in the README's setup guide.

- **WHEN** `/ui/help?lang=de` is requested
  **THEN** the German guide is shown with both compose examples, and the top bar marks "Hilfe" as the current page
- **WHEN** no password is set and the homes folder holds two users
  **THEN** "This installation" says the password is off and that 2 users were found
- **WHEN** the README is compared with the help page
  **THEN** it contains both compose examples unchanged (indented as part of a numbered list)

**Code:** `internal/webui/help.go`, `internal/webui/templates/help.html`, `internal/webui/templates/help_en.html`, `internal/webui/templates/help_de.html`, `internal/webui/templates/help_it.html`, `internal/webui/templates/help_fr.html`, `internal/webui/templates/layout.html`, `internal/webui/templates/page.html`, `README.md`
**Tests:** `internal/webui/help_test.go#TestHelpPageInEveryLanguage`, `internal/webui/help_test.go#TestHelpPageDescribesThisInstallation`, `internal/webui/help_test.go#TestReadmeHasTheHelpPageComposeFiles`, `internal/webui/help_test.go#TestHelpLinkOnEveryPage`
**Notes:** The guide text is one template per language instead of message keys, because it is long prose with inline code. The Synology steps follow DSM 7.2 and were written without a NAS to click through: the `homes` permission for a non-admin user, finding the user ID with Task Scheduler, and the reverse proxy reaching `localhost:8080` still need a check on a real NAS.
