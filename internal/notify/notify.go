package notify

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 10 * time.Second

// Client sends notifications via ntfy.sh.
type Client struct {
	url        string
	httpClient *http.Client
}

// New creates a new notification client. Returns nil if url is empty.
func New(url string) *Client {
	if url == "" {
		return nil
	}
	return &Client{
		url: url,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// SendSummary sends a run summary notification (fire-and-forget).
func (c *Client) SendSummary(title, message string) {
	if c == nil {
		return
	}

	go func() {
		if err := c.send(title, message); err != nil {
			slog.Warn("notification delivery failed", "url", c.url, "error", err)
		}
	}()
}

// send performs the HTTP POST to ntfy.sh.
func (c *Client) send(title, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Title", title)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned status %d", resp.StatusCode)
	}

	return nil
}

// FormatOrganizerSummary builds a human-readable organizer run summary.
func FormatOrganizerSummary(source string, processed, skipped, errors int, duration time.Duration) (string, string) {
	title := "Phorganizer: Organizer Run"
	if errors > 0 {
		title += " ⚠️"
	} else {
		title += " ✅"
	}

	msg := fmt.Sprintf("Source: %s\nProcessed: %d\nSkipped: %d\nErrors: %d\nDuration: %s",
		source, processed, skipped, errors, duration.Round(time.Second))

	return title, msg
}

// FormatDedupSummary builds a human-readable duplicate detection run summary.
func FormatDedupSummary(totalFiles, duplicates, actions, errors int, duration time.Duration) (string, string) {
	title := "Phorganizer: Duplicate Detection"
	if errors > 0 {
		title += " ⚠️"
	} else {
		title += " ✅"
	}

	msg := fmt.Sprintf("Files scanned: %d\nDuplicates found: %d\nActions applied: %d\nErrors: %d\nDuration: %s",
		totalFiles, duplicates, actions, errors, duration.Round(time.Second))

	return title, msg
}

