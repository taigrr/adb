package adb

import (
	"errors"
)

// When an execution should have data but has none, but the exact error is
// indeterminite, this error is returned
var ErrStdoutEmpty = errors.New("stdout expected to contain data but was empty")
