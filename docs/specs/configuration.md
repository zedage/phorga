# Spec: Configuration (`CFG`)

> Human explanation: [product.md › Configuration](../product.md#configuration) · Reference example: [`config.example.yaml`](../../config.example.yaml) · User docs: [README › Configuration](../../README.md#configuration) · Format: [README](README.md)

---

### CFG-001 YAML and JSON config files
**Status:** implemented

The system MUST read its configuration from a single file. The format follows the file extension: `.yaml`/`.yml` for YAML, `.json` for JSON. Any other extension MUST be rejected.

Defaults applied after loading:

| Field | Default |
|---|---|
| `stateDir` | `/data/state` |
| `organizer[].targetStructure` | `{year}/{month}` |
| `webUI.address` (if a `webUI` block exists) | `127.0.0.1:8080` |

**Code:** `internal/config/loader.go`, `internal/config/config.go`
**Tests:** `internal/config/config_test.go#TestLoadYAML`, `internal/config/config_test.go#TestLoadJSON`, `internal/config/config_test.go#TestLoadUnsupportedExtension`, `internal/config/defaults_test.go#TestLoadAppliesDefaults`

### CFG-002 Fail-fast validation
**Status:** implemented

The whole configuration MUST be validated at startup. On any problem, the process MUST exit with a non-zero status, listing **all** problems with their path (e.g. `organizer[0].sources[1].mode`).

Rules:
- `organizer[]`: `target` required; `targetStructure` must contain only known tokens (ORG-009); at least one source
- `organizer[].sources[]`: `path` required; `mode` ∈ {`move`, `copy`}; `schedule` must be a valid 5-field cron expression
- `organizer[].sources[].duplicates`: `detectionMode` uses only `filename`/`filedate`/`exif`, without mixing `+` and `|`; `action.method` ∈ {`replace`, `addSuffix`, `moveToFolder`, `skip`}; `duplicateFolder` is required for `moveToFolder`; `replace` is rejected when `detectionMode` would match on the filename alone (`filename`, or any `|` expression containing `filename`), see ORG-012
- `duplicateDetection[]`: `schedule` required and valid; at least one source; `detectionMode` ∈ {`content`, `metadata`}; `action.method` ∈ {`moveToFolder`, `skip`}; `duplicateFolder` is required for `moveToFolder`
- `webUI` (when `enabled`): `address` is required and must be in `host:port` form
- `synologyPhotos.homes` (when set): see CFG-010
- A config without organizer profiles and duplicate detection jobs is valid; Phorga then runs in setup mode (CFG-009)

- **WHEN** `target` is missing from an organizer profile
  **THEN** startup fails with `organizer[0].target is required`
- **WHEN** a `schedule` is `every day`
  **THEN** startup fails and names the invalid expression
- **WHEN** an organizer source has `detectionMode: filename` and `action.method: replace`
  **THEN** startup fails with `organizer[0].sources[0].duplicates.action.method 'replace' needs a detectionMode that compares filedate or exif …`

**Code:** `internal/config/validate.go`, `internal/config/detection.go`, `cmd/phorga/main.go`
**Notes:** An empty config used to be invalid. Since setup mode (CFG-009) it is valid.
**Tests:** `internal/config/detection_test.go#TestValidateReplaceNeedsMoreThanTheFilename`, `internal/config/detection_test.go#TestParseDetectionMode`, `internal/config/config_test.go#TestValidateValid`, `internal/config/config_test.go#TestValidateMissingTarget`, `internal/config/config_test.go#TestValidateInvalidMode`, `internal/config/config_test.go#TestValidateInvalidCron`, `internal/config/config_test.go#TestValidateInvalidTargetToken`, `internal/config/config_test.go#TestValidateMoveToFolderRequiresDuplicateFolder`, `internal/config/config_test.go#TestValidateMixedOperators`, `internal/config/config_test.go#TestValidateDuplicateDetection`, `internal/config/config_test.go#TestValidateEmptyConfig`, `internal/config/config_test.go#TestValidateWebUIRequiresAddressWhenEnabled`, `internal/config/config_test.go#TestValidateProblemsReturnsIndividualMessages`, `cmd/phorga/main_test.go#TestRun_InvalidConfigExitsNonZero`

### CFG-003 Config path resolution
**Status:** implemented

The config path MUST be resolved in this order:

1. `--config <path>` CLI flag
2. `PHORGA_CONFIG` environment variable
3. `./config.yaml` in the working directory

The first option that is set wins, whether or not that file exists. If it does not exist, Phorga starts in setup mode (CFG-009). Unknown command-line flags MUST fail with exit status 2.

**Code:** `internal/config/resolve.go`, `cmd/phorga/main.go`
**Tests:** `internal/config/config_test.go#TestResolveConfigPath_CLI`, `internal/config/config_test.go#TestResolveConfigPath_Env`, `internal/config/config_test.go#TestResolveConfigPath_NotFound`, `internal/config/defaults_test.go#TestResolveConfigPath_DefaultFileInWorkingDir`, `cmd/phorga/main_test.go#TestLoadConfig_MissingFileGivesSetupConfig`, `cmd/phorga/main_test.go#TestRun_UnknownFlagExitsWithUsageError`
**Notes:** A missing file used to stop startup with exit status 1. It now starts setup mode, so a mistyped `--config` path starts an empty Phorga; the log and the web UI name the path in use.

### CFG-004 Saving configuration
**Status:** implemented

The system MUST be able to write a configuration back to disk. It writes in the format given by the target file's extension, writes atomically (temp file + rename), creates missing directories, and applies the defaults (CFG-001). Writing and then reading back a config MUST give an equal configuration.

**Code:** `internal/config/saver.go`
**Tests:** `internal/config/config_test.go#TestSaveAndLoadYAMLRoundTrip`, `internal/config/config_test.go#TestSaveAndLoadJSONRoundTrip`
**Notes:** Saving writes the file from the data model, so comments and key order in the original YAML are lost. Used by the web UI (WEB-004). A file that Docker mounts on its own cannot be replaced by a rename (`EBUSY`); the error then says to mount its folder instead.

### CFG-005 No hot reload
**Status:** removed

Replaced by CFG-008: changes saved in the web UI now apply without a restart ([ADR-0022](../decisions/0022-live-apply-and-setup-mode.md)).

**Code:** none
**Tests:** none

### CFG-006 Folder picker roots (`webUI.browseRoots`)
**Status:** implemented

`webUI.browseRoots` is an optional list of folders for the web UI folder picker (WEB-010). It has no default; empty or missing turns the picker off. Validation (CFG-002) MUST report each entry that is not an absolute path, does not exist, or is not a directory, as `webUI.browseRoots[<i>] …`. The value MUST survive a save and load round trip (CFG-004). It is editable in the UI as one path per line.

- **WHEN** `browseRoots: [photos]`
  **THEN** validation reports `webUI.browseRoots[0] must be an absolute path`
- **WHEN** `browseRoots` names a file
  **THEN** validation reports `webUI.browseRoots[0] '<path>' is not a directory`

**Code:** `internal/config/config.go`, `internal/config/validate.go`, `internal/config/saver.go`, `internal/webui/model.go`
**Tests:** `internal/config/browseroots_test.go#TestValidateBrowseRoots`, `internal/config/browseroots_test.go#TestSaveAndLoadKeepsBrowseRoots`
**Notes:** See [ADR-0013](../decisions/0013-folder-picker-limited-to-browse-roots.md).

### CFG-007 Exclude patterns (`exclude`)
**Status:** implemented

`organizer[].sources[].exclude` and `duplicateDetection[].exclude` are optional lists of patterns (ORG-023, DUP-013). They have no default. Validation (CFG-002) MUST report each entry that is empty, starts with `/`, contains a `..` segment, or is not a valid `path.Match` pattern, with its path, e.g. `organizer[0].sources[1].exclude[2]`. The values MUST survive a save and load round trip (CFG-004). They are editable in the web UI as one pattern per line, like `browseRoots` (CFG-006).

- **WHEN** `exclude: ["[abc"]`
  **THEN** validation reports `organizer[0].sources[0].exclude[0] is not a valid pattern`
- **WHEN** `exclude: [/photos/tmp]`
  **THEN** validation reports `duplicateDetection[0].exclude[0] must be relative to the source`

**Code:** `internal/config/config.go`, `internal/config/validate.go`, `internal/config/saver.go`, `internal/exclude/exclude.go`, `internal/webui/model.go`, `internal/webui/templates/partials.html`
**Tests:** `internal/exclude/exclude_test.go#TestValidate`, `internal/config/exclude_test.go#TestValidateExclude`, `internal/config/exclude_test.go#TestSaveAndLoadKeepsExclude`, `internal/webui/exclude_test.go#TestExcludeIsEditableOnePatternPerLine`, `internal/webui/exclude_test.go#TestInvalidExcludeShowsFieldError`
**Notes:** See [ADR-0016](../decisions/0016-fixed-skip-list-and-simple-exclude-patterns.md).

### CFG-008 Apply saved changes without a restart
**Status:** implemented

When the web UI saves to the active config file (WEB-004), the system MUST validate the new configuration, write it (CFG-004), read it back, and put it into effect without a restart:
- every job of the old configuration is unscheduled, and the jobs of the new one are scheduled at once (ORG-010, DUP-008)
- a job that is running at that moment MUST NOT be stopped; it finishes with the settings it started with. While it runs, a job registered again under the same name does not start (no overlap, ORG-010), and jobs still run one at a time (RUN-002)
- the last result of a job whose name now stands for another folder or kind MUST be dropped from the status page (WEB-011), so it does not show another job's numbers; a job that only changed its schedule or options keeps it
- the web UI then shows the new configuration, and its folder picker uses the new `browseRoots` (WEB-010)

If validation or writing fails, nothing changes. Three settings only take effect after a restart, and the save result MUST name the ones that changed: `stateDir` (jobs keep the state folder they started with until then), `webUI.address`, and turning `webUI.enabled` off. Edits to the file on disk are not watched; they take effect after a restart.

- **WHEN** Phorga runs without jobs and a profile with source `/in` and target `/out` is saved in the web UI
  **THEN** the status page lists `/in → /out` right away, and after a restart the profile and job are still there
- **WHEN** a dedup job is removed and saved
  **THEN** it has no next run any more
- **WHEN** an organizer source is running and its schedule is changed and saved
  **THEN** the run finishes undisturbed, and the new schedule applies to later runs
- **WHEN** `stateDir` and `webUI.address` are changed and saved
  **THEN** the result says that these take effect after a restart
- **WHEN** the config file is changed on disk while Phorga runs
  **THEN** the web UI still shows the values in effect

**Code:** `cmd/phorga/engine.go`, `cmd/phorga/main.go`, `internal/scheduler/scheduler.go`, `internal/runstatus/runstatus.go`, `internal/webui/server.go`
**Tests:** `cmd/phorga/engine_test.go#TestEngineApplySavesAndSchedulesJobs`, `cmd/phorga/engine_test.go#TestEngineApplyRejectsInvalidConfig`, `cmd/phorga/engine_test.go#TestEngineApplyForgetsResultsOfChangedJobs`, `cmd/phorga/engine_test.go#TestEngineApplyReportsRestartFields`, `internal/scheduler/unregister_test.go#TestUnregisterLetsTheRunningJobFinish`, `internal/scheduler/unregister_test.go#TestRegisterSameNameReplacesSchedule`, `internal/runstatus/forget_test.go#TestForgetDropsRecordsAndPersists`, `internal/webui/apply_test.go#TestSaveAppliesThroughApplyFunc`, `cmd/phorga/main_test.go#TestRun_SaveAppliesWithoutRestartAndPersists`, `cmd/phorga/main_test.go#TestRun_ConfigIsNotReloaded`
**Notes:** See [ADR-0022](../decisions/0022-live-apply-and-setup-mode.md). Job names are positions (`organizer-0-source-1`, `dedup-0`), so removing a profile renames the ones after it; their last results are dropped. A removed job that is still running is not listed on the status page until it ends, but other jobs show "waiting for another job" meanwhile.

### CFG-009 Start without a config (setup mode)
**Status:** implemented

When the config file does not exist, the system MUST start anyway with this configuration and log a warning with the path: no jobs, `stateDir: /data/state`, `webUI.enabled: true`, `webUI.address: 127.0.0.1:8080`. The path must still end in `.yaml`, `.yml` or `.json`, otherwise startup fails. Of the folders `/homes` and `/photos`, the ones that exist become `webUI.browseRoots`, so the folder picker works at once. The first save in the web UI creates the file and its folder (CFG-004, CFG-008).

Whenever the configuration has no jobs, at startup or from an existing file, the web UI MUST start (WEB-001), the log MUST name its URL, and the configuration page MUST show a greeting that explains how to add work. If such a configuration has no `webUI` block, it MUST be treated as `webUI.enabled: true`, so the first save keeps the UI on after a restart.

- **WHEN** `PHORGA_CONFIG=/config/config.yaml` and the file does not exist
  **THEN** Phorga runs, the web UI answers, and saving writes `/config/config.yaml`
- **WHEN** the missing file is `/config/config.txt`
  **THEN** startup fails with exit status 1
- **WHEN** the config only sets `stateDir`
  **THEN** the web UI starts, even without `webUI.enabled`, greets with "Phorga has nothing to do yet", and shows "Web UI enabled" switched on

**Code:** `internal/config/resolve.go`, `cmd/phorga/main.go`, `internal/webui/server.go`, `internal/webui/templates/page.html`
**Tests:** `internal/config/config_test.go#TestResolveConfigPath_NotFound`, `internal/config/config_test.go#TestLoadOrDefault`, `internal/config/config_test.go#TestValidateEmptyConfig`, `cmd/phorga/main_test.go#TestLoadConfig_MissingFileGivesSetupConfig`, `cmd/phorga/main_test.go#TestRun_StartsWebUIWithoutJobs`, `cmd/phorga/engine_test.go#TestEngineApplySavesAndSchedulesJobs`, `internal/webui/apply_test.go#TestSetupCalloutOnlyWithoutJobs`
**Gap:** The default browse roots (`/homes`, `/photos`) are not tested, because the test machine cannot create those folders.
**Notes:** See [ADR-0022](../decisions/0022-live-apply-and-setup-mode.md). In the Docker image, `PHORGA_WEB_UI_ADDR` makes the UI listen on `0.0.0.0:8080` (WEB-001).

### CFG-010 Synology homes folder (`synologyPhotos.homes`)
**Status:** implemented

`synologyPhotos.homes` is an optional path to the folder holding the user home folders, i.e. where the NAS folder `/volume1/homes` is mounted. It is used to find Synology Photos users (SYN-001). When missing or empty, `/homes` is used. When set, validation (CFG-002) MUST report it if it is not absolute, does not exist, or is not a directory, as `synologyPhotos.homes …`. The value MUST survive a save and load round trip (CFG-004) and is editable in the web UI (SYN-003).

- **WHEN** `synologyPhotos.homes: homes`
  **THEN** validation reports `synologyPhotos.homes must be an absolute path`
- **WHEN** the block is missing
  **THEN** users are searched in `/homes`

**Code:** `internal/config/config.go`, `internal/config/validate.go`, `internal/config/saver.go`, `internal/webui/model.go`
**Tests:** `internal/config/synology_test.go#TestValidateSynologyHomes`, `internal/config/synology_test.go#TestSaveAndLoadKeepsSynologyHomes`

### CFG-011 Names for profiles and jobs (`name`)
**Status:** implemented

`organizer[].name` and `duplicateDetection[].name` are optional labels. They have no default and no effect on what a profile or job does. They are shown in the web UI as card title and in the sidebar (WEB-016), and on the status page (WEB-011). Without a name, the UI shows "Organizer profile <n>" or "Duplicate job <n>", counted from 1. The value MUST survive a save and load round trip (CFG-004); an empty name is left out of the file. Leading and trailing spaces entered in the UI are removed. A profile added for a Synology Photos user is named after the user (SYN-002).

- **WHEN** profile 0 has `name: Robin's phone`
  **THEN** its card title and status rows show `Robin's phone`
- **WHEN** job 0 has no name
  **THEN** its card title is `Duplicate job 1`

**Code:** `internal/config/config.go`, `internal/webui/model.go`, `internal/webui/templates/page.html`, `internal/webui/templates/partials.html`, `cmd/phorga/main.go`
**Tests:** `internal/config/synology_test.go#TestSaveAndLoadKeepsNames`, `internal/webui/overview_test.go#TestNamesAreEditableAndShown`, `internal/synology/synology_test.go#TestProfileMovesMobileBackupIntoPhotosAsDryRun`
