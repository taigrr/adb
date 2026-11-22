package adb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPull_DanglingSymlinkDestination(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "missing.txt")
	destination := filepath.Join(directory, "link.txt")
	if err := os.Symlink(target, destination); err != nil {
		t.Fatal(err)
	}
	client, argsFile := fakeADB(t, "", "", 0)
	if err := client.Device("S").Pull(context.Background(), "/sdcard/file.txt", destination); !errors.Is(err, ErrDestExists) {
		t.Fatalf("Pull() error = %v, want ErrDestExists", err)
	}
	if _, err := os.Stat(argsFile); !os.IsNotExist(err) {
		t.Fatalf("adb was invoked or args file inaccessible: %v", err)
	}
	if got, err := os.Readlink(destination); err != nil || got != target {
		t.Fatalf("destination symlink = %q, %v; want %q", got, err, target)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("symlink target was created or inaccessible: %v", err)
	}
}
