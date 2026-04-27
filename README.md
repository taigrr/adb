[![PkgGoDev](https://pkg.go.dev/badge/github.com/taigrr/adb)](https://pkg.go.dev/github.com/taigrr/adb)
# adb

This library aims at providing idiomatic `adb` bindings for go developers, in order to make it easier to write system tooling using golang.
This tool tries to take guesswork out of arbitrarily shelling out to `adb` by providing a structured, thoroughly-tested wrapper for the `adb` functions most-likely to be used in a system program.

`adb` must be installed and available in your `PATH`. At this time, while this library may work on Windows or macOS, only Linux is supported.
If you would like to add support for Windows, macOS, *BSD, etc., please [Submit a Pull Request](https://github.com/taigrr/adb/pulls).

## What is adb

`adb`, the Android Debug Bridge, is a command-line program which allows a user to remote-control and debug Android devices.


## Supported adb functions

- [x] `adb connect`
- [x] `adb disconnect`
- [x] `adb shell <command>`
- [x] `adb kill-server`
- [x] `adb devices`
- [x] `adb pull`
- [ ] `adb install`
- [x] `adb push`
- [x] `adb reboot`
- [x] `adb root`
- [x] `adb shell input tap X Y`
- [x] `adb shell input swipe X1 Y1 X2 Y2 duration`
- [x] `adb shell input keyevent` (home, back, app switch)
- [x] `adb shell wm size` (screen resolution)
- [x] `adb shell getevent` (capture and replay tap sequences)

Please note that as this is a purpose-driven project library, not all functionality of ADB is currently supported, but if you need functionality that's not currently supported,
Feel free to [Open an Issue](https://github.com/taigrr/adb/issues) or [Submit a Pull Request](https://github.com/taigrr/adb/pulls)

## Helper functionality

- In addition to using the shell commands, this library provides helper methods for stateful connections.
  That is, you can connect to a device and get back a handler object and call functions against it with better error handling.

- In addition to the connection commands, this library also has helper functions for many common shell commands, including:
  - [ ] pm grant
  - [ ] am start
  - [ ] dumpsys
  - [ ] screencap
  - [ ] screenrecord
  - [ ] rm



## Useful errors

All functions return a predefined error type, and it is highly recommended these errors are handled properly.

## Context support

All calls into this library support go's `context` functionality.
Therefore, blocking calls can time out according to the caller's needs, and the returned error should be checked to see if a timeout occurred (`ErrExecTimeout`).

## Simple example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net"
    "time"

    "github.com/taigrr/adb"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    // Equivalent to `adb connect 192.168.2.5:5555` with a 10 second timeout
    opts := adb.ConnOptions{ Address: net.IPAddr{IP: net.ParseIP("192.168.2.5")} }
    dev, err := adb.Connect(ctx, opts)
    if err != nil {
        log.Fatalf("unable to connect to device %s: %v", opts.Address.String(), err)
    }
    defer func() {
        if err := dev.Disconnect(ctx); err != nil {
            log.Printf("disconnect failed: %v", err)
        }
    }()
    stdout, stderr, errCode, err := dev.Shell(ctx, "ls")
    if err != nil {
        log.Fatalf("unable to shell into device %s: %v", opts.Address.String(), err)
    }
    log.Printf("Stdout: %s\nStderr: %s\n, ErrCode: %d", stdout, stderr, errCode)
}
```

## License

This project is licensed under the 0BSD License, written by [Rob Landley](https://github.com/landley).
As such, you may use this library without restriction or attribution, but please don't pass it off as your own.
Attribution, though not required, is appreciated.

By contributing, you agree all code submitted also falls under the License.

## External resources

- [Official adb documentation](https://developer.android.com/studio/command-line/adb)
- [Inspiration for this repo](https://github.com/taigrr/systemctl)
