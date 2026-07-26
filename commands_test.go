package adb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDeviceInstall_ArgsAndReplace(t *testing.T) {
	apk := filepath.Join(t.TempDir(), "app.apk")
	if err := os.WriteFile(apk, []byte("dex"), 0o600); err != nil {
		t.Fatalf("write apk: %v", err)
	}

	tests := []struct {
		name     string
		call     func(Device) error
		wantArgs []string
	}{
		{
			name:     "install",
			call:     func(d Device) error { return d.Install(context.Background(), apk) },
			wantArgs: []string{"-s", "SERIAL123", "install", apk},
		},
		{
			name:     "install replace",
			call:     func(d Device) error { return d.InstallReplace(context.Background(), apk) },
			wantArgs: []string{"-s", "SERIAL123", "install", "-r", apk},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argsFile := withFakeADB(t, "Success\n", "", "0")
			if err := tt.call(Device{SerialNo: "SERIAL123"}); err != nil {
				t.Fatalf("%s error = %v", tt.name, err)
			}
			if gotArgs := readArgs(t, argsFile); !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("args = %v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}

func TestDeviceInstall_MissingAPK(t *testing.T) {
	withFakeADB(t, "", "", "0")
	err := Device{SerialNo: "SERIAL123"}.Install(context.Background(), filepath.Join(t.TempDir(), "missing.apk"))
	if err == nil {
		t.Fatal("Install() with missing APK expected error, got nil")
	}
}

func TestDeviceInstall_FailureOnStdoutExitZero(t *testing.T) {
	apk := filepath.Join(t.TempDir(), "app.apk")
	if err := os.WriteFile(apk, []byte("dex"), 0o600); err != nil {
		t.Fatalf("write apk: %v", err)
	}
	withFakeADB(t, "Failure [INSTALL_FAILED_INSUFFICIENT_STORAGE]\n", "", "0")
	err := Device{SerialNo: "SERIAL123"}.Install(context.Background(), apk)
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("Install() error = %v, want ErrCommandFailed", err)
	}
}

func TestDeviceStartActivity_ErrorOnStdoutExitZero(t *testing.T) {
	withFakeADB(t, "Error: Activity class {com.example/.Bad} does not exist.\n", "", "0")
	err := Device{SerialNo: "SERIAL123"}.StartActivity(context.Background(), "com.example/.Bad")
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("StartActivity() error = %v, want ErrCommandFailed", err)
	}
}

func TestDeviceStartActivity_WarningNotStarted(t *testing.T) {
	withFakeADB(t, "Warning: Activity not started, unable to resolve Intent { ... }\n", "", "0")
	err := Device{SerialNo: "SERIAL123"}.StartActivity(context.Background(), "com.example/.Bad")
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("StartActivity() error = %v, want ErrCommandFailed", err)
	}
}

func TestDeviceStartActivity_SuccessEchoesIntentWithExceptionInName(t *testing.T) {
	// A successful launch echoes the intent; a component name containing
	// "Exception" must not be misclassified as a failure.
	withFakeADB(t, "Starting: Intent { cmp=com.example/.ExceptionHandlerActivity }\n", "", "0")
	err := Device{SerialNo: "SERIAL123"}.StartActivity(context.Background(), "com.example/.ExceptionHandlerActivity")
	if err != nil {
		t.Fatalf("StartActivity() error = %v, want nil", err)
	}
}

func TestDeviceGrantPermission_ExceptionOnStderrExitZero(t *testing.T) {
	withFakeADB(t, "", "java.lang.SecurityException: Permission ... Exception\n", "0")
	err := Device{SerialNo: "SERIAL123"}.GrantPermission(context.Background(), "com.example", "android.permission.CAMERA")
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("GrantPermission() error = %v, want ErrCommandFailed", err)
	}
}

func TestDeviceCommandArgs(t *testing.T) {
	tests := []struct {
		name     string
		stdout   string
		call     func(Device) error
		wantArgs []string
	}{
		{
			name:     "uninstall",
			call:     func(d Device) error { return d.Uninstall(context.Background(), "com.example") },
			wantArgs: []string{"-s", "SERIAL123", "uninstall", "com.example"},
		},
		{
			name:     "tcpip",
			call:     func(d Device) error { return d.TCPIP(context.Background(), 5555) },
			wantArgs: []string{"-s", "SERIAL123", "tcpip", "5555"},
		},
		{
			name:     "wait-for-device",
			call:     func(d Device) error { return d.WaitForDevice(context.Background()) },
			wantArgs: []string{"-s", "SERIAL123", "wait-for-device"},
		},
		{
			name:     "remount",
			call:     func(d Device) error { return d.Remount(context.Background()) },
			wantArgs: []string{"-s", "SERIAL123", "remount"},
		},
		{
			name:     "forward",
			call:     func(d Device) error { return d.Forward(context.Background(), "tcp:8000", "tcp:9000") },
			wantArgs: []string{"-s", "SERIAL123", "forward", "tcp:8000", "tcp:9000"},
		},
		{
			name:     "reverse",
			call:     func(d Device) error { return d.Reverse(context.Background(), "tcp:9000", "tcp:8000") },
			wantArgs: []string{"-s", "SERIAL123", "reverse", "tcp:9000", "tcp:8000"},
		},
		{
			name:     "input text",
			call:     func(d Device) error { return d.InputText(context.Background(), "hello world") },
			wantArgs: []string{"-s", "SERIAL123", "shell", "input", "text", "hello%sworld"},
		},
		{
			name:     "keyevent",
			call:     func(d Device) error { return d.KeyEvent(context.Background(), "KEYCODE_ENTER") },
			wantArgs: []string{"-s", "SERIAL123", "shell", "input", "keyevent", "KEYCODE_ENTER"},
		},
		{
			name:     "setprop",
			call:     func(d Device) error { return d.SetProp(context.Background(), "debug.foo", "1") },
			wantArgs: []string{"-s", "SERIAL123", "shell", "setprop", "debug.foo", "1"},
		},
		{
			name: "grant permission",
			call: func(d Device) error {
				return d.GrantPermission(context.Background(), "com.example", "android.permission.CAMERA")
			},
			wantArgs: []string{"-s", "SERIAL123", "shell", "pm", "grant", "com.example", "android.permission.CAMERA"},
		},
		{
			name: "revoke permission",
			call: func(d Device) error {
				return d.RevokePermission(context.Background(), "com.example", "android.permission.CAMERA")
			},
			wantArgs: []string{"-s", "SERIAL123", "shell", "pm", "revoke", "com.example", "android.permission.CAMERA"},
		},
		{
			name:     "start activity",
			call:     func(d Device) error { return d.StartActivity(context.Background(), "com.example/.MainActivity") },
			wantArgs: []string{"-s", "SERIAL123", "shell", "am", "start", "-n", "com.example/.MainActivity"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argsFile := withFakeADB(t, tt.stdout, "", "0")
			if err := tt.call(Device{SerialNo: "SERIAL123"}); err != nil {
				t.Fatalf("%s error = %v", tt.name, err)
			}
			if gotArgs := readArgs(t, argsFile); !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("args = %v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}

func TestDeviceState(t *testing.T) {
	withFakeADB(t, "device\n", "", "0")
	got, err := Device{SerialNo: "SERIAL123"}.State(context.Background())
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if got != "device" {
		t.Fatalf("State() = %q, want %q", got, "device")
	}
}

func TestDeviceGetProp(t *testing.T) {
	withFakeADB(t, "arm64-v8a\n", "", "0")
	got, err := Device{SerialNo: "SERIAL123"}.GetProp(context.Background(), "ro.product.cpu.abi")
	if err != nil {
		t.Fatalf("GetProp() error = %v", err)
	}
	if got != "arm64-v8a" {
		t.Fatalf("GetProp() = %q, want %q", got, "arm64-v8a")
	}
}

func TestDeviceUnroot(t *testing.T) {
	withFakeADB(t, "restarting adbd as non root\n", "", "0")
	got, err := Device{SerialNo: "SERIAL123"}.Unroot(context.Background())
	if err != nil {
		t.Fatalf("Unroot() error = %v", err)
	}
	if !got {
		t.Fatal("Unroot() = false, want true")
	}
}

func TestParsePackages(t *testing.T) {
	stdout := "package:com.android.settings\npackage:com.example.app\n\nnot a package\n"
	want := []string{"com.android.settings", "com.example.app"}
	if got := parsePackages(stdout); !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePackages() = %v, want %v", got, want)
	}
}

func TestDeviceScreencap(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "shot.png")
	argsFile := withFakeADB(t, "PNGDATA", "", "0")

	if err := (Device{SerialNo: "SERIAL123"}).Screencap(context.Background(), dest); err != nil {
		t.Fatalf("Screencap() error = %v", err)
	}
	wantArgs := []string{"-s", "SERIAL123", "exec-out", "screencap", "-p"}
	if gotArgs := readArgs(t, argsFile); !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("Screencap() args = %v, want %v", gotArgs, wantArgs)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read screenshot: %v", err)
	}
	if string(data) != "PNGDATA" {
		t.Fatalf("screenshot = %q, want %q", string(data), "PNGDATA")
	}
}

func TestDeviceScreencap_DestExists(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(dest, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write dest: %v", err)
	}
	withFakeADB(t, "PNGDATA", "", "0")
	if err := (Device{SerialNo: "SERIAL123"}).Screencap(context.Background(), dest); !errors.Is(err, ErrDestExists) {
		t.Fatalf("Screencap() error = %v, want ErrDestExists", err)
	}
}

func TestPair(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantArgs []string
	}{
		{name: "with code", code: "123456", wantArgs: []string{"pair", "192.168.1.5:37013", "123456"}},
		{name: "without code", code: "", wantArgs: []string{"pair", "192.168.1.5:37013"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argsFile := withFakeADB(t, "Successfully paired to 192.168.1.5:37013\n", "", "0")
			if err := Pair(context.Background(), "192.168.1.5:37013", tt.code); err != nil {
				t.Fatalf("Pair() error = %v", err)
			}
			if gotArgs := readArgs(t, argsFile); !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("Pair() args = %v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}
