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
- `organizer[].sources[].duplicates`: `detectionMode` uses only `filename`/`filedate`/`exif`, without mixing `+` and `|`; `action.method` ∈ {`replace`, `addSuffix`, `moveToFolder`, `skip`}; `duplicateFolder` is required for `moveToFolder`
- `duplicateDetection[]`: `schedule` required and valid; at least one source; `detectionMode` ∈ {`content`, `metadata`}; `action.method` ∈ {`moveToFolder`, `skip`}; `duplicateFolder` is required for `moveToFolder`
- `webUI` (when `enabled`): `address` is required and must be in `host:port` form
- An empty config (no organizer, no duplicate detection) is invalid

- **WHEN** `target` is missing from an organizer profile
  **THEN** startup fails with `organizer[0].target is required`
- **WHEN** a `schedule` is `every day`
  **THEN** startup fails and names the invalid expression

**Code:** `internal/config/validate.go`, `cmd/phorga/main.go`
**Tests:** `internal/config/config_test.go#TestValidateValid`, `internal/config/config_test.go#TestValidateMissingTarget`, `internal/config/config_test.go#TestValidateInvalidMode`, `internal/config/config_test.go#TestValidateInvalidCron`, `internal/config/config_test.go#TestValidateInvalidTargetToken`, `internal/config/config_test.go#TestValidateMoveToFolderRequiresDuplicateFolder`, `internal/config/config_test.go#TestValidateMixedOperators`, `internal/config/config_test.go#TestValidateDuplicateDetection`, `internal/config/config_test.go#TestValidateEmptyConfig`, `internal/config/config_test.go#TestValidateWebUIRequiresAddressWhenEnabled`, `internal/config/config_test.go#TestValidateProblemsReturnsIndividualMessages`

### CFG-003 Config path resolution
**Status:** implemented

The config path MUST be resolved in this order:

1. `--config <path>` CLI flag
2. `PHORGA_CONFIG` environment variable
3. `./config.yaml` in the working directory

If the chosen file does not exist, startup MUST fail with a message that names all three options.

**Code:** `internal/config/resolve.go`, `cmd/phorga/main.go`
**Tests:** `internal/config/config_test.go#TestResolveConfigPath_CLI`, `internal/config/config_test.go#TestResolveConfigPath_Env`, `internal/config/config_test.go#TestResolveConfigPath_NotFound`, `internal/config/defaults_test.go#TestResolveConfigPath_DefaultFileInWorkingDir`

### CFG-004 Saving configuration
**Status:** implemented

The system MUST be able to write a configuration back to disk. It writes in the format given by the target file's extension, writes atomically (temp file + rename), creates missing directories, and applies the defaults (CFG-001). Writing and then reading back a config MUST give an equal configuration.

**Code:** `internal/config/saver.go`
**Tests:** `internal/config/config_test.go#TestSaveAndLoadYAMLRoundTrip`, `internal/config/config_test.go#TestSaveAndLoadJSONRoundTrip`
**Notes:** Saving writes the file from the data model, so comments and key order in the original YAML are lost. Used by the web UI (WEB-004).

### CFG-005 No hot reload
**Status:** implemented

The running process MUST keep using the configuration it loaded at startup. Changes to the file, including changes saved via the web UI, take effect only after a restart.

**Code:** `cmd/phorga/main.go`
**Tests:** none
**Gap:** Behavior by construction; not tested. See [ADR-0009](../decisions/0009-ui-save-destination-and-no-hot-reload.md).
