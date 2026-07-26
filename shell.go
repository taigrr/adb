package adb

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/shlex"
)

// Shell allows you to run an arbitrary shell command against a device.
//
// This function is useful if you need to run an obscure shell command or if
// you require functionality not provided by the exposed functions here.
// Instead of using Shell, please consider submitting a PR with the functionality
// you require.
func (d Device) Shell(ctx context.Context, command string) (stdout string, stderr string, errCode int, err error) {
	cmd, err := shlex.Split(command)
	if err != nil {
		return "", "", 1, err
	}
	prefix := []string{"-s", string(d.SerialNo), "shell"}
	cmd = append(prefix, cmd...)
	return execute(ctx, cmd)
}

// GetScreenResolution returns the device's physical screen resolution,
// equivalent to `adb shell wm size`.
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

// Tap simulates a tap at the given screen coordinates.
func (d Device) Tap(ctx context.Context, x, y int) error {
	cmd := []string{
		"-s", string(d.SerialNo), "shell",
		"input", "tap",
		strconv.Itoa(x), strconv.Itoa(y),
	}
	_, _, _, err := execute(ctx, cmd)
	return err
}

// LongPress simulates a long press at the given coordinates.
//
// Under the hood, this calls swipe with the same start and end coordinates
// with a duration of 250ms
func (d Device) LongPress(ctx context.Context, x, y int) error {
	return d.Swipe(ctx, x, y, x, y, time.Millisecond*250)
}

// Swipe simulates a swipe gesture from (x1, y1) to (x2, y2) over the given
// duration.
func (d Device) Swipe(ctx context.Context, x1, y1, x2, y2 int, duration time.Duration) error {
	cmd := []string{
		"-s", string(d.SerialNo), "shell",
		"input", "swipe",
		strconv.Itoa(x1), strconv.Itoa(y1),
		strconv.Itoa(x2), strconv.Itoa(y2),
		strconv.Itoa(int(duration.Milliseconds())),
	}
	_, _, _, err := execute(ctx, cmd)
	return err
}

// GoHome simulates pressing the home button.
//
// Calls `input keyevent KEYCODE_HOME` under the hood
func (d Device) GoHome(ctx context.Context) error {
	cmd := []string{"-s", string(d.SerialNo), "shell", "input", "keyevent", "KEYCODE_HOME"}
	_, _, _, err := execute(ctx, cmd)
	return err
}

// GoBack simulates pressing the back button.
//
// Calls `input keyevent KEYCODE_BACK` under the hood
func (d Device) GoBack(ctx context.Context) error {
	cmd := []string{"-s", string(d.SerialNo), "shell", "input", "keyevent", "KEYCODE_BACK"}
	_, _, _, err := execute(ctx, cmd)
	return err
}

// SwitchApp opens the app switcher. You probably want to call this twice.
//
// Calls `input keyevent KEYCODE_APP_SWITCH` under the hood
func (d Device) SwitchApp(ctx context.Context) error {
	cmd := []string{"-s", string(d.SerialNo), "shell", "input", "keyevent", "KEYCODE_APP_SWITCH"}
	_, _, _, err := execute(ctx, cmd)
	return err
}

// KeyEvent sends an arbitrary key event, equivalent to
// `adb shell input keyevent <keycode>`. The keycode may be numeric ("3") or a
// KEYCODE_ constant ("KEYCODE_HOME").
func (d Device) KeyEvent(ctx context.Context, keycode string) error {
	cmd := []string{"-s", string(d.SerialNo), "shell", "input", "keyevent", keycode}
	_, _, _, err := execute(ctx, cmd)
	return err
}

// InputText types the given text on the device, equivalent to
// `adb shell input text <text>`.
//
// Spaces are encoded as %s so that multi-word strings are typed intact, as
// required by Android's `input text`. A literal "%" in text is not escaped and
// may be misinterpreted by `input text`; other shell-special characters are
// likewise not escaped, so send those via [Device.Shell] with your own quoting
// if needed.
func (d Device) InputText(ctx context.Context, text string) error {
	cmd := []string{"-s", string(d.SerialNo), "shell", "input", "text", strings.ReplaceAll(text, " ", "%s")}
	_, _, _, err := execute(ctx, cmd)
	return err
}

// GetProp returns the value of a device system property, equivalent to
// `adb shell getprop <prop>`.
func (d Device) GetProp(ctx context.Context, prop string) (string, error) {
	cmd := []string{"-s", string(d.SerialNo), "shell", "getprop", prop}
	stdout, _, _, err := execute(ctx, cmd)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

// SetProp sets a device system property, equivalent to
// `adb shell setprop <prop> <value>`.
func (d Device) SetProp(ctx context.Context, prop, value string) error {
	cmd := []string{"-s", string(d.SerialNo), "shell", "setprop", prop, value}
	_, _, _, err := execute(ctx, cmd)
	return err
}

// ListPackages returns the package names of applications installed on the
// device, equivalent to `adb shell pm list packages`.
func (d Device) ListPackages(ctx context.Context) ([]string, error) {
	cmd := []string{"-s", string(d.SerialNo), "shell", "pm", "list", "packages"}
	stdout, _, _, err := execute(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return parsePackages(stdout), nil
}

func parsePackages(stdout string) []string {
	packages := []string{}
	for line := range strings.SplitSeq(stdout, "\n") {
		if pkg, ok := strings.CutPrefix(strings.TrimSpace(line), "package:"); ok && pkg != "" {
			packages = append(packages, pkg)
		}
	}
	return packages
}

// GrantPermission grants a runtime permission to a package, equivalent to
// `adb shell pm grant <pkg> <permission>`.
func (d Device) GrantPermission(ctx context.Context, pkg, permission string) error {
	cmd := []string{"-s", string(d.SerialNo), "shell", "pm", "grant", pkg, permission}
	stdout, stderr, _, err := execute(ctx, cmd)
	if err != nil {
		return err
	}
	return outputFailure(stdout, stderr)
}

// RevokePermission revokes a runtime permission from a package, equivalent to
// `adb shell pm revoke <pkg> <permission>`.
func (d Device) RevokePermission(ctx context.Context, pkg, permission string) error {
	cmd := []string{"-s", string(d.SerialNo), "shell", "pm", "revoke", pkg, permission}
	stdout, stderr, _, err := execute(ctx, cmd)
	if err != nil {
		return err
	}
	return outputFailure(stdout, stderr)
}

// StartActivity launches an activity by fully-qualified component name
// (for example "com.example/.MainActivity"), equivalent to
// `adb shell am start -n <component>`.
func (d Device) StartActivity(ctx context.Context, component string) error {
	cmd := []string{"-s", string(d.SerialNo), "shell", "am", "start", "-n", component}
	stdout, stderr, _, err := execute(ctx, cmd)
	if err != nil {
		return err
	}
	return outputFailure(stdout, stderr)
}
