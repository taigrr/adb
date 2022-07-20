package adb

import (
	"errors"
)

var (
	// When an execution should have data but has none, but the exact error is
	// indeterminite, this error is returned
	ErrStdoutEmpty  = errors.New("stdout expected to contain data but was empty")
	ErrNotInstalled = errors.New("adb is not installed or not in PATH")
	ErrUnspecified  = errors.New("an unknown error has occurred, please open an issue on GitHub")
)
