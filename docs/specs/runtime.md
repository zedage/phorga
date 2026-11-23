# Spec: Runtime (`RUN`)

> Human explanation: [product.md › Starting and stopping](../product.md#starting-and-stopping) · Architecture: [arc42 §6 Shutdown](../arc42/arc42.adoc) · Format: [README](README.md)

Process-wide behavior of the Phorga daemon that does not belong to one feature: how jobs share the process, how it checks its own files at start, and how it stops.

---

### RUN-001 Prompt, graceful shutdown
**Status:** implemented

On the first SIGINT (Ctrl+C) or SIGTERM (e.g. `docker stop`), Phorga MUST:

1. Stop starting new job runs.
2. Log the names of the jobs that are running at that moment.
3. Tell every running job to stop. Organizer runs (ORG-021) and duplicate detection runs (DUP-010) MUST react between two files, so a normal file takes at most a few seconds.
4. Wait until the running jobs have returned, and repeat every 10 seconds which jobs it is still waiting for.
5. Shut down the web UI (WEB-001) and exit with status 0.

A run that was stopped this way MUST NOT be recorded as the job's last result on the status page (WEB-011), and MUST NOT send a notification (NTF-001), because its counters describe an incomplete run.

A second SIGINT or SIGTERM while Phorga is waiting MUST end the process immediately.

- **WHEN** a duplicate detection run is scanning 7,000 images and Ctrl+C is pressed
  **THEN** the log names the job, the scan stops after the current image, no file is moved, and Phorga exits within a few seconds
- **WHEN** a job ignores the stop request and keeps running
  **THEN** Phorga logs every 10 seconds that it is still waiting for that job
- **WHEN** Ctrl+C is pressed a second time while Phorga is waiting
  **THEN** the process ends at once
- **WHEN** an organizer run is stopped by shutdown
  **THEN** the status page keeps showing the previous completed run, and no ntfy message is sent

**Code:** `cmd/phorga/main.go`, `internal/scheduler/scheduler.go`
**Tests:** `internal/scheduler/shutdown_test.go#TestStopCancelsRunningJob`, `internal/scheduler/shutdown_test.go#TestStopKeepsLoggingWhileWaiting`, `internal/scheduler/shutdown_test.go#TestNoJobStartsAfterStop`, `internal/scheduler/shutdown_test.go#TestStopTwiceIsHarmless`, `cmd/phorga/status_test.go#TestCancelledRunIsNotRecorded`
**Notes:** The second signal works by unregistering Phorga's signal handler after the first signal, which restores the operating system's default (terminate). That part needs a real terminal and is checked manually. Before this requirement, Phorga waited for running jobs without telling them to stop, and Go silently swallowed every further Ctrl+C, so a long duplicate detection run made Phorga look hung for many minutes.

### RUN-002 One job at a time
**Status:** implemented

Phorga MUST run at most one job (organizer source or duplicate detection job) at a time. When a job is due while another job runs, it MUST wait until that job has finished and then start, and log once at info level `waiting for another job to finish` with its name and the running job. While it waits, the status page shows it as waiting (WEB-011). A job MUST still never overlap with itself: if its schedule fires while it runs or waits, that run is skipped (ORG-010). On shutdown (RUN-001), waiting jobs MUST NOT start. Neither may a waiting run whose job was removed or replaced by a saved configuration in the meantime (CFG-008).

- **WHEN** a duplicate detection job is due at 02:00 while the organizer run that started at 01:55 is still busy
  **THEN** the duplicate detection waits, the log names both jobs, and it starts when the organizer run has finished
- **WHEN** Phorga is told to stop while a job waits
  **THEN** that job does not start, and Phorga exits as in RUN-001
- **WHEN** a job waits and its configuration is saved with changed settings
  **THEN** the waiting run does not start; the next run uses the new settings

**Code:** `internal/scheduler/scheduler.go`, `cmd/phorga/main.go`
**Tests:** `internal/scheduler/serial_test.go#TestJobsRunOneAtATime`, `internal/scheduler/serial_test.go#TestWaitingJobDoesNotStartAfterStop`, `internal/scheduler/unregister_test.go#TestWaitingRunOfUnregisteredJobDoesNotStart`, `internal/scheduler/unregister_test.go#TestWaitingRunOfReplacedJobDoesNotStart`, `internal/webui/usability_test.go#TestStatusTableShowsWaitingJob`
**Notes:** Before this requirement, different jobs ran in parallel, so an organizer and a duplicate detection job on overlapping folders could act on the same file, and two sources of one profile could write into the same target folder at once. Running in parallel gained little on a NAS with spinning disks. See [ADR-0020](../decisions/0020-run-one-job-at-a-time.md).

### RUN-003 Access to the state folder is checked at start
**Status:** implemented

At start, after creating `stateDir` (CFG-002), Phorga MUST check that the user it runs as can read every file below `stateDir` and write in every folder of it. State, run status and cache files are replaced through a temp file in their folder, so the folder needs write access. If the check fails, Phorga MUST NOT schedule any job. It MUST log an error that names the folder or file, the cause, and the user and group it runs as (where the platform has them), and exit with code 1.

When the web UI starts, Phorga SHOULD also check that a save can replace the config file: its folder must be writable, and an existing file too. If not, it logs a warning (`saving in the web UI will fail`) and keeps running.

- **WHEN** `stateDir` holds state files that belong to root and Phorga runs as user 1026
  **THEN** Phorga exits with code 1, and the log names the file and `uid 1026`
- **WHEN** `stateDir` is accessible
  **THEN** Phorga starts, and no probe file is left in it
- **WHEN** the web UI is on and `config.yaml` is mounted read-only
  **THEN** a warning is logged at start, and the jobs run normally

**Code:** `cmd/phorga/access.go`, `cmd/phorga/main.go`
**Tests:** `cmd/phorga/access_test.go#TestCheckStateAccess_AccessibleDirLeavesNothingBehind`, `cmd/phorga/access_test.go#TestRun_StateDirNotWritableExitsNonZero`, `cmd/phorga/access_test.go#TestRun_UnreadableStateFileExitsNonZero`, `cmd/phorga/access_test.go#TestCheckConfigWritable`, `cmd/phorga/access_test.go#TestOrganizerJob_UnreadableStateTransfersNothing`
**Notes:** Prompted by the switch to a non-root image (`gcr.io/distroless/static-debian13:nonroot`): state written by the old root image could not be saved any more, so every `copy` run copied all files again, with `addSuffix` as new copies every day. The check creates and removes one probe file per folder; that is not a state file, so it also runs before dry runs (ORG-014).

### RUN-004 Version information
**Status:** implemented

Phorga MUST know its version and show it in three places: `phorga --version` prints `phorga <version>` and exits with code 0 without loading a config; the start log line `phorga starting` has the attribute `version`; the web UI shows the version next to the name in the top bar of every page. A release build has the calendar version of its tag, `YYYY.MM.MICRO` (ADR-0024), set at build time with `-ldflags "-X main.version=…"` (Docker build argument `VERSION`). A build without it reports `dev`, plus the commit from the Go build info if there is one (`dev+1a2b3c4`, `.dirty` with uncommitted changes).

- **WHEN** the image built from tag `v2026.11.0` runs with `--version`
  **THEN** it prints `phorga 2026.11.0`
- **WHEN** Phorga is started with `go run ./cmd/phorga`
  **THEN** the start log shows `"version":"dev+<commit>"` or `"dev"`
- **WHEN** the web UI is opened
  **THEN** the top bar shows the version next to `Phorga`

**Code:** `cmd/phorga/version.go`, `cmd/phorga/main.go`, `internal/webui/server.go`, `internal/webui/templates/layout.html`, `Dockerfile`, `.github/workflows/docker.yml`
**Tests:** `cmd/phorga/version_test.go#TestRun_VersionFlagPrintsVersion`, `cmd/phorga/version_test.go#TestRun_StartLogNamesVersion`, `cmd/phorga/version_test.go#TestCurrentVersionWithoutLdflags`, `internal/webui/version_test.go#TestTopBarShowsVersion`, `internal/webui/version_test.go#TestTopBarWithoutVersion`
**Gap:** The image tags and the GitHub release of the Docker workflow run only on a real tag push and have no automated test.
**Notes:** See [ADR-0024](../decisions/0024-calendar-versions-released-by-tag.md).
