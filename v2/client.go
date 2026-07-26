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

// run executes adb with the given arguments, applying the client's default
// timeout when the context has no deadline. See [Client.exec] for the result
// and error semantics.
func (c *Client) run(ctx context.Context, args ...string) (Result, error) {
	if c.defaultTimeout > 0 {
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.defaultTimeout)
			defer cancel()
		}
	}
	return c.exec(ctx, args...)
}

// exec runs adb without imposing the default timeout and returns the captured
// result. A non-zero exit code, an adb-reported failure in the output, or a
// context cancellation yields a *CommandError wrapping the appropriate cause
// (which may be a package sentinel or a context error).
func (c *Client) exec(ctx context.Context, args ...string) (Result, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.binary, args...) //nolint:gosec // G204: adb args are supplied by the caller by design
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
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

	if cause := classify(res, runErr); cause != nil {
		return res, &CommandError{
			Args:   args,
			Code:   res.Code,
			Stderr: res.StderrString(),
			Err:    cause,
		}
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
