# Phorga

Headless photo/video organizer and duplicate detector. Runs as a daemon, organizes media by date metadata into a configurable folder structure, and detects duplicates across sources.

## Features

- **Organizer**: date-based file organization (embedded date from EXIF, HEIC or MP4/MOV → filename → source path → mtime fallback), move/copy, per-source cron scheduling, lightweight duplicate handling, dry-run, state/resume. Transfers keep the file's modification time, are verified before the source is deleted, never overwrite an existing file, and moves within one volume don't copy at all
- **Duplicate Detection**: independent metadata- or content-based (perceptual hash) dedup across sources, moveToFolder/skip actions, dry-run
- **Web UI**: built-in configuration UI with dropdowns, help texts, live previews, a folder picker and a job status page with live progress, in DE/EN/IT/FR. Saved changes apply without a restart, and Phorga can start without any config and be set up there. Optional password, built-in setup guide
- **Synology Photos users**: finds the users with a Synology Photos library and sets up sorting of their phone uploads with one checkbox each
- **Notifications**: optional ntfy.sh push notifications on run completion
- **Docker-first**: single container, multi-arch (amd64/arm64), works on Synology NAS

## Quick Start

```bash
# Run directly: without a config file, Phorga starts its web UI for setup
go build -o phorga ./cmd/phorga
./phorga --config config.yaml        # then open http://127.0.0.1:8080/ui

# Or with Docker (see docker-compose.yml)
docker compose up -d                 # then open http://127.0.0.1:8080/ui
```

**New here?** Follow the [Setup guide](#setup-guide): Synology NAS or another computer, step by step.

You can also write the config yourself: copy [`config.example.yaml`](config.example.yaml) to `config.yaml` and edit it.

Stop Phorga with Ctrl+C (or `docker stop`). It finishes the current file and exits within a few seconds; press Ctrl+C again to quit at once.

## Setup guide

Two ways to run Phorga with Synology Photos: **on the NAS** in Container Manager, or **on another computer** that is always on (Mac mini, NUC, home server) and reaches the NAS over the network. Both end in the web UI, where you pick the users. The same guide is built into the web UI under **Help** (`/ui/help`), in German, English, Italian and French.

### A. On a Synology NAS (Container Manager)

You need DSM 7.2 or later and an administrator account. All steps are done in DSM in your browser.

1. **Install Container Manager.** Package Center → search for "Container Manager" → Install. If you use Synology Photos, the user home service is already on; otherwise turn it on in Control Panel → User & Group → Advanced → User Home.
2. **Create a user for Phorga.** Control Panel → User & Group → Create. Name it `phorga` and give it a long random password (you will not need it again). At "Assign shared folder permissions", give **Read/Write** to `docker` and `homes`. At "Assign application permissions", deny everything; Phorga never signs in to DSM.
3. **Create the folders.** In File Station, open `docker`, create the folder `phorga`, and inside it the folders `config` and `state`.
4. **Find the user ID.** Control Panel → Task Scheduler → Create → Scheduled Task → User-defined script. Name `phorga id`, user `root`, untick "Enabled"; under Task Settings, enter the command `id phorga > /volume1/docker/phorga/id.txt`. Save, select the task, click Run, and open `docker/phorga/id.txt` in File Station: `uid=1030(phorga) gid=100(users)` means `1030:100`. Then delete the task.
5. **Create the project.** Container Manager → Project → Create. Project name `phorga`, path `/docker/phorga`, source "Create docker-compose.yml", and paste:

   ```yaml
   services:
     phorga:
       image: mnrd001/phorga:latest
       container_name: phorga
       restart: unless-stopped
       user: "1030:100"                 # uid:gid of the DSM user "phorga"
       ports:
         - "127.0.0.1:8080:8080"        # NAS only; the reverse proxy opens it to your network
       volumes:
         - ./config:/config             # config.yaml, written by the web UI
         - ./state:/data/state
         - /volume1/homes:/homes        # the Synology Photos libraries of all users
       environment:
         - TZ=Europe/Berlin
         - PHORGA_WEB_UI_PASSWORD=choose-a-long-password
   ```

   Replace `1030:100` with the numbers from step 4, and `choose-a-long-password` with your own password for the web UI. Click Next, skip the web portal settings, and click Done. Container Manager downloads Phorga and starts it.
6. **Make it reachable with HTTPS.** Control Panel → Login Portal → Advanced → Reverse Proxy → Create: name `Phorga`; source protocol `HTTPS`, hostname `*`, port `8443`; destination protocol `HTTP`, hostname `localhost`, port `8080`. Save. If the DSM firewall is on, allow port 8443 (Control Panel → Security → Firewall).
7. **Open Phorga.** On any device in your network, open `https://<NAS address>:8443/ui`, e.g. `https://192.168.1.20:8443/ui`. If your NAS uses its own certificate, the browser warns once; at home that is expected. Sign in with any user name and your web UI password, then continue with [C. Set Phorga up](#c-set-phorga-up).

### B. On another computer (Mac mini, NUC, server)

Phorga runs on a computer that is always on and works on the NAS through a network share.

1. **Install Docker.** macOS and Windows: Docker Desktop, with "Start Docker Desktop when you sign in" turned on in its settings. Linux: Docker Engine with the Compose plugin.
2. **Allow access on the NAS.** In DSM, create the user `phorga` with Read/Write on `homes` (step A2), and check that SMB is on (Control Panel → File Services → SMB).
3. **Connect the share.** macOS: Finder → Go → Connect to Server → `smb://<NAS address>/homes`, sign in as `phorga`; the share appears as `/Volumes/homes`. To connect it again after a restart, add it under System Settings → General → Login Items. Linux: mount it, for example with this line in `/etc/fstab` (the credentials file holds `username=phorga` and `password=…`):

   ```
   //192.168.1.20/homes  /mnt/nas-homes  cifs  credentials=/etc/phorga-nas,uid=1000,gid=1000,_netdev  0  0
   ```

4. **Create the project.** Create a folder `phorga` in your home folder, with the folders `config` and `state` inside, and save this as `docker-compose.yml` in it:

   ```yaml
   services:
     phorga:
       image: mnrd001/phorga:latest
       container_name: phorga
       restart: unless-stopped
       user: "501:20"                   # output of "id -u" and "id -g" on this computer
       ports:
         - "127.0.0.1:8080:8080"        # this computer only; "8080:8080" opens it to your network
       volumes:
         - ./config:/config             # config.yaml, written by the web UI
         - ./state:/data/state
         - /Volumes/homes:/homes        # the NAS share "homes"; Linux: /mnt/nas-homes:/homes
       environment:
         - TZ=Europe/Berlin
         - PHORGA_WEB_UI_PASSWORD=choose-a-long-password
   ```

   Set `user:` to the output of `id -u` and `id -g`, the password to your own, and on Linux the share to your mount point.
5. **Start Phorga.** In a terminal: `cd ~/phorga && docker compose up -d`. Open `http://127.0.0.1:8080/ui` on this computer, sign in with any user name and your password, and continue with [C. Set Phorga up](#c-set-phorga-up).
6. **Keep it running.** The computer must not go to sleep (macOS: System Settings → Energy → Prevent automatic sleeping). The container only sees a share that was connected when it started: after reconnecting the share, run `docker compose restart`.

### C. Set Phorga up

1. **Pick the users.** On the configuration page, "Synology Photos users" lists everyone with a Synology Photos library. Tick the users whose phone uploads Phorga should sort, and click **Add selected users**.
2. **Check the profile.** Each user gets a profile that moves new files from `Photos/MobileBackup` into `Photos/<year>/<month>` every night at 03:00. It starts as a **dry run**: Phorga only writes to its log what it would do. Click the profile to change the time or the folder structure.
3. **Save.** Saving writes the config file and schedules the jobs at once. No restart is needed.
4. **Check the dry run.** After the first run, the status page shows how many files would be moved, and the container log lists each file (Container Manager → Container → phorga → Log, or `docker compose logs phorga`). To try it right away, set the schedule to `*/5 * * * *` (every 5 minutes) and set it back afterwards.
5. **Switch the dry run off.** When the log looks right, open the profile, untick **Dry run** and save. From the next run on, Phorga moves the files. Synology Photos shows them in their new folders once it has indexed them. After the first real run, check that the Synology Photos app on the phone does not upload the moved photos again.
6. **Optional: find duplicates.** Under "Duplicate detection jobs", add a job that compares folders and finds copies of the same photo. It also starts as a dry run, and it only ever moves duplicates to a folder you choose; it never deletes anything.

### D. Open the UI from other devices

Whoever can open the web UI can change which files Phorga moves. Protect it like this:

- **Always set a password** with `PHORGA_WEB_UI_PASSWORD`, as in the compose files above. Use a long one: there is no lockout after wrong attempts. The browser asks for it once per session (any user name). To change it, edit the compose file and recreate the container (Container Manager → Project → phorga → YAML Configurations → Edit, or `docker compose up -d`). The password is read only from the environment; it never appears in the config file or the UI.
- **Synology:** the reverse proxy (step A6) adds HTTPS, and because the container port stays on `127.0.0.1`, the proxy is the only way in. To allow only your home network, create an access control profile (Control Panel → Login Portal → Advanced → Access Control Profile) that allows e.g. `192.168.1.0/24` and denies everything else, and select it in the reverse proxy rule. With your own domain, use it as the hostname instead of `*` and assign its certificate under Control Panel → Security → Certificate → Settings.
- **Other computer:** change the port to `"8080:8080"` and run `docker compose up -d`. Then open `http://<computer address>:8080/ui`. Without HTTPS the password crosses your network unencrypted, which is only acceptable in a home network you trust; for HTTPS, put a reverse proxy such as Caddy in front.
- **Never** forward the port to the internet in your router. To use Phorga on the go, connect to your home network with a VPN (for example Synology VPN Server or Tailscale).

The UI also rejects form posts that a browser marks as coming from another website, so a page you visit cannot change Phorga's settings through your browser.

### E. Troubleshooting

| Problem | What to do |
|---|---|
| "homes folder not found" | The container does not see the homes folder. Check the volume `/volume1/homes:/homes`; on another computer, check that the share is connected. If it is mounted at another path, enter that path as the homes folder in the Synology Photos section. |
| A user is marked "no access" | Phorga's user may not read that home folder. Give the user `phorga` Read/Write on the shared folder `homes`, and check that `user:` in the compose file holds its uid and gid. |
| Saving fails with "permission denied" | The `config` folder must be writable for the user in `user:`. Mount the folder (`./config:/config`), not `config.yaml` on its own. |
| The page does not open | Check that the container is running and that its log shows no error. Synology: check the reverse proxy rule (destination `localhost`, port `8080`) and the firewall. Other computer: with `127.0.0.1` in the port, only that computer can open the page. |
| The browser keeps asking for the password | Any user name works; the password must match `PHORGA_WEB_UI_PASSWORD` exactly. After changing it, the container must be recreated. |
| Moved photos do not show up in Synology Photos | Synology Photos has to index new folders first. Wait a few minutes; if they still do not appear, start indexing again in the Synology Photos settings. |

## Configuration

YAML or JSON. Below is a short example; [`config.example.yaml`](config.example.yaml) is a complete, commented one, and the [reference](#reference) lists every field.

```yaml
organizer:
  - target: /photos/organized
    targetStructure: "{year}/{month}"   # tokens explained below
    fallbackFolder: /photos/unknown     # files without date metadata
    sources:
      - path: /photos/incoming
        mode: move                      # move | copy
        schedule: "0 2 * * *"
        duplicates:
          detectionMode: filename       # filename, filedate, exif (embedded date only); combine with + or |
          action:
            method: skip                # replace | addSuffix | moveToFolder | skip

duplicateDetection:
  - enabled: true
    schedule: "0 4 * * *"
    sources: [/photos/organized, /photos/archive]
    detectionMode: content              # content (pHash) | metadata (size + embedded date)
    action:
      method: moveToFolder
      duplicateFolder: /photos/duplicates

webUI:
  enabled: true
  address: 127.0.0.1:8080
```

`webUI` is optional. The UI is intended for **local/private deployments**. Do not expose it publicly without additional protection.

### Where Phorga looks for its config

Phorga uses the first of these that is set:

1. `--config <path>`
2. the environment variable `PHORGA_CONFIG` (set to `/config/config.yaml` in the Docker image)
3. `config.yaml` in the current working directory

Files ending in `.yaml`, `.yml` or `.json` are accepted. An invalid config stops Phorga at startup with exit code 1 and a list of all problems found.

**No config yet?** If the file does not exist, Phorga starts anyway, without jobs, and always starts its web UI so you can set it up there. The first save creates the file (and its folder). The same happens with a config that defines no jobs.

**Changing the config.** Saving in the web UI applies the changes at once; a job that is running finishes with its old settings. Only `stateDir`, `webUI.address` and turning the web UI off need a restart, and the UI says so. Edits made to the file in an editor are picked up at the next restart.

| Flag | Meaning |
|---|---|
| `--config <path>` | Config file to use (see above) |
| `--web-ui` | Start the web UI even if `webUI.enabled` is `false` |
| `--web-ui-addr <host:port>` | Listen address of the web UI, overrides `webUI.address` and `PHORGA_WEB_UI_ADDR` |

The environment variable `PHORGA_WEB_UI_ADDR` sets the web UI address like `--web-ui-addr` (the flag wins). The Docker image sets it to `0.0.0.0:8080`.

### Reference

Relative paths are resolved against the working directory. In Docker, use the paths *inside* the container (the right-hand side of your volume mounts).

**Top level**

| Field | Type | Required | Default | Meaning |
|---|---|---|---|---|
| `stateDir` | path | no | `/data/state` | Folder for Phorga's own files (see [Files in `stateDir`](#files-in-statedir)). Created if missing. |
| `organizer` | list | no | – | Organizer profiles, see below. |
| `duplicateDetection` | list | no | – | Duplicate detection jobs, see below. Without any profile or job, Phorga only runs its web UI. |
| `webUI` | object | no | off | Built-in web UI, see below. |
| `synologyPhotos` | object | no | – | Where to find Synology Photos users, see below. |

**`organizer[]`**: one entry per target folder

| Field | Type | Required | Default | Meaning |
|---|---|---|---|---|
| `name` | text | no | – | Your label for the profile, shown in the web UI and on the status page. |
| `target` | path | yes | – | Folder the organized files go into. |
| `targetStructure` | text | no | `{year}/{month}` | Subfolder template below `target`, see [tokens](#targetstructure-tokens). Unknown tokens are rejected. |
| `fallbackFolder` | path | no | – | Where files go when no date can be determined (only possible with `ignoreModTime`). Without it, such files are skipped and logged. |
| `ignoreModTime` | bool | no | `false` | Don't use the file modification time as the last date source. Files without an embedded, filename or folder date then go to `fallbackFolder` (or are skipped) instead of the folder of the day they were copied. |
| `dryRun` | bool | no | `false` | Only log what would happen. No file is changed, not even Phorga's state. |
| `ntfy.url` | URL | no | – | ntfy topic URL that receives a short summary after every run, e.g. `https://ntfy.sh/my-topic`. |
| `sources` | list | yes, ≥ 1 | – | Folders to take files from, see below. |

**`organizer[].sources[]`**: one entry per source folder, each with its own schedule

| Field | Type | Required | Default | Meaning |
|---|---|---|---|---|
| `path` | path | yes | – | Folder to read from, including all subfolders. |
| `mode` | `move` \| `copy` | yes | – | `move` deletes the source file after a verified copy; `copy` leaves it in place. |
| `schedule` | cron | yes | – | When this source runs, see [Schedules](#schedules). |
| `exclude` | list of patterns | no | – | Files and folders below `path` to leave out, see [Skipped and excluded folders](#skipped-and-excluded-folders). |
| `duplicates` | object | no | – | What to do when a file with the same name already exists in the target. Without it, such files are skipped and logged. |
| `duplicates.detectionMode` | text | yes, if `duplicates` | – | When an existing file counts as the same: `filename` (same name), `filedate` (same modification time), `exif` (same embedded capture date: EXIF in JPEG, RAW and HEIC, creation time in MP4/MOV; files without one never match). Combine with `+` (all must match) or `\|` (any may match), not both. If it doesn't match, the file is kept with a random suffix. |
| `duplicates.action.method` | `replace` \| `addSuffix` \| `moveToFolder` \| `skip` | yes, if `duplicates` | – | `replace` overwrites the existing file (only allowed if `detectionMode` compares `filedate` or `exif`, e.g. `filename+exif`); `addSuffix` keeps both (`photo-a3f9.jpg`); `moveToFolder` puts the new file into `duplicateFolder`; `skip` leaves it in the source. |
| `duplicates.action.duplicateFolder` | path | for `moveToFolder` | – | Folder for duplicates. Existing files there are never overwritten. |

**`duplicateDetection[]`**: one entry per job

| Field | Type | Required | Default | Meaning |
|---|---|---|---|---|
| `name` | text | no | – | Your label for the job, shown in the web UI and on the status page. |
| `enabled` | bool | no | `false` | Only enabled jobs are scheduled. Disabled jobs are still validated. |
| `sources` | list of paths | yes, ≥ 1 | – | Folders to compare, including all subfolders. Duplicates are found across all of them. |
| `exclude` | list of patterns | no | – | Files and folders below each source to leave out, see [Skipped and excluded folders](#skipped-and-excluded-folders). |
| `schedule` | cron | yes | – | When the job runs, see [Schedules](#schedules). |
| `detectionMode` | `content` \| `metadata` | yes | – | `content` compares what images look like (perceptual hash, then an area-by-area check so burst shots don't match; JPEG and PNG only); `metadata` compares file size and embedded capture date (much faster; works for JPEG, RAW, HEIC, MP4 and MOV; files without one never match). |
| `action.method` | `moveToFolder` \| `skip` | yes | – | `moveToFolder` moves each duplicate into `duplicateFolder` (the file with the most pixels stays, then the larger one, then the first found); `skip` only logs it. Nothing is ever deleted. |
| `action.duplicateFolder` | path | for `moveToFolder` | – | Folder for duplicates. Existing files there are never overwritten. |
| `dryRun` | bool | no | `false` | Only log what would be done. No photo is moved. |
| `ntfy.url` | URL | no | – | ntfy topic URL for a summary after every run. |

**`webUI`**

| Field | Type | Required | Default | Meaning |
|---|---|---|---|---|
| `enabled` | bool | no | `false` | Start the web UI together with Phorga (also possible with `--web-ui`). |
| `address` | `host:port` | no | `127.0.0.1:8080` | Listen address. `--web-ui-addr` and `PHORGA_WEB_UI_ADDR` override it; in the Docker image the UI therefore always listens on `0.0.0.0:8080`. |
| `browseRoots` | list of absolute paths | no | – | Folders the folder picker may show. Each must exist. Without it, the picker is off. Without a config file, `/homes` and `/photos` are used if they exist. |

**`synologyPhotos`**

| Field | Type | Required | Default | Meaning |
|---|---|---|---|---|
| `homes` | absolute path | no | `/homes` | Folder that holds the user home folders, i.e. where the NAS folder `/volume1/homes` is mounted. Must exist when set. Used to find Synology Photos users, see [Synology Photos users](#synology-photos-users). |

### Skipped and excluded folders

Both features always skip these entries below a source, with everything inside them:

- `@eaDir` (Synology thumbnails), `#recycle` (recycle bin) and `#snapshot` (Btrfs snapshots). DSM hides them in File Station, but Phorga would see them.
- Every file or folder whose name starts with `.`, e.g. `.thumbnails` or macOS `._IMG_1234.jpg` files.

This cannot be turned off. A source path that itself contains such a name (e.g. `/photos/.import`) still works.

`exclude` adds your own patterns, one per entry. Case is ignored.

| Pattern | Leaves out |
|---|---|
| `Screenshots` | every file or folder named `Screenshots`, at any depth |
| `*_edited.jpg` | every file whose name matches, at any depth |
| `2019/Scans` | exactly this path below the source |
| `*/WhatsApp` | `WhatsApp` one level below the source |
| `Export/` | folders named `Export` only (trailing `/`) |

Wildcards: `*` (any characters except `/`), `?` (one character), `[abc]`. There is no `**` and no way to re-include something. Each skipped folder is logged once as `skipping folder` at debug level, so it does not appear in the normal (info) log.

### Schedules

`schedule` is a standard 5-field cron expression: `minute hour day-of-month month day-of-week`, evaluated in the **local time of the machine** (in Docker, set `TZ`, e.g. `TZ=Europe/Zurich`). A run never overlaps with itself: if the previous run is still busy, the next one is skipped. Different jobs run **one at a time**: a job that is due while another one runs waits for it, and the status page shows it as "waiting for another job".

| Expression | Runs |
|---|---|
| `0 2 * * *` | every day at 02:00 |
| `30 3 * * 0` | every Sunday at 03:30 |
| `0 */6 * * *` | every 6 hours |
| `*/15 * * * *` | every 15 minutes |

The web UI shows the next three run times while you type.

### Files in `stateDir`

| File | Written by | Content |
|---|---|---|
| `org-<target>-<source>.state.json` | organizer, one per source | Files already handled, so later runs skip them. Never written in a dry run. |
| `run-status.json` | all jobs | Last result of each job for the status page. Dry runs are not stored. |
| `dedup-cache.json` | duplicate detection, shared by all jobs | Fingerprints (pHash or embedded date, plus a small thumbnail for look-alike candidates) of unchanged files, so later runs only read new or changed files. Written in dry runs too; it never touches a photo. Delete it to force a full scan. |

All files are written atomically. Deleting a state file makes the organizer look at all files of that source again (already organized files are recognized as duplicates).

### Web UI

Phorga can expose a small built-in configuration UI from the same process that runs the scheduler.

- Server-rendered HTML with HTMX-compatible validation/save endpoints. HTMX and the stylesheet are embedded in the binary, so no internet access is needed. Light/dark mode follows the system setting, with a manual switch in the top bar.
- UI localization: German, English, Italian, French, switchable from the top bar of every page
- Displays the currently loaded effective configuration values
- Dropdowns for fields with fixed values, and a help text under every field
- Live previews: the folder a `targetStructure` produces (for 15 March 2024), and the next 3 runs of each cron `schedule`
- Validation errors are shown next to the field they belong to
- Folder picker ("Browse…" dialog and autocomplete) for all path fields, limited to `webUI.browseRoots`
- Configuration page grouped into sections (Synology Photos users, organizer profiles, duplicate detection jobs, general, web UI, save target) with a sidebar to jump between them
- Each profile and job is a collapsed card showing its flow (source folders with mode and schedule → target folder and structure); rarely used fields are under "More options". Profiles and jobs can be given a `name`
- Duplicate detection mode as checkboxes ("same file name", "same modification date", "same capture date") with all/any, and plain-language labels in the dropdowns
- **Edit YAML**: the config file as text, validated and applied on save, comments kept (`/ui/yaml`)
- Buttons to add and remove organizer profiles, sources and duplicate detection jobs. New entries start as dry runs
- Status page (`/ui/status`): summary of running/waiting/idle jobs, then next run, running state and last result of every job, refreshed every 10 s. Each job links to its schedule on the configuration page
- Saves either to the active config file, which applies the changes at once, or to a separate config file, which changes nothing in the running Phorga
- **Help** (`/ui/help`): the [Setup guide](#setup-guide) in all four languages, with a summary of this installation (config file, Synology homes folder, password, address)
- Optional password: set `PHORGA_WEB_UI_PASSWORD` and every page asks for it (HTTP Basic auth, any user name). It is read only from the environment, never from the config. Form posts that a browser marks as coming from another website are rejected

Enable it in config:

```yaml
webUI:
  enabled: true
  address: 127.0.0.1:8080
```

Or override at startup:

```bash
go run ./cmd/phorga --config config.yaml --web-ui
go run ./cmd/phorga --config config.yaml --web-ui --web-ui-addr 127.0.0.1:9090
```

The UI shows the configuration in effect. Saving to the active file writes it and applies it right away (see [Changing the config](#where-phorga-looks-for-its-config)). Saving to a separate file only writes that file. The web UI always starts while there are no jobs.

**Folder picker.** It is off until you list the folders it may browse:

```yaml
webUI:
  enabled: true
  address: 127.0.0.1:8080
  browseRoots:
    - /photos   # absolute paths; in Docker, the mounted media folder
```

The picker shows only folders (no files, no hidden folders) below these roots, and never creates or changes anything.

#### Synology Photos users

Synology Photos keeps each user's library in `/volume1/homes/<user>/Photos`, and its phone app uploads to `Photos/MobileBackup`. If Phorga can see the homes folder (in Docker: mount `/volume1/homes:/homes`, or set `synologyPhotos.homes`), the first section of the configuration page lists every user with a `Photos` folder. Tick users and click **Add selected users**; each gets this profile, which you can still change before saving:

```yaml
- dryRun: true                       # switch off after checking the log
  target: /homes/<user>/Photos
  targetStructure: "{year}/{month}"
  sources:
    - path: /homes/<user>/Photos/MobileBackup
      mode: move
      schedule: "0 3 * * *"
      duplicates:
        detectionMode: filename+exif # same photo already there: leave it in MobileBackup
        action:
          method: skip
```

Users who already have a profile are shown as "already set up"; users whose home folder Phorga's user may not read are shown as "no access". Finding users only reads folder names.

**Status page.** The last result of each job is stored in `<stateDir>/run-status.json`, so it survives a restart. Dry runs are shown too, but only kept in memory, so a dry run never writes a file.

### TargetStructure Tokens

Phorga supports **multilingual month and day names** in `targetStructure`. Tokens are **case-sensitive** for output formatting.

#### Supported Tokens

| Token | Format | Language | Example |
|-------|--------|----------|---------|
| `{year}`, `{jahr}`, `{année}`, `{anno}` | 4-digit numeric | EN/DE/FR/IT | 2024 |
| `{month}` | 2-digit numeric | English context | 03 |
| `{monat}` | month name | Deutsch | **märz** (case: `{märz}`, `{März}`, `{MÄRZ}`) |
| `{mois}` | month name | Français | **mars** (case: `{mars}`, `{Mars}`, `{MARS}`) |
| `{mese}` | month name | Italiano | **marzo** (case: `{marzo}`, `{Marzo}`, `{MARZO}`) |
| `{monthWithName}` | numeric + English name | English | 03-march |
| `{monthNumber}` | 2-digit zero-padded | all | 03 |
| `{day}`, `{tag}`, `{jour}`, `{giorno}` | 2-digit numeric | EN/DE/FR/IT | 15 |
| `{dayNumber}` | 2-digit zero-padded | all | 15 |

#### Case Sensitivity

Month names respect the case of the placeholder token:

```yaml
# Examples with date 2024-03-15:

targetStructure: "{year}/{monat}"      # → 2024/märz
targetStructure: "{year}/{Monat}"      # → 2024/März
targetStructure: "{year}/{MONAT}"      # → 2024/MÄRZ

targetStructure: "{year}/{mois}/{jour}"           # → 2024/mars/15
targetStructure: "{year}/{Mois}/{Jour}"           # → 2024/Mars/15
targetStructure: "{year}/{monthNumber}-{Monat}"   # → 2024/03-März
```

#### Examples for Different Users

**German user:**
```yaml
targetStructure: "{year}/{Monat}"              # 2024/März
targetStructure: "{year}/{Monat}/{Tag}"        # (Tag=numeric 15)
```

**French user:**
```yaml
targetStructure: "{year}/{Mois}"               # 2024/Mars
targetStructure: "{year}/{mois}/{jour}"        # 2024/mars/15
```

**Italian user:**
```yaml
targetStructure: "{year}/{Mese}"               # 2024/Marzo
targetStructure: "{anno}/{mese}/{giorno}"      # 2024/marzo/15
```

**Mixed format user:**
```yaml
targetStructure: "{year}/{monthNumber}-{Monat}/{dayNumber}"  # 2024/03-März/15
targetStructure: "{year}/{monthNumber}_{Monat}"              # 2024/03_März
```

## Known Limits

- **Video dates** come from the movie header's creation time, which is UTC and converted to `TZ`. Some cameras write local time there instead, which shifts their videos by the zone offset. Set `TZ` in Docker, otherwise dates near midnight can land on the wrong day.
- **Content duplicate detection** compares JPEG and PNG only. HEIC, RAW and videos never match. The first run decodes every image, which takes long on a large library; later runs only read new or changed files, thanks to the cache in `<stateDir>/dedup-cache.json`.
- **Duplicate detection keeps the file with the most pixels**, then the larger file. An upscaled or enlarged copy therefore beats its original. Duplicates are only moved to `duplicateFolder`, never deleted, so review that folder before you delete anything.
- **Other file types stay in the source without a log line**, for example DNG, GIF, WebP, TIFF, AVI, M4V, 3GP and sidecars (XMP, AAE). The photo and the video of a Live Photo can land in different folders.
- **Hard links:** Phorga gives finished files their name with a hard link, so a file another program writes at the same moment is never replaced. On filesystems without hard links (exFAT, some network mounts), it falls back to a check followed by a rename, which leaves a short window.

**Before you point Phorga at your only copy of your photos:** make a backup, start with `mode: copy` and `dryRun: true`, check the log and the status page, and switch to `mode: move` only after a few clean runs.

## Build & Test

```bash
go build ./...
go vet ./...
go test ./...
```

CI runs the same three commands on every pull request ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)).

## Demo / Test Setup

Generate sample images and run Phorga locally:

```bash
go run testdata/tools/generate.go            # generate/reset test images
go run ./cmd/phorga --config data/config.yaml  # run with test config
```

Reset to original state at any time by re-running the generate command. See [`data/README.md`](data/README.md) for details.

## Project Structure

```
cmd/phorga/         Main entry point
internal/
  config/                YAML/JSON parsing, validation
  exclude/               Skipped system folders and exclude patterns
  organizer/             Date-based organization engine
  dedup/                 Duplicate detection engine
  metadata/              Embedded (EXIF, HEIC, MP4/MOV), filename & path dates, target path resolution
  fileops/               Atomic copy/move operations
  state/                 JSON state persistence (resume support)
  scheduler/             Cron wrapper, one job at a time
  notify/                ntfy.sh client
  runstatus/             Last run result per job for the status page
  synology/              Finds Synology Photos users and builds their profile
  webui/                 Built-in configuration UI (HTTP, templates, i18n, folder picker, status page)
```

## Docker

A ready-made image for **amd64 and arm64** is published on Docker Hub as [`mnrd001/phorga`](https://hub.docker.com/r/mnrd001/phorga) ([`.github/workflows/docker.yml`](.github/workflows/docker.yml)). Releases have calendar versions `YYYY.MM.MICRO`: `2026.11.0` is the first release in November 2026, `2026.11.1` the next one that month ([ADR-0024](docs/decisions/0024-calendar-versions-released-by-tag.md)).

| Tag | Meaning |
|---|---|
| `latest` | The newest release |
| `2026.11` | The newest release of that month, e.g. with fixes |
| `2026.11.0` | Exactly this release, to pin a version or go back |
| `edge` | The current state of `main`, built after every merge; not a release |
| `sha-<commit>` | A fixed build of `main`, e.g. `sha-60d4bc4` |

Before releases existed, `latest` followed every merge to `main`; that is what `edge` does now. `docker run --rm mnrd001/phorga --version` prints the version, and the start log and the web UI's top bar show it too.

```bash
docker pull mnrd001/phorga:latest
docker compose up -d       # uses the image from Docker Hub, see docker-compose.yml
```

Mount your source/target directories and the **folder** that holds `config.yaml` (e.g. `./config:/config`) as volumes. The folder must be writable for Phorga's user, so the web UI can save; a `config.yaml` mounted on its own cannot be replaced. It may be empty at first. State is persisted in `/data/state`. In `mode: move`, the source folder must be writable (no `:ro`).

The image is based on distroless (no shell) and runs as the unprivileged user `65532`. Set `user:` in `docker-compose.yml` to your NAS user (`id` in an SSH shell shows uid and gid), so Phorga can write your folders and organized files belong to you.

**Fast moves:** a move within one volume renames the file instead of copying it, but Linux only allows that within one mount. Mount the common parent once (e.g. `/volume1/photos:/photos`) instead of `incoming` and `organized` separately.

**Upgrading from an image before 2026-11-21:** older images ran as root, so an existing state volume belongs to root and the new user cannot write it. Either set `user: "0:0"` to keep running as root, or hand the volume to the user you set in `user:` once, on the NAS host: `sudo chown -R <uid>:<gid> "$(docker volume inspect -f '{{.Mountpoint}}' phorga-state)"` (with Compose, the volume name usually starts with the project name; `docker volume ls` shows it). If Phorga cannot read or write its state folder, it does not start and logs `state directory not accessible` with the folder and the uid it runs as, instead of copying everything again.

**Synology:** see the [Setup guide](#a-on-a-synology-nas-container-manager), which creates a project in Container Manager with a dedicated user and `/volume1/homes:/homes` mounted, so the web UI can list the Synology Photos users.

**Build it yourself:**

```bash
docker build -t phorga .
# Multi-arch; Go cross-compiles and the runtime stage has no RUN steps, so no QEMU is needed
docker buildx build --platform linux/amd64,linux/arm64 -t <registry>/phorga --push .
```

**Web UI in Docker:** the image sets `PHORGA_WEB_UI_ADDR=0.0.0.0:8080`, so the UI listens on port 8080 inside the container whatever `webUI.address` says. Publish the port on the host's loopback only, e.g. `-p 127.0.0.1:8080:8080` (see `ports` in [`docker-compose.yml`](docker-compose.yml)), and reach it from other devices through a reverse proxy with HTTPS. Set `PHORGA_WEB_UI_PASSWORD` whenever anyone but you can reach the port; without it, Phorga logs a warning at start-up when the UI listens beyond loopback. See [D. Open the UI from other devices](#d-open-the-ui-from-other-devices). Never publish the UI to the internet.

## Documentation

- [`docs/product.md`](docs/product.md): what Phorga does and why
- [`docs/specs/`](docs/specs/README.md): exact, testable requirements per feature
- [`docs/arc42/arc42.adoc`](docs/arc42/arc42.adoc): architecture
- [`docs/decisions/`](docs/decisions/README.md): architecture decision records

## Contributing

Start with [`AGENTS.md`](AGENTS.md). It covers the repo layout, commands, conventions, and the change workflow (spec → code + tests → docs) for people and AI agents.

## License

[Beer-Ware License](LICENSE) (Revision 42)

