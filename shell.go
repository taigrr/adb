package adb

import (
	"context"
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
func (d Device) GetScreenResolution(ctx context.Context) (width int, lenght int, err error) {
	return 0, 0, nil
}

func (d Device) Tap(ctx context.Context, X, Y int) error {
	return nil
}

// Simulates a long press
//
// Under the hood, this calls swipe with the same start and end coordinates
// with a duration of 250ms
func (d Device) LongPress(ctx context.Context, X, Y int) error {
	return nil
}

func (d Device) Swipe(ctx context.Context, X1, Y1, X2, Y2 int, duration time.Duration) error {
	return nil
}

// Equivalent to pressing the home button
//
// Calls `input keyevent KEYCODE_HOME` under the hood
func (d Device) GoHome(ctx context.Context) error {
	return nil
}

//Equivalent to pressing the back button
//
// Calls `input keyevent KEYCODE_BACK` under the hood
func (d Device) GoBack(ctx context.Context) error {
	return nil
}

// Equivalent to pushing the app switcher. You probably want to call this twice.
//
// Calls `input keyevent KEYCODE_APP_SWITCH` under the hood
func (d Device) SwitchApp(ctx context.Context) error {
	return nil
}
