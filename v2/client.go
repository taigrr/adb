package adb

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// Client wraps a single adb server. Create one with [New] and use it to
// enumerate or connect to devices. A Client is safe for concurrent use.
type Client struct {
	binary         string
	defaultTimeout time.Duration
}

// Option configures a [Client].
type Option func(*Client)

// WithBinary overrides the adb executable path. By default the binary is
// resolved from PATH.
func WithBinary(path string) Option {
	return func(c *Client) { c.binary = path }
}

// WithDefaultTimeout applies a timeout to commands whose context has no
// deadline. A zero duration (the default) disables the implicit timeout.
func WithDefaultTimeout(d time.Duration) Option {
	return func(c *Client) { c.defaultTimeout = d }
}

// New creates a Client, resolving the adb binary from PATH unless overridden
// with [WithBinary]. It returns [ErrNotInstalled] if adb cannot be found.
func New(opts ...Option) (*Client, error) {
	c := &Client{}
	for _, opt := range opts {
		opt(c)
	}
	if c.binary == "" {
		path, err := exec.LookPath("adb")
		if err != nil {
			return nil, ErrNotInstalled
		}
		c.binary = path
	}
	return c, nil
}

// run executes adb, treating a non-zero exit code as a failure. It applies the
// client's default timeout when the context has no deadline.
func (c *Client) run(ctx context.Context, args ...string) (Result, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.exec(ctx, args...)
}

// withTimeout wraps ctx with the client's default timeout when the caller has
// not set its own deadline. The returned cancel is always safe to call.
func (c *Client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.defaultTimeout > 0 {
		if _, ok := ctx.Deadline(); !ok {
			return context.WithTimeout(ctx, c.defaultTimeout)
		}
	}
	return ctx, func() {}
}

// capture runs adb and returns the raw result plus the run error, accounting
// for context cancellation but applying no failure classification. The caller
// decides how to interpret the exit code.
func (c *Client) capture(ctx context.Context, args ...string) (Result, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.binary, args...) //nolint:gosec // G204: adb args are supplied by the caller by design
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// adb auto-forks a persistent background server that inherits the child's
	// stdout/stderr pipes; without a WaitDelay, cmd.Run would block until that
	// daemon closes them (potentially forever), and context cancellation would
	// not force a return. WaitDelay force-closes the pipes shortly after the
	// direct process exits or the context is cancelled.
	cmd.WaitDelay = 5 * time.Second
	runErr := cmd.Run()

	res := Result{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
		Code:   cmd.ProcessState.ExitCode(),
	}

	// A cancelled/expired context takes precedence so callers can match
	// context.Canceled / context.DeadlineExceeded rather than a generic
	// ErrCommandFailed from the killed process.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return res, &CommandError{Args: args, Code: res.Code, Stderr: res.StderrString(), Err: ctxErr}
	}
	// WaitDelay fires when a surviving adb daemon keeps the inherited stdout
	// pipe open after the direct process already exited successfully. Treat a
	// clean exit whose only error is the forced pipe close as success.
	if errors.Is(runErr, exec.ErrWaitDelay) && res.Code == 0 {
		return res, nil
	}
	return res, runErr
}

// exec runs adb without imposing the default timeout. A non-zero exit code, a
// known adb stderr message, or a spawn failure yields a *CommandError wrapping
// the appropriate cause.
func (c *Client) exec(ctx context.Context, args ...string) (Result, error) {
	res, runErr := c.capture(ctx, args...)
	if _, ok := errors.AsType[*CommandError](runErr); ok {
		return res, runErr
	}
	if cause := classify(res, runErr); cause != nil {
		return res, &CommandError{Args: args, Code: res.Code, Stderr: res.StderrString(), Err: cause}
	}
	return res, nil
}

// execShell runs adb, tolerating a non-zero exit code (which for `adb shell`
// carries the device command's own exit status) so it is reported via
// [Result.Code] rather than as an error. Known adb stderr failures and spawn
// failures are still returned as errors.
func (c *Client) execShell(ctx context.Context, args ...string) (Result, error) {
	res, runErr := c.capture(ctx, args...)
	if _, ok := errors.AsType[*CommandError](runErr); ok {
		return res, runErr
	}
	if cause := filterStderr(res.StderrString()); cause != nil {
		return res, &CommandError{Args: args, Code: res.Code, Stderr: res.StderrString(), Err: cause}
	}
	// A non-ExitError run error means adb itself failed to run.
	if _, ok := errors.AsType[*exec.ExitError](runErr); runErr != nil && !ok {
		return res, &CommandError{Args: args, Code: res.Code, Stderr: res.StderrString(), Err: runErr}
	}
	return res, nil
}

// classify maps a completed adb invocation to a sentinel error, or nil on
// success. It inspects stderr for known messages, then the exit code.
//
// It deliberately does NOT scan output for adb-reported failures: that check
// (outputFailure) is applied only by the specific command wrappers that need
// it, so that general-purpose commands like [Device.Shell] can return output
// that legitimately contains words like "Error:" or "Exception:".
func classify(res Result, runErr error) error {
	if err := filterStderr(res.StderrString()); err != nil {
		return err
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) && res.Code != 0 {
		return ErrCommandFailed
	}
	return runErr
}

// filterStderr maps known adb stderr messages to typed sentinel errors.
func filterStderr(stderr string) error {
	switch {
	case stderr == "":
		return nil
	case strings.Contains(stderr, "device not found"):
		return ErrDeviceNotFound
	case strings.Contains(stderr, "device offline"):
		return ErrDeviceOffline
	case strings.Contains(stderr, "device unauthorized"):
		return ErrDeviceUnauthorized
	case strings.Contains(stderr, "Connection refused"):
		return ErrConnectionRefused
	case strings.Contains(stderr, "more than one device"):
		return ErrMoreThanOneDevice
	default:
		return nil
	}
}

// outputFailure detects failures that adb (and the on-device pm/am tools)
// report through their output while still exiting 0. Matching is anchored to
// line prefixes (and an "Exception:" marker for Java stack traces) rather than
// a blanket substring search so that data echoed back by adb — such as a
// component name containing "Exception" or an APK path containing "Error" —
// does not trigger a false positive.
func outputFailure(streams ...string) error {
	for _, s := range streams {
		for line := range strings.SplitSeq(s, "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "Failure"),
				strings.HasPrefix(line, "Error:"),
				strings.HasPrefix(line, "Error type"),
				strings.HasPrefix(line, "Warning: Activity not started"),
				strings.Contains(line, "Exception:"):
				return ErrCommandFailed
			}
		}
	}
	return nil
}
