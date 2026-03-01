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

// maxOutputSize is the maximum amount of stdout/stderr captured from
// commands. Prevents unbounded memory usage from chatty processes.
const maxOutputSize = 1 << 20 // 1 MiB

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
		return Result{Output: err.Error(), Err: fmt.Errorf("performing request: %w", err)}
	}
	defer resp.Body.Close()

	// Read body (capped at 1 MiB) so output-matching rules can inspect it.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Result{
			Code:   resp.StatusCode,
			Output: fmt.Sprintf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode)),
			Err:    fmt.Errorf("reading response body: %w", err),
		}
	}

	statusLine := fmt.Sprintf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	output := statusLine
	if len(body) > 0 {
		output = statusLine + "\n\n" + string(body)
	}

	return Result{
		Code:   resp.StatusCode,
		Output: output,
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

	cmd := exec.CommandContext(ctx, "bash", "-c", "set -o pipefail; "+c.Command)

	stdout := &limitedBuffer{max: maxOutputSize}
	stderr := &limitedBuffer{max: maxOutputSize}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

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

// limitedBuffer is an io.Writer that silently discards writes once the
// buffer exceeds max bytes. This prevents runaway command output from
// consuming unbounded memory.
type limitedBuffer struct {
	buf bytes.Buffer
	max int
}

func (lb *limitedBuffer) Write(p []byte) (int, error) {
	remaining := lb.max - lb.buf.Len()
	if remaining <= 0 {
		return len(p), nil // discard silently
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	return lb.buf.Write(p)
}

func (lb *limitedBuffer) String() string {
	return lb.buf.String()
}
