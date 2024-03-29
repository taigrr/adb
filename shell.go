package adb

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"github.com/google/shlex"
)

// Shell allows you to run an arbitrary shell command against a device.
//
// This function is useful if you need to run an obscure shell command or if
// you require functionality not provided by the exposed functions here.
// Instead of using Shell, please consider submitting a PR with the functionality
// you require.
func (d Device) Shell(ctx context.Context, command string) (stdout string, stderr string, ErrCode int, err error) {
	cmd, err := shlex.Split(command)
	if err != nil {
		return "", "", 1, err
	}
	prefix := []string{"-s", string(d.SerialNo), "shell"}
	cmd = append(prefix, cmd...)
	stdout, stderr, errcode, err := execute(ctx, cmd)
	return stdout, stderr, errcode, err
}

// adb shell wm size
// Physical size: 1440x3120
func (d Device) GetScreenResolution(ctx context.Context) (res Resolution, err error) {
	cmd := []string{"-s", string(d.SerialNo), "shell", "wm", "size"}
	stdout, _, _, err := execute(ctx, cmd)
	if err != nil {
		return Resolution{Width: -1, Height: -1}, err
	}
	return parseScreenResolution(stdout)
}

// Parses input, example:
// Physical size: 1440x3040
func parseScreenResolution(in string) (Resolution, error) {
	r := regexp.MustCompile(`Physical size: ([0-9]+)x([0-9]+)`)
	resStr := r.FindStringSubmatch(in)
	if len(resStr) != 3 {
		return Resolution{Width: -1, Height: -1}, ErrResolutionParseFail
	}
	w, _ := strconv.Atoi(resStr[1])
	h, _ := strconv.Atoi(resStr[2])
	return Resolution{Width: w, Height: h}, nil
}

func (d Device) Tap(ctx context.Context, X, Y int) error {
	cmd := []string{
		"-s", string(d.SerialNo), "shell",
		"input", "tap",
		strconv.Itoa(X), strconv.Itoa(Y),
	}
	_, _, _, err := execute(ctx, cmd)
	return err
}

// Simulates a long press
//
// Under the hood, this calls swipe with the same start and end coordinates
// with a duration of 250ms
func (d Device) LongPress(ctx context.Context, X, Y int) error {
	return d.Swipe(ctx, X, Y, X, Y, time.Millisecond*250)
}

func (d Device) Swipe(ctx context.Context, X1, Y1, X2, Y2 int, duration time.Duration) error {
	cmd := []string{
		"-s", string(d.SerialNo), "shell",
		"input", "swipe",
		strconv.Itoa(X1), strconv.Itoa(Y1),
		strconv.Itoa(X2), strconv.Itoa(Y2),
		strconv.Itoa(int(duration.Milliseconds())),
	}
	_, _, _, err := execute(ctx, cmd)
	return err
}

// Equivalent to pressing the home button
//
// Calls `input keyevent KEYCODE_HOME` under the hood
func (d Device) GoHome(ctx context.Context) error {
	cmd := []string{"-s", string(d.SerialNo), "shell", "input", "keyevent", "KEYCODE_HOME"}
	_, _, _, err := execute(ctx, cmd)
	return err
}

// Equivalent to pressing the back button
//
// Calls `input keyevent KEYCODE_BACK` under the hood
func (d Device) GoBack(ctx context.Context) error {
	cmd := []string{"-s", string(d.SerialNo), "shell", "input", "keyevent", "KEYCODE_BACK"}
	_, _, _, err := execute(ctx, cmd)
	return err
}

// Equivalent to pushing the app switcher. You probably want to call this twice.
//
// Calls `input keyevent KEYCODE_APP_SWITCH` under the hood
func (d Device) SwitchApp(ctx context.Context) error {
	cmd := []string{"-s", string(d.SerialNo), "shell", "input", "keyevent", "KEYCODE_APP_SWITCH"}
	_, _, _, err := execute(ctx, cmd)
	return err
}
