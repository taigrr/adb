package adb

import (
	"context"
	"os"
	"strconv"
	"strings"
)

// Install pushes and installs a single APK on the device, equivalent to
// `adb install <apk>`.
//
// Returns an error if the APK does not exist locally or the installation fails.
func (d Device) Install(ctx context.Context, apkPath string) error {
	return d.install(ctx, apkPath, false)
}

// InstallReplace installs an APK, replacing any existing copy while keeping its
// data, equivalent to `adb install -r <apk>`.
func (d Device) InstallReplace(ctx context.Context, apkPath string) error {
	return d.install(ctx, apkPath, true)
}

func (d Device) install(ctx context.Context, apkPath string, replace bool) error {
	if _, err := os.Stat(apkPath); err != nil {
		return err
	}
	cmd := []string{"-s", string(d.SerialNo), "install"}
	if replace {
		cmd = append(cmd, "-r")
	}
	cmd = append(cmd, apkPath)
	stdout, stderr, errcode, err := execute(ctx, cmd)
	if err != nil {
		return err
	}
	if errcode != 0 {
		return ErrUnspecified
	}
	return outputFailure(stdout, stderr)
}

// Uninstall removes an installed package, equivalent to `adb uninstall <pkg>`.
func (d Device) Uninstall(ctx context.Context, pkg string) error {
	stdout, stderr, errcode, err := execute(ctx, []string{"-s", string(d.SerialNo), "uninstall", pkg})
	if err != nil {
		return err
	}
	if errcode != 0 {
		return ErrUnspecified
	}
	return outputFailure(stdout, stderr)
}

// TCPIP restarts adbd on the device listening on the given TCP port, equivalent
// to `adb tcpip <port>`. This is the first step in switching a USB-attached
// device to a wireless connection.
func (d Device) TCPIP(ctx context.Context, port uint) error {
	_, _, errcode, err := execute(ctx, []string{"-s", string(d.SerialNo), "tcpip", strconv.FormatUint(uint64(port), 10)})
	if err != nil {
		return err
	}
	if errcode != 0 {
		return ErrUnspecified
	}
	return nil
}

// WaitForDevice blocks until the device is available or ctx is cancelled,
// equivalent to `adb wait-for-device`.
func (d Device) WaitForDevice(ctx context.Context) error {
	_, _, errcode, err := execute(ctx, []string{"-s", string(d.SerialNo), "wait-for-device"})
	if err != nil {
		return err
	}
	if errcode != 0 {
		return ErrUnspecified
	}
	return nil
}

// State returns the device's connection state (for example "device",
// "offline", or "bootloader"), equivalent to `adb get-state`.
func (d Device) State(ctx context.Context) (string, error) {
	stdout, _, errcode, err := execute(ctx, []string{"-s", string(d.SerialNo), "get-state"})
	if err != nil {
		return "", err
	}
	if errcode != 0 {
		return "", ErrUnspecified
	}
	return strings.TrimSpace(stdout), nil
}

// Unroot restarts adbd without root permissions, equivalent to `adb unroot`.
//
// Returns true if adbd was (re)started as a non-root user.
func (d Device) Unroot(ctx context.Context) (bool, error) {
	stdout, stderr, errcode, err := execute(ctx, []string{"-s", string(d.SerialNo), "unroot"})
	if err != nil {
		return false, err
	}
	if errcode != 0 {
		return false, ErrUnspecified
	}
	out := stdout + stderr
	if strings.Contains(out, "adbd not running as root") ||
		strings.Contains(out, "restarting adbd as non root") {
		return true, nil
	}
	return false, nil
}

// Remount remounts the device's partitions read-write, equivalent to
// `adb remount`. The device must be rooted.
func (d Device) Remount(ctx context.Context) error {
	_, _, errcode, err := execute(ctx, []string{"-s", string(d.SerialNo), "remount"})
	if err != nil {
		return err
	}
	if errcode != 0 {
		return ErrUnspecified
	}
	return nil
}

// Forward forwards a host socket to a device socket, equivalent to
// `adb forward <local> <remote>` (for example "tcp:8000", "tcp:9000").
func (d Device) Forward(ctx context.Context, local, remote string) error {
	_, _, errcode, err := execute(ctx, []string{"-s", string(d.SerialNo), "forward", local, remote})
	if err != nil {
		return err
	}
	if errcode != 0 {
		return ErrUnspecified
	}
	return nil
}

// Reverse reverses a device socket to a host socket, equivalent to
// `adb reverse <remote> <local>`.
func (d Device) Reverse(ctx context.Context, remote, local string) error {
	_, _, errcode, err := execute(ctx, []string{"-s", string(d.SerialNo), "reverse", remote, local})
	if err != nil {
		return err
	}
	if errcode != 0 {
		return ErrUnspecified
	}
	return nil
}

// Screencap captures a PNG screenshot from the device and writes it to dest,
// equivalent to `adb exec-out screencap -p > dest`.
//
// Returns [ErrDestExists] if dest already exists.
func (d Device) Screencap(ctx context.Context, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return ErrDestExists
	} else if !os.IsNotExist(err) {
		return err
	}
	stdout, _, errcode, err := execute(ctx, []string{"-s", string(d.SerialNo), "exec-out", "screencap", "-p"})
	if err != nil {
		return err
	}
	if errcode != 0 {
		return ErrUnspecified
	}
	if stdout == "" {
		return ErrStdoutEmpty
	}
	return os.WriteFile(dest, []byte(stdout), 0o600)
}

// Pair pairs with a device for secure wireless debugging, equivalent to
// `adb pair HOST[:PORT] [CODE]` (Android 11+). Pass an empty code to pair
// interactively-provisioned devices that do not require one.
func Pair(ctx context.Context, hostPort, code string) error {
	cmd := []string{"pair", hostPort}
	if code != "" {
		cmd = append(cmd, code)
	}
	stdout, _, errcode, err := execute(ctx, cmd)
	if err != nil {
		return err
	}
	if errcode != 0 {
		return ErrUnspecified
	}
	if !strings.Contains(stdout, "Successfully paired") {
		return ErrUnspecified
	}
	return nil
}
