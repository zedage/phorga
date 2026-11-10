# Spec: Notifications (`NTF`)

> Human explanation: [product.md › Notifications](../product.md#notifications) · Format: [README](README.md)

---

### NTF-001 Optional ntfy run summary
**Status:** implemented

When an organizer profile or duplicate detection job has `ntfy.url` set, the system MUST send a summary to that URL (HTTP POST, `Title` header, plain-text body) after every run. Without `ntfy.url`, nothing is sent and no error occurs.

Content:
- Organizer: title `Phorga: Organizer Run ✅` (or `⚠️` if errors > 0); body lists source, processed, skipped, errors and duration
- Duplicate detection: title `Phorga: Duplicate Detection ✅`/`⚠️`; body lists files scanned, duplicates found, actions applied, errors and duration

- **WHEN** an organizer run finishes with 0 errors and `ntfy.url` is set
  **THEN** a summary titled `Phorga: Organizer Run ✅` is sent
- **WHEN** a run has errors
  **THEN** the title ends with `⚠️` and the body contains the error count
- **WHEN** `ntfy.url` is not set
  **THEN** nothing is sent

**Code:** `internal/notify/notify.go`, `cmd/phorga/main.go`
**Tests:** `internal/notify/notify_test.go#TestNewNil`, `internal/notify/notify_test.go#TestNewWithURL`, `internal/notify/notify_test.go#TestSendSummary_Success`, `internal/notify/notify_test.go#TestSendSummary_NilClient`, `internal/notify/notify_test.go#TestFormatOrganizerSummary`, `internal/notify/notify_test.go#TestFormatOrganizerSummary_WithErrors`, `internal/notify/notify_test.go#TestFormatDedupSummary`
**Notes:** The old spec said a failure notification should include the "affected files". The code sends only counts, and this spec now matches the code. Listing the files would be a new requirement.

### NTF-002 Notification failures do not affect runs
**Status:** implemented

Sending MUST be fire-and-forget: asynchronous, with a timeout and no retry. A failed delivery MUST only log a warning.

- **WHEN** the ntfy endpoint returns HTTP 500 or cannot be reached
  **THEN** a warning is logged and the run result is unchanged

**Code:** `internal/notify/notify.go`
**Tests:** `internal/notify/notify_test.go#TestSendSummary_ServerError`
