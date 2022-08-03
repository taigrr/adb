package adb

import (
	"errors"
)

var (
	// When an execution should have data but has none, but the exact error is
	// indeterminite, this error is returned
	ErrStdoutEmpty         = errors.New("stdout expected to contain data but was empty")
	ErrNotInstalled        = errors.New("adb is not installed or not in PATH")
	ErrCoordinatesNotFound = errors.New("coordinates for an input event are missing")
	ErrConnUSB             = errors.New("cannot call connect to device using USB")
	ErrResolutionParseFail = errors.New("failed to parse screen size from input text")
	ErrUnspecified         = errors.New("an unknown error has occurred, please open an issue on GitHub")
)
