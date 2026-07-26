package adb

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Shell runs an arbitrary command on the device via `adb shell`. Arguments are
// passed as a pre-split argv (no host-side shell parsing), so quoting is the
// caller's responsibility only for the device-side shell.
func (d Device) Shell(ctx context.Context, name string, args ...string) (Result, error) {
	argv := append([]string{"-s", d.serial, "shell", name}, args...)
	return d.client.run(ctx, argv...)
}

// exec runs an adb subcommand against this device, discarding the result.
func (d Device) exec(ctx context.Context, args ...string) error {
	_, err := d.client.run(ctx, append([]string{"-s", d.serial}, args...)...)
	return err
}

// execChecked runs an adb subcommand and additionally fails with
// [ErrCommandFailed] when adb reports a failure in its output while exiting 0
// (see outputFailure). Use it for subcommands — install, pm, am — that are
// known to do this; general commands should use exec.
func (d Device) execChecked(ctx context.Context, args ...string) error {
	full := append([]string{"-s", d.serial}, args...)
	res, err := d.client.run(ctx, full...)
	if err != nil {
		return err
	}
	if cause := outputFailure(res.StdoutString(), res.StderrString()); cause != nil {
		return &CommandError{Args: full, Code: res.Code, Stderr: res.StderrString(), Err: cause}
	}
	return nil
}

// Push copies a local file to the device, equivalent to `adb push`.
func (d Device) Push(ctx context.Context, src, dest string) error {
	if _, err := os.Stat(src); err != nil {
		return err
	}
	return d.exec(ctx, "push", src, dest)
}

// Pull copies a file from the device to dest, equivalent to `adb pull`. It
// returns [ErrDestExists] if dest already exists.
func (d Device) Pull(ctx context.Context, src, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return ErrDestExists
	} else if !os.IsNotExist(err) {
		return err
	}
	return d.exec(ctx, "pull", src, dest)
}

// Reboot reboots the device, equivalent to `adb reboot`. The handle must be
// reconnected afterward.
func (d Device) Reboot(ctx context.Context) error {
	return d.exec(ctx, "reboot")
}

// Root restarts adbd with root permissions, equivalent to `adb root`. It
// returns true when adbd is running as root afterward.
func (d Device) Root(ctx context.Context) (bool, error) {
	res, err := d.client.run(ctx, "-s", d.serial, "root")
	if err != nil {
		return false, err
	}
	out := res.StdoutString() + res.StderrString()
	return strings.Contains(out, "already running as root") ||
		strings.Contains(out, "restarting adbd as root"), nil
}

// Unroot restarts adbd without root permissions, equivalent to `adb unroot`.
// It returns true when adbd is running as a non-root user afterward.
func (d Device) Unroot(ctx context.Context) (bool, error) {
	res, err := d.client.run(ctx, "-s", d.serial, "unroot")
	if err != nil {
		return false, err
	}
	out := res.StdoutString() + res.StderrString()
	return strings.Contains(out, "not running as root") ||
		strings.Contains(out, "restarting adbd as non root"), nil
}

// Remount remounts the device partitions read-write, equivalent to
// `adb remount`. The device must be rooted.
func (d Device) Remount(ctx context.Context) error {
	return d.exec(ctx, "remount")
}

// TCPIP restarts adbd on the device listening on the given TCP port,
// equivalent to `adb tcpip <port>`.
func (d Device) TCPIP(ctx context.Context, port uint16) error {
	return d.exec(ctx, "tcpip", strconv.Itoa(int(port)))
}

// WaitForDevice blocks until the device is available or ctx is cancelled,
// equivalent to `adb wait-for-device`.
func (d Device) WaitForDevice(ctx context.Context) error {
	return d.exec(ctx, "wait-for-device")
}

// State returns the device's connection state (for example "device",
// "offline", or "bootloader"), equivalent to `adb get-state`.
func (d Device) State(ctx context.Context) (string, error) {
	res, err := d.client.run(ctx, "-s", d.serial, "get-state")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.StdoutString()), nil
}

// Forward forwards a host socket to a device socket, equivalent to
// `adb forward <local> <remote>` (for example "tcp:8000", "tcp:9000").
func (d Device) Forward(ctx context.Context, local, remote string) error {
	return d.exec(ctx, "forward", local, remote)
}

// Reverse reverses a device socket to a host socket, equivalent to
// `adb reverse <remote> <local>`.
func (d Device) Reverse(ctx context.Context, remote, local string) error {
	return d.exec(ctx, "reverse", remote, local)
}

// Install pushes and installs an APK, equivalent to `adb install`. When
// replace is true the app is reinstalled keeping its data (`-r`).
func (d Device) Install(ctx context.Context, apkPath string, replace bool) error {
	if _, err := os.Stat(apkPath); err != nil {
		return err
	}
	args := []string{"install"}
	if replace {
		args = append(args, "-r")
	}
	args = append(args, apkPath)
	return d.execChecked(ctx, args...)
}

// Uninstall removes an installed package, equivalent to `adb uninstall`.
func (d Device) Uninstall(ctx context.Context, pkg string) error {
	return d.execChecked(ctx, "uninstall", pkg)
}

// Screencap captures a PNG screenshot and writes it to dest, equivalent to
// `adb exec-out screencap -p`. It returns [ErrDestExists] if dest exists.
func (d Device) Screencap(ctx context.Context, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return ErrDestExists
	} else if !os.IsNotExist(err) {
		return err
	}
	res, err := d.client.run(ctx, "-s", d.serial, "exec-out", "screencap", "-p")
	if err != nil {
		return err
	}
	if len(res.Stdout) == 0 {
		return ErrStdoutEmpty
	}
	return os.WriteFile(dest, res.Stdout, 0o600)
}

// Tap taps the screen at (x, y), equivalent to `adb shell input tap`.
func (d Device) Tap(ctx context.Context, x, y int) error {
	return d.exec(ctx, "shell", "input", "tap", strconv.Itoa(x), strconv.Itoa(y))
}

// Swipe swipes from (x1, y1) to (x2, y2) over duration, equivalent to
// `adb shell input swipe`.
func (d Device) Swipe(ctx context.Context, x1, y1, x2, y2 int, duration time.Duration) error {
	return d.exec(ctx, "shell", "input", "swipe",
		strconv.Itoa(x1), strconv.Itoa(y1),
		strconv.Itoa(x2), strconv.Itoa(y2),
		strconv.FormatInt(duration.Milliseconds(), 10),
	)
}

// LongPress simulates a long press at (x, y) using a 250ms swipe in place.
func (d Device) LongPress(ctx context.Context, x, y int) error {
	return d.Swipe(ctx, x, y, x, y, 250*time.Millisecond)
}

// KeyEvent sends a key event, equivalent to `adb shell input keyevent`. The
// keycode may be numeric ("3") or a KEYCODE_ constant ("KEYCODE_HOME").
func (d Device) KeyEvent(ctx context.Context, keycode string) error {
	return d.exec(ctx, "shell", "input", "keyevent", keycode)
}

// GoHome presses the home button.
func (d Device) GoHome(ctx context.Context) error { return d.KeyEvent(ctx, "KEYCODE_HOME") }

// GoBack presses the back button.
func (d Device) GoBack(ctx context.Context) error { return d.KeyEvent(ctx, "KEYCODE_BACK") }

// SwitchApp opens the app switcher. You probably want to call this twice.
func (d Device) SwitchApp(ctx context.Context) error { return d.KeyEvent(ctx, "KEYCODE_APP_SWITCH") }

// InputText types text on the device, equivalent to `adb shell input text`.
//
// Spaces are encoded as %s so multi-word strings are typed intact. A literal
// "%" is not escaped and may be misinterpreted; other shell-special characters
// are not escaped either.
func (d Device) InputText(ctx context.Context, text string) error {
	return d.exec(ctx, "shell", "input", "text", strings.ReplaceAll(text, " ", "%s"))
}

// GetProp returns a device system property, equivalent to
// `adb shell getprop <prop>`.
func (d Device) GetProp(ctx context.Context, prop string) (string, error) {
	res, err := d.client.run(ctx, "-s", d.serial, "shell", "getprop", prop)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.StdoutString()), nil
}

// SetProp sets a device system property, equivalent to
// `adb shell setprop <prop> <value>`.
func (d Device) SetProp(ctx context.Context, prop, value string) error {
	return d.exec(ctx, "shell", "setprop", prop, value)
}

// ListPackages returns the installed package names, equivalent to
// `adb shell pm list packages`.
func (d Device) ListPackages(ctx context.Context) ([]string, error) {
	res, err := d.client.run(ctx, "-s", d.serial, "shell", "pm", "list", "packages")
	if err != nil {
		return nil, err
	}
	return parsePackages(res.StdoutString()), nil
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

// GrantPermission grants a runtime permission, equivalent to
// `adb shell pm grant <pkg> <permission>`.
func (d Device) GrantPermission(ctx context.Context, pkg, permission string) error {
	return d.execChecked(ctx, "shell", "pm", "grant", pkg, permission)
}

// RevokePermission revokes a runtime permission, equivalent to
// `adb shell pm revoke <pkg> <permission>`.
func (d Device) RevokePermission(ctx context.Context, pkg, permission string) error {
	return d.execChecked(ctx, "shell", "pm", "revoke", pkg, permission)
}

// StartActivity launches an activity by component name (for example
// "com.example/.MainActivity"), equivalent to `adb shell am start -n`.
func (d Device) StartActivity(ctx context.Context, component string) error {
	return d.execChecked(ctx, "shell", "am", "start", "-n", component)
}

// ScreenResolution returns the device's physical screen resolution, equivalent
// to `adb shell wm size`.
func (d Device) ScreenResolution(ctx context.Context) (Resolution, error) {
	res, err := d.client.run(ctx, "-s", d.serial, "shell", "wm", "size")
	if err != nil {
		return Resolution{}, err
	}
	return parseScreenResolution(res.StdoutString())
}

var reResolution = regexp.MustCompile(`Physical size: ([0-9]+)x([0-9]+)`)

func parseScreenResolution(in string) (Resolution, error) {
	m := reResolution.FindStringSubmatch(in)
	if len(m) != 3 {
		return Resolution{}, ErrResolutionParseFail
	}
	w, _ := strconv.Atoi(m[1])
	h, _ := strconv.Atoi(m[2])
	return Resolution{Width: w, Height: h}, nil
}
