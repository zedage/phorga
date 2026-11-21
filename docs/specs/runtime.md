# Spec: Runtime (`RUN`)

> Human explanation: [product.md › Starting and stopping](../product.md#starting-and-stopping) · Architecture: [arc42 §6 Shutdown](../arc42/arc42.adoc) · Format: [README](README.md)

Process-wide behavior of the Phorga daemon that does not belong to one feature: how jobs share the process, and how it stops.

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

Phorga MUST run at most one job (organizer source or duplicate detection job) at a time. When a job is due while another job runs, it MUST wait until that job has finished and then start, and log once at info level `waiting for another job to finish` with its name and the running job. While it waits, the status page shows it as waiting (WEB-011). A job MUST still never overlap with itself: if its schedule fires while it runs or waits, that run is skipped (ORG-010). On shutdown (RUN-001), waiting jobs MUST NOT start.

- **WHEN** a duplicate detection job is due at 02:00 while the organizer run that started at 01:55 is still busy
  **THEN** the duplicate detection waits, the log names both jobs, and it starts when the organizer run has finished
- **WHEN** Phorga is told to stop while a job waits
  **THEN** that job does not start, and Phorga exits as in RUN-001

**Code:** `internal/scheduler/scheduler.go`, `cmd/phorga/main.go`
**Tests:** `internal/scheduler/serial_test.go#TestJobsRunOneAtATime`, `internal/scheduler/serial_test.go#TestWaitingJobDoesNotStartAfterStop`, `internal/webui/usability_test.go#TestStatusTableShowsWaitingJob`
**Notes:** Before this requirement, different jobs ran in parallel, so an organizer and a duplicate detection job on overlapping folders could act on the same file, and two sources of one profile could write into the same target folder at once. Running in parallel gained little on a NAS with spinning disks. See [ADR-0020](../decisions/0020-run-one-job-at-a-time.md).
