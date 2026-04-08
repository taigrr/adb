package adb

import (
	"bytes"
	"context"
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

	cmd := exec.CommandContext(ctx, adb, args...)
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
	if _, ok := err.(*exec.ExitError); ok && code != 0 {
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
