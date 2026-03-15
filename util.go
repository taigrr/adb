package adb

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
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
// TODO: implement
func filterErr(stderr string) error {
	return nil
}
