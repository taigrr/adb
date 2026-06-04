package adb

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func withFakeADB(t *testing.T, stdout, stderr string, exitCode string) string {
	t.Helper()

	tempDir := t.TempDir()
	argsFile := filepath.Join(tempDir, "args.txt")
	scriptPath := filepath.Join(tempDir, "adb")
	script := `#!/bin/sh
printf '%s\n' "$@" > "$ADB_TEST_ARGS_FILE"
if [ -n "$ADB_TEST_STDOUT" ]; then
	printf '%b' "$ADB_TEST_STDOUT"
fi
if [ -n "$ADB_TEST_STDERR" ]; then
	printf '%b' "$ADB_TEST_STDERR" >&2
fi
exit "${ADB_TEST_EXITCODE:-0}"
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake adb: %v", err)
	}

	t.Setenv("ADB_TEST_ARGS_FILE", argsFile)
	t.Setenv("ADB_TEST_STDOUT", stdout)
	t.Setenv("ADB_TEST_STDERR", stderr)
	t.Setenv("ADB_TEST_EXITCODE", exitCode)

	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	previousADB := adb
	previousOnce := adbOnce
	adb = ""
	adbOnce = sync.Once{}
	t.Cleanup(func() {
		adb = previousADB
		adbOnce = previousOnce
	})

	return argsFile
}

func readArgs(t *testing.T, path string) []string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	if len(contents) == 0 {
		return nil
	}
	return splitTrimmedLines(string(contents))
}

func splitTrimmedLines(in string) []string {
	lines := []string{}
	current := ""
	for _, char := range in {
		if char == '\n' {
			if current != "" {
				lines = append(lines, current)
			}
			current = ""
			continue
		}
		current += string(char)
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func TestConnect_DefaultPortAndParsedDevice(t *testing.T) {
	argsFile := withFakeADB(t, "connected to 192.168.1.10:5555\n", "", "0")

	device, err := Connect(context.Background(), ConnOptions{Address: netIP(t, "192.168.1.10")})
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	wantDevice := Device{
		SerialNo:     "192.168.1.10:5555",
		IsAuthorized: true,
		ConnType:     Network,
		IP:           netIP(t, "192.168.1.10"),
		Port:         5555,
	}
	if !reflect.DeepEqual(device, wantDevice) {
		t.Fatalf("Connect() = %#v, want %#v", device, wantDevice)
	}

	wantArgs := []string{"connect", "192.168.1.10:5555"}
	if gotArgs := readArgs(t, argsFile); !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("Connect() args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestDeviceShell_QuotedArguments(t *testing.T) {
	argsFile := withFakeADB(t, "", "", "0")
	device := Device{SerialNo: "SERIAL123"}

	_, _, _, err := device.Shell(context.Background(), `echo "hello world" 'two words'`)
	if err != nil {
		t.Fatalf("Shell() error = %v", err)
	}

	wantArgs := []string{"-s", "SERIAL123", "shell", "echo", "hello world", "two words"}
	if gotArgs := readArgs(t, argsFile); !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("Shell() args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestDeviceRoot_SuccessOutput(t *testing.T) {
	argsFile := withFakeADB(t, "restarting adbd as root\n", "", "0")
	device := Device{SerialNo: "SERIAL123"}

	success, err := device.Root(context.Background())
	if err != nil {
		t.Fatalf("Root() error = %v", err)
	}
	if !success {
		t.Fatal("Root() success = false, want true")
	}

	wantArgs := []string{"-s", "SERIAL123", "root"}
	if gotArgs := readArgs(t, argsFile); !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("Root() args = %v, want %v", gotArgs, wantArgs)
	}
}

func TestDeviceDisconnect_UsesDefaultPort(t *testing.T) {
	argsFile := withFakeADB(t, "", "", "0")
	device := Device{ConnType: Network, IP: netIP(t, "192.168.1.20")}

	if err := device.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}

	wantArgs := []string{"disconnect", "192.168.1.20:5555"}
	if gotArgs := readArgs(t, argsFile); !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("Disconnect() args = %v, want %v", gotArgs, wantArgs)
	}
}

func netIP(t *testing.T, address string) net.IPAddr {
	t.Helper()
	ip := net.ParseIP(address)
	if ip == nil {
		t.Fatalf("parse IP %q", address)
	}
	return net.IPAddr{IP: ip}
}

func Test_filterErr(t *testing.T) {
	tests := []struct {
		name    string
		stderr  string
		wantErr error
	}{
		{name: "empty stderr", stderr: "", wantErr: nil},
		{name: "random output", stderr: "some warning text", wantErr: nil},
		{name: "device not found", stderr: "error: device not found", wantErr: ErrDeviceNotFound},
		{name: "device offline", stderr: "error: device offline", wantErr: ErrDeviceOffline},
		{name: "device unauthorized", stderr: "error: device unauthorized.\nThis adb server's $ADB_VENDOR_KEYS is not set", wantErr: ErrDeviceUnauthorized},
		{name: "connection refused", stderr: "cannot connect to daemon at tcp:5037: Connection refused", wantErr: ErrConnectionRefused},
		{name: "more than one device", stderr: "error: more than one device/emulator", wantErr: ErrMoreThanOneDevice},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := filterErr(tt.stderr)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("filterErr() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
