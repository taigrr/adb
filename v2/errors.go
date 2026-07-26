// Package adb provides idiomatic, context-aware Go bindings for the Android
// Debug Bridge (adb) command-line tool. The adb binary must be installed and
// available on PATH.
//
// The entry point is [Client], which wraps a single adb server. Obtain devices
// via [Client.Devices] or [Client.Connect], then run commands against the
// returned [Device] values.
package adb

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrNotInstalled is returned when adb cannot be found in PATH.
	ErrNotInstalled = errors.New("adb is not installed or not in PATH")
	// ErrStdoutEmpty is returned when a command should produce output but did not.
	ErrStdoutEmpty = errors.New("stdout expected to contain data but was empty")
	// ErrCoordinatesNotFound is returned when touch-event coordinates are missing.
	ErrCoordinatesNotFound = errors.New("coordinates for an input event are missing")
	// ErrNotNetworkDevice is returned when a network-only operation
	// (connect/disconnect/reconnect) is attempted on a USB device.
	ErrNotNetworkDevice = errors.New("operation requires a network device")
	// ErrResolutionParseFail is returned when screen-resolution output cannot be parsed.
	ErrResolutionParseFail = errors.New("failed to parse screen size from adb output")
	// ErrDestExists is returned when a destination file already exists.
	ErrDestExists = errors.New("destination file already exists")
	// ErrDeviceNotFound is returned when the target device cannot be found.
	ErrDeviceNotFound = errors.New("device not found")
	// ErrDeviceOffline is returned when the target device is offline.
	ErrDeviceOffline = errors.New("device offline")
	// ErrDeviceUnauthorized is returned when the device has not authorized debugging.
	ErrDeviceUnauthorized = errors.New("device unauthorized; check the confirmation dialog on the device")
	// ErrConnectionRefused is returned when the connection to a device is refused.
	ErrConnectionRefused = errors.New("connection refused")
	// ErrMoreThanOneDevice is returned when multiple devices are connected and no serial is specified.
	ErrMoreThanOneDevice = errors.New("more than one device/emulator; select a device")
	// ErrCommandFailed is returned when adb exits successfully but its output
	// reports a failure (for example `Failure [INSTALL_FAILED_*]` or an
	// on-device Exception).
	ErrCommandFailed = errors.New("adb reported a failure in its output")
)

// CommandError describes a failed adb invocation. It exposes the arguments,
// exit code, and captured stderr so callers can inspect the details, while
// remaining matchable against the sentinel errors above via [errors.Is].
type CommandError struct {
	// Args is the argument vector passed to adb (without the binary name).
	Args []string
	// Code is the process exit code (-1 if the process never ran).
	Code int
	// Stderr is the captured standard error output.
	Stderr string
	// Err is the underlying cause, and is what [errors.Is]/[errors.As] unwrap to.
	Err error
}

func (e *CommandError) Error() string {
	detail := e.Err.Error()
	if e.Stderr != "" {
		detail = strings.TrimSpace(e.Stderr)
	}
	return fmt.Sprintf("adb %s: exit %d: %s", strings.Join(e.Args, " "), e.Code, detail)
}

// Unwrap returns the underlying error so [errors.Is] and [errors.As] can match
// against the package sentinels.
func (e *CommandError) Unwrap() error { return e.Err }
