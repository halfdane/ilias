package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPChecker_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	checker := &HTTPChecker{URL: server.URL, Timeout: 5 * time.Second}
	result := checker.Check(context.Background())

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Code != 200 {
		t.Errorf("code = %d, want 200", result.Code)
	}
	if result.Output != `{"status": "ok"}` {
		t.Errorf("output = %q, want %q", result.Output, `{"status": "ok"}`)
	}
}

func TestHTTPChecker_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	checker := &HTTPChecker{URL: server.URL, Timeout: 5 * time.Second}
	result := checker.Check(context.Background())

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Code != 500 {
		t.Errorf("code = %d, want 500", result.Code)
	}
}

func TestHTTPChecker_ConnectionRefused(t *testing.T) {
	checker := &HTTPChecker{URL: "http://127.0.0.1:1", Timeout: 2 * time.Second}
	result := checker.Check(context.Background())

	if result.Err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestHTTPChecker_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	checker := &HTTPChecker{URL: server.URL, Timeout: 5 * time.Second}
	result := checker.Check(ctx)

	if result.Err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestCommandChecker_Success(t *testing.T) {
	checker := &CommandChecker{Command: "echo hello world", Timeout: 5 * time.Second}
	result := checker.Check(context.Background())

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Code != 0 {
		t.Errorf("code = %d, want 0", result.Code)
	}
	if result.Output != "hello world" {
		t.Errorf("output = %q, want %q", result.Output, "hello world")
	}
}

func TestCommandChecker_NonZeroExit(t *testing.T) {
	checker := &CommandChecker{Command: "exit 42", Timeout: 5 * time.Second}
	result := checker.Check(context.Background())

	// Non-zero exit is NOT an error — it's a valid result
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Code != 42 {
		t.Errorf("code = %d, want 42", result.Code)
	}
}

func TestCommandChecker_CommandNotFound(t *testing.T) {
	checker := &CommandChecker{
		Command: "/nonexistent/command/xyz",
		Timeout: 5 * time.Second,
	}
	result := checker.Check(context.Background())

	// Command not found should be an error with code -1
	if result.Code != -1 && result.Err == nil {
		// Some systems return exit code 127 for command not found via sh -c
		if result.Code != 127 {
			t.Errorf("expected error or code 127/-1, got code=%d err=%v", result.Code, result.Err)
		}
	}
}

func TestCommandChecker_Timeout(t *testing.T) {
	checker := &CommandChecker{
		Command: "sleep 60",
		Timeout: 100 * time.Millisecond,
	}
	result := checker.Check(context.Background())

	if result.Err == nil && result.Code == 0 {
		t.Fatal("expected error or non-zero exit for timeout")
	}
}

func TestNewChecker(t *testing.T) {
	_, err := NewChecker("http", "https://example.com", 0)
	if err != nil {
		t.Fatalf("unexpected error for http: %v", err)
	}

	_, err = NewChecker("command", "echo test", 0)
	if err != nil {
		t.Fatalf("unexpected error for command: %v", err)
	}

	_, err = NewChecker("ftp", "ftp://example.com", 0)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}
