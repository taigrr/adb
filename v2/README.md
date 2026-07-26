# adb/v2

Version 2 of the idiomatic Go bindings for the Android Debug Bridge. v2 is a
clean redesign of v1 and lives in its own module, so v1 consumers are
unaffected.

```sh
go get github.com/taigrr/adb/v2
```

## What changed from v1

- A `Client` wraps the adb server; create it once with `New(...)`. Devices are
  obtained from the client and carry a reference back to it.
- `Device` is an opaque, immutable handle (accessors `Serial`, `Transport`,
  `Addr`, `Authorized`) using `net/netip` instead of `net.IPAddr`.
- `Shell` takes an argv slice and returns a single `Result{Stdout, Stderr
  []byte, Code int}` instead of four values.
- Errors are sentinels wrapped in `*CommandError` (which exposes the argv, exit
  code, and stderr). Match causes with `errors.Is`; timeouts and cancellations
  surface as `context.DeadlineExceeded` / `context.Canceled`.
- adb failures reported on stdout while exiting 0 (install/uninstall/am/pm)
  surface as `ErrCommandFailed`.
- Record/replay uses an opaque `Sequence` with `MarshalJSON`/`ParseSequence`.

## Record/replay caveat

`adb shell input` can only inject single-touch taps and swipes, so a recorded
multi-finger gesture (pinch/zoom) is decomposed into one swipe per finger and
replayed **sequentially**, not simultaneously. True multitouch cannot be
reproduced through adb.

## Releasing

Because this module lives in the `v2/` subdirectory, releases must be tagged
with the `v2/` prefix, e.g. `v2/v2.0.0` (a bare `v2.0.0` tag will not resolve
for `go get github.com/taigrr/adb/v2`). The root v1 module is tagged
independently with plain `vX.Y.Z` tags.

## Example

```go
package main

import (
	"context"
	"log"
	"time"

	adb "github.com/taigrr/adb/v2"
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

## License

0BSD, same as the parent module.
