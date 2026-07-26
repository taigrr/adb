package adb

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseDevices(t *testing.T) {
	c := &Client{}
	tests := []struct {
		name   string
		stdout string
		want   []Device
	}{
		{
			name:   "empty",
			stdout: "List of devices attached\n",
			want:   []Device{},
		},
		{
			name:   "usb authorized",
			stdout: "List of devices attached\n19291FDEE0023W\tdevice\n",
			want:   []Device{{client: c, serial: "19291FDEE0023W", transport: USB, authorized: true}},
		},
		{
			name:   "usb unauthorized",
			stdout: "List of devices attached\nHT75R0202681\tunauthorized\n",
			want:   []Device{{client: c, serial: "HT75R0202681", transport: USB, authorized: false}},
		},
		{
			name:   "network",
			stdout: "List of devices attached\n192.168.1.10:5555\tdevice\n",
			want: []Device{{
				client: c, serial: "192.168.1.10:5555", transport: Network, authorized: true,
				addr: netip.MustParseAddrPort("192.168.1.10:5555"), hasAddr: true,
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.parseDevices(tt.stdout)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseDevices() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNormalizeAddr(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "192.168.1.5", want: "192.168.1.5:5555"},
		{in: "192.168.1.5:5556", want: "192.168.1.5:5556"},
		{in: "mydevice.local", want: "mydevice.local:5555"},
		{in: "phone:5555", want: "phone:5555"},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		got, err := normalizeAddr(tt.in)
		if (err != nil) != tt.wantErr {
			t.Fatalf("normalizeAddr(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.want {
			t.Fatalf("normalizeAddr(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestConnect(t *testing.T) {
	c, argsFile := fakeADB(t, "connected to 192.168.1.10:5555\n", "", 0)
	dev, err := c.Connect(context.Background(), "192.168.1.10")
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if dev.Serial() != "192.168.1.10:5555" || dev.Transport() != Network || !dev.Authorized() {
		t.Fatalf("Connect() device = %#v", dev)
	}
	if got, want := readArgs(t, argsFile), []string{"connect", "192.168.1.10:5555"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Connect() args = %v, want %v", got, want)
	}
}

func TestConnect_FailureOnStdout(t *testing.T) {
	c, _ := fakeADB(t, "unable to connect to 192.168.1.10:5555\n", "", 0)
	_, err := c.Connect(context.Background(), "192.168.1.10")
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("Connect() error = %v, want ErrCommandFailed", err)
	}
}

func TestDisconnect_USBRejected(t *testing.T) {
	c, _ := fakeADB(t, "", "", 0)
	dev := fakeDevice(c, "SERIAL", USB)
	if err := dev.Disconnect(context.Background()); !errors.Is(err, ErrNotNetworkDevice) {
		t.Fatalf("Disconnect() error = %v, want ErrNotNetworkDevice", err)
	}
}

func TestPair(t *testing.T) {
	tests := []struct {
		name string
		code string
		want []string
	}{
		{name: "with code", code: "123456", want: []string{"pair", "192.168.1.5:37013", "123456"}},
		{name: "without code", code: "", want: []string{"pair", "192.168.1.5:37013"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, argsFile := fakeADB(t, "Successfully paired to 192.168.1.5:37013\n", "", 0)
			if err := c.Pair(context.Background(), "192.168.1.5:37013", tt.code); err != nil {
				t.Fatalf("Pair() error = %v", err)
			}
			if got := readArgs(t, argsFile); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Pair() args = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShellArgsAndResult(t *testing.T) {
	c, argsFile := fakeADB(t, "hello\n", "warn\n", 0)
	dev := fakeDevice(c, "SERIAL123", Network)

	res, err := dev.Shell(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("Shell() error = %v", err)
	}
	if res.StdoutString() != "hello\n" || res.Code != 0 {
		t.Fatalf("Shell() result = %#v", res)
	}
	want := []string{"-s", "SERIAL123", "shell", "echo", "hello"}
	if got := readArgs(t, argsFile); !reflect.DeepEqual(got, want) {
		t.Fatalf("Shell() args = %v, want %v", got, want)
	}
}

func TestCommandError_Fields(t *testing.T) {
	c, _ := fakeADB(t, "", "error: device not found\n", 1)
	dev := fakeDevice(c, "SERIAL123", Network)

	_, err := dev.Shell(context.Background(), "ls")
	if !errors.Is(err, ErrDeviceNotFound) {
		t.Fatalf("Shell() error = %v, want ErrDeviceNotFound", err)
	}
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("Shell() error not a *CommandError: %v", err)
	}
	if cmdErr.Code != 1 {
		t.Fatalf("CommandError.Code = %d, want 1", cmdErr.Code)
	}
}

func TestDeviceCommandArgs(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
		call   func(Device) error
		want   []string
	}{
		{name: "reboot", call: func(d Device) error { return d.Reboot(context.Background()) }, want: []string{"-s", "S", "reboot"}},
		{name: "remount", call: func(d Device) error { return d.Remount(context.Background()) }, want: []string{"-s", "S", "remount"}},
		{name: "tcpip", call: func(d Device) error { return d.TCPIP(context.Background(), 5555) }, want: []string{"-s", "S", "tcpip", "5555"}},
		{name: "wait", call: func(d Device) error { return d.WaitForDevice(context.Background()) }, want: []string{"-s", "S", "wait-for-device"}},
		{name: "forward", call: func(d Device) error { return d.Forward(context.Background(), "tcp:8000", "tcp:9000") }, want: []string{"-s", "S", "forward", "tcp:8000", "tcp:9000"}},
		{name: "reverse", call: func(d Device) error { return d.Reverse(context.Background(), "tcp:9000", "tcp:8000") }, want: []string{"-s", "S", "reverse", "tcp:9000", "tcp:8000"}},
		{name: "uninstall", call: func(d Device) error { return d.Uninstall(context.Background(), "com.example") }, want: []string{"-s", "S", "uninstall", "com.example"}},
		{name: "tap", call: func(d Device) error { return d.Tap(context.Background(), 10, 20) }, want: []string{"-s", "S", "shell", "input", "tap", "10", "20"}},
		{name: "keyevent", call: func(d Device) error { return d.GoHome(context.Background()) }, want: []string{"-s", "S", "shell", "input", "keyevent", "KEYCODE_HOME"}},
		{name: "input text", call: func(d Device) error { return d.InputText(context.Background(), "hello world") }, want: []string{"-s", "S", "shell", "input", "text", "hello%sworld"}},
		{name: "setprop", call: func(d Device) error { return d.SetProp(context.Background(), "debug.foo", "1") }, want: []string{"-s", "S", "shell", "setprop", "debug.foo", "1"}},
		{name: "grant", call: func(d Device) error {
			return d.GrantPermission(context.Background(), "com.example", "android.permission.CAMERA")
		}, want: []string{"-s", "S", "shell", "pm", "grant", "com.example", "android.permission.CAMERA"}},
		{name: "start activity", call: func(d Device) error { return d.StartActivity(context.Background(), "com.example/.Main") }, want: []string{"-s", "S", "shell", "am", "start", "-n", "com.example/.Main"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, argsFile := fakeADB(t, tt.stdout, "", 0)
			if err := tt.call(fakeDevice(c, "S", Network)); err != nil {
				t.Fatalf("%s error = %v", tt.name, err)
			}
			if got := readArgs(t, argsFile); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("args = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInstall(t *testing.T) {
	apk := filepath.Join(t.TempDir(), "app.apk")
	if err := os.WriteFile(apk, []byte("dex"), 0o600); err != nil {
		t.Fatalf("write apk: %v", err)
	}
	c, argsFile := fakeADB(t, "Success\n", "", 0)
	if err := fakeDevice(c, "S", Network).Install(context.Background(), apk, true); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	want := []string{"-s", "S", "install", "-r", apk}
	if got := readArgs(t, argsFile); !reflect.DeepEqual(got, want) {
		t.Fatalf("Install() args = %v, want %v", got, want)
	}
}

func TestInstall_FailureOnStdoutExitZero(t *testing.T) {
	apk := filepath.Join(t.TempDir(), "app.apk")
	if err := os.WriteFile(apk, []byte("dex"), 0o600); err != nil {
		t.Fatalf("write apk: %v", err)
	}
	c, _ := fakeADB(t, "Failure [INSTALL_FAILED_INSUFFICIENT_STORAGE]\n", "", 0)
	err := fakeDevice(c, "S", Network).Install(context.Background(), apk, false)
	if !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("Install() error = %v, want ErrCommandFailed", err)
	}
}

func TestInstall_MissingAPK(t *testing.T) {
	c, _ := fakeADB(t, "", "", 0)
	err := fakeDevice(c, "S", Network).Install(context.Background(), filepath.Join(t.TempDir(), "nope.apk"), false)
	if err == nil {
		t.Fatal("Install() expected error for missing APK")
	}
}

func TestStartActivity_SuccessEchoesExceptionComponent(t *testing.T) {
	c, _ := fakeADB(t, "Starting: Intent { cmp=com.example/.ExceptionHandlerActivity }\n", "", 0)
	if err := fakeDevice(c, "S", Network).StartActivity(context.Background(), "com.example/.ExceptionHandlerActivity"); err != nil {
		t.Fatalf("StartActivity() error = %v, want nil", err)
	}
}

func TestStartActivity_WarningNotStarted(t *testing.T) {
	c, _ := fakeADB(t, "Warning: Activity not started, unable to resolve Intent\n", "", 0)
	if err := fakeDevice(c, "S", Network).StartActivity(context.Background(), "com.example/.Bad"); !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("StartActivity() error = %v, want ErrCommandFailed", err)
	}
}

func TestState(t *testing.T) {
	c, _ := fakeADB(t, "device\n", "", 0)
	got, err := fakeDevice(c, "S", Network).State(context.Background())
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if got != "device" {
		t.Fatalf("State() = %q, want %q", got, "device")
	}
}

func TestScreencap(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "shot.png")
	c, argsFile := fakeADB(t, "PNGDATA", "", 0)
	if err := fakeDevice(c, "S", Network).Screencap(context.Background(), dest); err != nil {
		t.Fatalf("Screencap() error = %v", err)
	}
	want := []string{"-s", "S", "exec-out", "screencap", "-p"}
	if got := readArgs(t, argsFile); !reflect.DeepEqual(got, want) {
		t.Fatalf("Screencap() args = %v, want %v", got, want)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read screenshot: %v", err)
	}
	if string(data) != "PNGDATA" {
		t.Fatalf("screenshot = %q", string(data))
	}
}

func TestScreencap_DestExists(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(dest, []byte("x"), 0o600); err != nil {
		t.Fatalf("write dest: %v", err)
	}
	c, _ := fakeADB(t, "PNGDATA", "", 0)
	if err := fakeDevice(c, "S", Network).Screencap(context.Background(), dest); !errors.Is(err, ErrDestExists) {
		t.Fatalf("Screencap() error = %v, want ErrDestExists", err)
	}
}

func TestListPackages(t *testing.T) {
	c, _ := fakeADB(t, "package:com.android.settings\npackage:com.example.app\n", "", 0)
	got, err := fakeDevice(c, "S", Network).ListPackages(context.Background())
	if err != nil {
		t.Fatalf("ListPackages() error = %v", err)
	}
	want := []string{"com.android.settings", "com.example.app"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListPackages() = %v, want %v", got, want)
	}
}

func TestScreenResolution(t *testing.T) {
	c, _ := fakeADB(t, "Physical size: 1440x3120\n", "", 0)
	got, err := fakeDevice(c, "S", Network).ScreenResolution(context.Background())
	if err != nil {
		t.Fatalf("ScreenResolution() error = %v", err)
	}
	if got != (Resolution{Width: 1440, Height: 3120}) {
		t.Fatalf("ScreenResolution() = %v", got)
	}
}

func TestNew_NotInstalled(t *testing.T) {
	if _, err := New(WithBinary("")); err != nil {
		// empty binary falls back to PATH lookup; only assert typed error when adb is absent.
		if !errors.Is(err, ErrNotInstalled) {
			t.Fatalf("New() error = %v", err)
		}
	}
}
