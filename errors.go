package adb

import (
	"errors"
)

var (
	// ErrStdoutEmpty is returned when an execution should have data but has none.
	ErrStdoutEmpty = errors.New("stdout expected to contain data but was empty")
	// ErrNotInstalled is returned when adb cannot be found in PATH.
	ErrNotInstalled = errors.New("adb is not installed or not in PATH")
	// ErrCoordinatesNotFound is returned when touch event coordinates are missing.
	ErrCoordinatesNotFound = errors.New("coordinates for an input event are missing")
	// ErrConnUSB is returned when connect/disconnect is called on a USB device.
	ErrConnUSB = errors.New("cannot call connect/disconnect to device using USB")
	// ErrResolutionParseFail is returned when screen resolution output cannot be parsed.
	ErrResolutionParseFail = errors.New("failed to parse screen size from input text")
	// ErrDestExists is returned when a pull destination file already exists.
	ErrDestExists = errors.New("destination file already exists")
	// ErrUnspecified is returned when the exact error cannot be determined.
	ErrUnspecified = errors.New("an unknown error has occurred, please open an issue on GitHub")
)
