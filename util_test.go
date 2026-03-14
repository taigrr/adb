package adb

import (
	"testing"
)

func Test_filterErr(t *testing.T) {
	tests := []struct {
		name    string
		stderr  string
		wantErr bool
	}{
		{name: "empty stderr", stderr: "", wantErr: false},
		{name: "random output", stderr: "some warning text", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := filterErr(tt.stderr)
			if (err != nil) != tt.wantErr {
				t.Errorf("filterErr() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
