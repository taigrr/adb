package adb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

var (
	adb     string
	adbOnce sync.Once
)

func findADB() {
	path, err := exec.LookPath("adb")
	if err != nil {
		adb = ""
		return
	}
	adb = path
}

func execute(ctx context.Context, args []string) (string, string, int, error) {
	adbOnce.Do(findADB)

	if adb == "" {
		return "", "", -1, ErrNotInstalled
	}

	var (
		stderr bytes.Buffer
		stdout bytes.Buffer
	)

	// Shelling out to adb with caller-supplied arguments is the entire
	// purpose of this package, so the variable command is intentional.
	cmd := exec.CommandContext(ctx, adb, args...) //nolint:gosec // G204: adb args are supplied by the caller by design
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := stdout.String()
	warnings := stderr.String()
	code := cmd.ProcessState.ExitCode()

	customErr := filterErr(warnings)
	if customErr != nil {
		err = customErr
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && code != 0 {
		err = fmt.Errorf("received error code %d for stderr `%s`: %w", code, warnings, ErrUnspecified)
	}

	return output, warnings, code, err
}

// filterErr matches known output strings against the stderr.
//
// The inferred error type is then returned.
func filterErr(stderr string) error {
	if stderr == "" {
		return nil
	}
	switch {
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

// outputFailure detects failures that adb reports through its output while
// still exiting 0. adb (and the on-device pm/am tools) routinely print a
// failure line and return a zero exit code, so callers that only inspect the
// exit status would treat these as success.
//
// Matching is anchored to line prefixes (and, for Java stack traces, an
// "Exception:" marker) rather than a blanket substring search so that data
// echoed back by adb — such as a component name containing "Exception" or an
// APK path containing "Error" — does not trigger a false positive.
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
