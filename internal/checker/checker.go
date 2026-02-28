// Package checker executes HTTP and command checks, returning their results.
package checker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

// Result holds the outcome of a check execution.
type Result struct {
	Code   int    // HTTP status code or process exit code
	Output string // response body or stdout
	Err    error  // non-nil if the check itself failed (timeout, DNS, etc.)
}

// Checker executes a check and returns its result.
type Checker interface {
	Check(ctx context.Context) Result
}

// NewChecker creates the appropriate checker based on check type.
func NewChecker(checkType, target string, timeout time.Duration) (Checker, error) {
	if timeout == 0 {
		timeout = defaultTimeout
	}

	switch checkType {
	case "http":
		return &HTTPChecker{URL: target, Timeout: timeout}, nil
	case "command":
		return &CommandChecker{Command: target, Timeout: timeout}, nil
	default:
		return nil, fmt.Errorf("unknown check type: %q", checkType)
	}
}

// HTTPChecker performs an HTTP GET request and returns the status code and body.
type HTTPChecker struct {
	URL     string
	Timeout time.Duration
	// Client is optional; if nil, a default client with the configured timeout is used.
	Client *http.Client
}

// Check performs the HTTP request.
func (c *HTTPChecker) Check(ctx context.Context) Result {
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: c.Timeout}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return Result{Err: fmt.Errorf("creating request: %w", err)}
	}

	resp, err := client.Do(req)
	if err != nil {
		return Result{Err: fmt.Errorf("performing request: %w", err)}
	}
	defer resp.Body.Close()

	// Read body with a size limit to avoid unbounded memory usage.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		return Result{Code: resp.StatusCode, Err: fmt.Errorf("reading response body: %w", err)}
	}

	return Result{
		Code:   resp.StatusCode,
		Output: string(body),
	}
}

// CommandChecker executes a shell command and returns the exit code and stdout.
type CommandChecker struct {
	Command string
	Timeout time.Duration
}

// Check executes the command.
func (c *CommandChecker) Check(ctx context.Context) Result {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", c.Command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Command couldn't be executed at all (not found, permission, timeout, etc.)
			combined := strings.TrimSpace(stdout.String() + stderr.String())
			return Result{
				Code:   -1,
				Output: combined,
				Err:    fmt.Errorf("executing command: %w", err),
			}
		}
	}

	// Include stderr alongside stdout so failures always have diagnostic output.
	out := stdout.String()
	if se := stderr.String(); se != "" {
		if out != "" {
			out += "\n" + se
		} else {
			out = se
		}
	}

	return Result{
		Code:   exitCode,
		Output: strings.TrimSpace(out),
	}
}
