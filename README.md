[![PkgGoDev](https://pkg.go.dev/badge/github.com/taigrr/adb)](https://pkg.go.dev/github.com/taigrr/adb)
# adb

Idiomatic `adb` bindings for Go, to make it easier to write system tooling that
drives Android devices. Instead of arbitrarily shelling out to `adb`, this
library provides a structured, thoroughly-tested, context-aware wrapper around
the `adb` commands most likely to be used in a system program.

`adb` must be installed and available in your `PATH`. At this time, while this
library may work on Windows or macOS, only Linux is supported. If you would like
to add support for Windows, macOS, *BSD, etc., please
[Submit a Pull Request](https://github.com/taigrr/adb/pulls).

```sh
go get github.com/taigrr/adb
```

## What is adb

`adb`, the Android Debug Bridge, is a command-line program which allows a user
to remote-control and debug Android devices.

## Design

- A `Client` wraps the adb server; create it once with `New(...)`. Devices are
  obtained from the client and carry a reference back to it.
- `Device` is an opaque, immutable handle (accessors `Serial`, `Transport`,
  `Addr`, `Authorized`) using `net/netip`.
- `Shell` takes an argv slice and returns a single `Result{Stdout, Stderr
  []byte, Code int}`. A device command's non-zero exit is reported via
  `Result.Code`, not as an error.
- Errors are sentinels wrapped in `*CommandError` (which exposes the argv, exit
  code, and stderr). Match causes with `errors.Is`; timeouts and cancellations
  surface as `context.DeadlineExceeded` / `context.Canceled`.
- adb failures reported on stdout while exiting 0 (install/uninstall/am/pm)
  surface as `ErrCommandFailed`.
- Record/replay uses an opaque `Sequence` with `MarshalJSON`/`ParseSequence`
  and a programmatic builder (`NewSequence`/`NewTap`/`NewSwipe`/`NewSleep`).

## Supported adb functions

- [x] `adb connect` / `adb disconnect`
- [x] `adb pair` (Android 11+ wireless pairing)
- [x] `adb tcpip`
- [x] `adb devices` / `adb get-state` / `adb wait-for-device`
- [x] `adb shell <command>`
- [x] `adb start-server` / `adb kill-server`
- [x] `adb push` / `adb pull`
- [x] `adb install` / `adb uninstall`
- [x] `adb forward` / `adb reverse` (and their remove variants)
- [x] `adb reboot` / `adb root` / `adb unroot` / `adb remount`
- [x] `adb exec-out screencap` (save a screenshot or get PNG bytes)
- [x] `adb shell input tap` / `swipe` / `text` / `keyevent`
- [x] `adb shell getprop` / `setprop`
- [x] `adb shell pm list packages` / `pm grant` / `pm revoke`
- [x] `adb shell am start`
- [x] `adb shell wm size` (screen resolution)
- [x] `adb shell getevent` (capture and replay tap sequences)

Not all of ADB's functionality is currently supported; if you need something
that isn't, please [open an issue](https://github.com/taigrr/adb/issues) or
[submit a PR](https://github.com/taigrr/adb/pulls).

## Example

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/taigrr/adb"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := adb.New()
	if err != nil {
		log.Fatal(err)
	}

	dev, err := client.Connect(ctx, "192.168.2.5")
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer dev.Disconnect(ctx)

	res, err := dev.Shell(ctx, "ls", "/sdcard")
	if err != nil {
		log.Fatalf("shell: %v", err)
	}
	log.Printf("stdout: %s", res.StdoutString())
}
```

## Record/replay caveat

`Record`/`Replay` go through `adb shell input`, which can only inject a single
pointer. A recorded multi-finger gesture (pinch/zoom) is therefore decomposed
into one swipe per finger and replayed **sequentially**, not simultaneously.

True simultaneous multitouch *is* possible over adb by replaying the raw evdev
stream (`sendevent`, or writing `struct input_event` records directly to
`/dev/input/eventX`) with `ABS_MT_SLOT`/`ABS_MT_TRACKING_ID`. That path is not
implemented here because it requires root (or the `shell` user to be in the
`input` group) and device-specific `/dev/input` handling — the `input`-based
path works on any authorized device without elevated permissions.

## Context support

All calls take a `context.Context`, so blocking calls can time out according to
the caller's needs. Cancellation and deadline expiry surface as
`context.Canceled` / `context.DeadlineExceeded` (matchable via `errors.Is`).

## License

This project is licensed under the 0BSD License, written by
[Rob Landley](https://github.com/landley). You may use this library without
restriction or attribution, but please don't pass it off as your own.
Attribution, though not required, is appreciated.

By contributing, you agree all code submitted also falls under the License.

## External resources

- [Official adb documentation](https://developer.android.com/studio/command-line/adb)
- [Inspiration for this repo](https://github.com/taigrr/systemctl)
