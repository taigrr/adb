package adb

import (
	"errors"
	"testing"
)

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
