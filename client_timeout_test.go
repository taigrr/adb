package adb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultTimeoutPolicy(t *testing.T) {
	for _, timeout := range []time.Duration{0, -time.Second, time.Minute} {
		t.Run(timeout.String(), func(t *testing.T) {
			client := &Client{defaultTimeout: timeout}
			parent, cancelParent := context.WithCancel(context.Background())
			defer cancelParent()
			ctx, cancel := client.withTimeout(parent)
			defer cancel()
			_, hasDeadline := ctx.Deadline()
			if hasDeadline != (timeout > 0) {
				t.Fatalf("deadline present = %v for timeout %v", hasDeadline, timeout)
			}
			cancelParent()
			if !errors.Is(ctx.Err(), context.Canceled) {
				t.Fatalf("parent cancellation not propagated: %v", ctx.Err())
			}
		})
	}
	for _, callerTimeout := range []time.Duration{time.Minute, time.Hour} {
		t.Run("caller-"+callerTimeout.String(), func(t *testing.T) {
			parent, cancelParent := context.WithTimeout(context.Background(), callerTimeout)
			defer cancelParent()
			client := &Client{defaultTimeout: 30 * time.Minute}
			ctx, cancel := client.withTimeout(parent)
			defer cancel()
			want, _ := parent.Deadline()
			got, ok := ctx.Deadline()
			if !ok || !got.Equal(want) {
				t.Fatalf("caller deadline changed: got %v, want %v", got, want)
			}
			cancel()
			if parent.Err() != nil {
				t.Fatalf("cleanup cancelled caller context: %v", parent.Err())
			}
		})
	}
}

func TestCommandsApplyDefaultTimeout(t *testing.T) {
	for _, shell := range []bool{false, true} {
		name := "server"
		if shell {
			name = "shell"
		}
		t.Run(name, func(t *testing.T) {
			binary := filepath.Join(t.TempDir(), "adb")
			// Replace the shell so no descendants keep the output pipes alive.
			if err := os.WriteFile(binary, []byte("#!/bin/sh\nexec sleep 30\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			client, err := New(WithBinary(binary), WithDefaultTimeout(50*time.Millisecond))
			if err != nil {
				t.Fatal(err)
			}
			// Bound the test independently without giving the command a caller
			// deadline, which would deliberately override the default timeout.
			ctx, cancel := context.WithCancel(context.Background())
			watchdog := time.AfterFunc(5*time.Second, cancel)
			defer watchdog.Stop()
			defer cancel()
			if shell {
				_, err = client.Device("test-device").Shell(ctx, "true")
			} else {
				err = client.StartServer(ctx)
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected default deadline exceeded, got %v", err)
			}
			var commandErr *CommandError
			if !errors.As(err, &commandErr) {
				t.Fatalf("expected CommandError, got %T", err)
			}
			if ctx.Err() != nil {
				t.Fatalf("command cancelled caller context: %v", ctx.Err())
			}
		})
	}
}
