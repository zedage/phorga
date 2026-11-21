# Spec: Synology Photos users (`SYN`)

> Human explanation: [product.md › Synology Photos users](../product.md#synology-photos-users) · Architecture: [arc42 §5 Building Blocks](../arc42/arc42.adoc) · Config: [CFG-010](configuration.md) · Format: [README](README.md)

---

### SYN-001 Find Synology Photos users
**Status:** implemented

The system MUST list the Synology Photos users below the homes folder (`synologyPhotos.homes`, default `/homes`, CFG-010). A user is a folder `<homes>/<name>` that contains a folder `Photos`. The list MUST:
- be sorted by name, ignoring case
- leave out files, folders without `Photos`, and entries whose name starts with `.`, `@` or `#` (DSM system folders such as `@eaDir` and `#recycle`)
- follow symlinked home folders
- include a user whose home or `Photos` folder Phorga may not open, marked as not readable
- mark whether `Photos/MobileBackup` exists

Finding users MUST only read folder names and MUST NOT create, change or delete anything. A homes folder that cannot be read is an error, which the UI shows as a hint (SYN-003).

- **WHEN** homes contains `Alice/Photos`, `bob/Photos/MobileBackup`, `carol/Documents`, `@eaDir/Photos`, `#recycle/Photos`, `.hidden/Photos` and `notes.txt`
  **THEN** the list is `Alice` (no MobileBackup), `bob` (with MobileBackup)
- **WHEN** the home folder `dave` cannot be opened by Phorga's user
  **THEN** `dave` is listed as not readable
- **WHEN** the homes folder does not exist
  **THEN** an error is returned

**Code:** `internal/synology/synology.go`
**Tests:** `internal/synology/synology_test.go#TestDiscoverListsUsersWithPhotos`, `internal/synology/synology_test.go#TestDiscoverMarksUnreadableHome`, `internal/synology/synology_test.go#TestDiscoverMissingHomesFails`, `internal/synology/synology_test.go#TestDiscoverNeverWrites`
**Notes:** Synology Photos keeps a user's personal space in `/volume1/homes/<user>/Photos` (DSM "User Home" service), and its mobile app backs up to `Photos/MobileBackup/<device>` by default. The shared space (`/volume1/photo`) is not offered yet.

### SYN-002 Profile for a user
**Status:** implemented

Adding a user MUST create one organizer profile with these values. The user can change them before saving.

| Field | Value |
|---|---|
| `name` | `<name>` (CFG-011) |
| `target` | `<homes>/<name>/Photos` |
| `targetStructure` | `{year}/{month}` |
| `dryRun` | `true` |
| `sources[0].path` | `<homes>/<name>/Photos/MobileBackup` |
| `sources[0].mode` | `move` |
| `sources[0].schedule` | `0 3 * * *` |
| `sources[0].duplicates` | `detectionMode: filename+exif`, `action.method: skip` |

The profile MUST pass validation (CFG-002). A user counts as already set up when an organizer profile's `target` is the user's `Photos` folder, or one of its sources lies inside it (compared as cleaned paths, so `/homes/bob/PhotosOld` does not count).

- **WHEN** user `bob` is added with homes `/homes`
  **THEN** the profile moves `/homes/bob/Photos/MobileBackup` into `/homes/bob/Photos/{year}/{month}` every day at 03:00, as a dry run
- **WHEN** a profile has the source `/homes/bob/Photos/Upload`
  **THEN** `bob` counts as already set up

**Code:** `internal/synology/synology.go`, `internal/webui/edit.go`
**Tests:** `internal/synology/synology_test.go#TestProfileMovesMobileBackupIntoPhotosAsDryRun`, `internal/synology/synology_test.go#TestConfigured`, `internal/webui/edit_test.go#TestAddUsersCreatesProfiles`
**Notes:** `move` within the same folder tree is a rename, so it needs no extra space; the dry run start lets the user check the log first. Whether the Synology Photos mobile app uploads a file again after Phorga moved it out of `MobileBackup` has not been verified on a real NAS.

### SYN-003 Pick users in the web UI
**Status:** implemented

The configuration page (WEB-002) MUST start with a "Synology Photos users" section that shows:
- the editable `synologyPhotos.homes` field (CFG-010)
- one row per user found (SYN-001) with a checkbox, the name and the `Photos` path. A user who is already set up (SYN-002) is shown checked, disabled and marked "already set up". A user who is not readable is disabled and marked "no access". A user without `MobileBackup` is marked accordingly but can be picked.
- a hint naming the folder when the homes folder cannot be read, or when it has no users
- "Add selected users" (only when users were found) and "Search again"

The users are searched again on every page render, in the homes folder of the form being edited. "Add selected users" posts the form to `POST /ui/form` with `op=addUsers` and the ticked `synologyUser` names (WEB-015). It MUST add a profile (SYN-002) for each ticked user that the search reports, that is readable and not set up yet, and MUST ignore every other name, so a request cannot point Phorga at other folders. Nothing is saved until the user saves (WEB-004).

- **WHEN** homes holds `alice` and `bob`, and a profile already targets `bob/Photos`
  **THEN** `alice` has an enabled checkbox and `bob` is shown checked, disabled and "already set up"
- **WHEN** `op=addUsers` is posted with `alice`, `bob`, `../etc` and `mallory`
  **THEN** exactly one profile (for `alice`) is added, the page scrolls to it, and no file is written
- **WHEN** the homes folder does not exist
  **THEN** the section shows "No homes folder found" with the path and no user list

**Code:** `internal/webui/edit.go`, `internal/webui/server.go`, `internal/webui/model.go`, `internal/webui/templates/page.html`, `internal/webui/i18n_ui.go`
**Tests:** `internal/webui/edit_test.go#TestSynologySectionListsUsers`, `internal/webui/edit_test.go#TestSynologySectionExplainsMissingHomes`, `internal/webui/edit_test.go#TestAddUsersCreatesProfiles`
