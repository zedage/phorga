package notify

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// Spec: NTF-001
func TestNewNil(t *testing.T) {
	c := New("")
	if c != nil {
		t.Error("expected nil client for empty URL")
	}
}

// Spec: NTF-001
func TestNewWithURL(t *testing.T) {
	c := New("https://ntfy.sh/test")
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.url != "https://ntfy.sh/test" {
		t.Errorf("unexpected URL: %s", c.url)
	}
}

// Spec: NTF-001
func TestSendSummary_Success(t *testing.T) {
	var called int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.StoreInt32(&called, 1)
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Title") == "" {
			t.Error("expected Title header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(server.URL)
	err := c.send("Test Title", "Test message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Spec: NTF-002
func TestSendSummary_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := New(server.URL)
	err := c.send("Test", "msg")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// Spec: NTF-001
func TestSendSummary_NilClient(t *testing.T) {
	var c *Client
	// Should not panic
	c.SendSummary("title", "message")
}

// Spec: NTF-001
func TestFormatOrganizerSummary(t *testing.T) {
	title, msg := FormatOrganizerSummary("/photos", 10, 2, 0, 5*time.Second)
	if title == "" || msg == "" {
		t.Error("expected non-empty title and message")
	}
	if len(msg) < 10 {
		t.Error("message seems too short")
	}
}

// Spec: NTF-001
func TestFormatOrganizerSummary_WithErrors(t *testing.T) {
	title, _ := FormatOrganizerSummary("/photos", 10, 2, 3, 5*time.Second)
	if title == "" {
		t.Error("expected non-empty title")
	}
}

// Spec: NTF-001
func TestFormatDedupSummary(t *testing.T) {
	title, msg := FormatDedupSummary(100, 5, 5, 0, 30*time.Second)
	if title == "" || msg == "" {
		t.Error("expected non-empty title and message")
	}
}

