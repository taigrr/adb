package adb

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// fakeADB writes a stub adb script that records its args to a file and emits
// the configured stdout/stderr/exit code, then returns a Client pointed at it
// along with the path of the recorded-args file.
func fakeADB(t *testing.T, stdout, stderr string, exitCode int) (*Client, string) {
	t.Helper()

	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	scriptPath := filepath.Join(dir, "adb")
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
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil { //nolint:gosec // test fixture
		t.Fatalf("write fake adb: %v", err)
	}

	t.Setenv("ADB_TEST_ARGS_FILE", argsFile)
	t.Setenv("ADB_TEST_STDOUT", stdout)
	t.Setenv("ADB_TEST_STDERR", stderr)
	t.Setenv("ADB_TEST_EXITCODE", strconv.Itoa(exitCode))

	client, err := New(WithBinary(scriptPath))
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	return client, argsFile
}

func readArgs(t *testing.T, path string) []string {
	t.Helper()
	contents, err := os.ReadFile(path) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	return splitLines(string(contents))
}

func splitLines(in string) []string {
	lines := []string{}
	current := ""
	for _, r := range in {
		if r == '\n' {
			if current != "" {
				lines = append(lines, current)
			}
			current = ""
			continue
		}
		current += string(r)
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// fakeDevice returns a Device bound to the given fake client.
func fakeDevice(c *Client, serial string, transport Transport) Device {
	return Device{client: c, serial: serial, transport: transport}
}
