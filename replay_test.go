package adb

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestReplayPreservesOrder(t *testing.T) {
	client, argsFile := fakeADB(t, "", "", 0)
	// Append every invocation so the assertion catches dropped or reordered actions.
	script := "#!/bin/sh\nprintf '%s\\0' \"$@\" >> \"$ADB_TEST_ARGS_FILE\"\n"
	if err := os.WriteFile(client.binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	sequence := NewSequence(Resolution{}, NewTap(10, 20), NewSleep(0), NewSwipe(30, 40, 50, 60, 200*time.Millisecond))
	if err := fakeDevice(client, "S", USB).Replay(context.Background(), sequence); err != nil {
		t.Fatal(err)
	}
	want := []string{"-s", "S", "shell", "input", "tap", "10", "20", "-s", "S", "shell", "input", "swipe", "30", "40", "50", "60", "200"}
	if got := readArgs(t, argsFile); !reflect.DeepEqual(got, want) {
		t.Fatalf("Replay args = %v, want %v", got, want)
	}
}

func TestReplayStopsOnCommandFailure(t *testing.T) {
	client, argsFile := fakeADB(t, "", "error: device offline", 1)
	sequence := NewSequence(Resolution{}, NewTap(10, 20), NewTap(30, 40))
	err := fakeDevice(client, "S", USB).Replay(context.Background(), sequence)
	if !errors.Is(err, ErrDeviceOffline) {
		t.Fatalf("Replay error = %v, want ErrDeviceOffline", err)
	}
	var commandError *CommandError
	if !errors.As(err, &commandError) || commandError.Code != 1 {
		t.Fatalf("Replay error = %#v, want command exit code 1", err)
	}
	want := []string{"-s", "S", "shell", "input", "tap", "10", "20"}
	if got := readArgs(t, argsFile); !reflect.DeepEqual(got, want) {
		t.Fatalf("Replay continued after failure: args = %v, want %v", got, want)
	}
}

func TestReplayCancelsPause(t *testing.T) {
	client, argsFile := fakeADB(t, "", "", 0)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	// A long pause must be interrupted before any following device command runs.
	sequence := NewSequence(Resolution{}, NewSleep(time.Hour), NewTap(10, 20))
	done := make(chan error, 1)
	go func() { done <- fakeDevice(client, "S", USB).Replay(ctx, sequence) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Replay error = %v, want deadline exceeded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Replay did not interrupt the pause")
	}
	if _, err := os.Stat(argsFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Replay invoked adb after cancelled pause: stat error = %v", err)
	}
}

func TestReplayEmptySequence(t *testing.T) {
	// No client is needed when there are no actions to replay.
	if err := (Device{}).Replay(context.Background(), Sequence{}); err != nil {
		t.Fatalf("Replay empty sequence: %v", err)
	}
}
