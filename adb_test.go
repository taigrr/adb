package adb

import (
	"reflect"
	"testing"
)

func Test_parseDevices(t *testing.T) {
	type args struct {
		stdout string
	}
	tests := []struct {
		name    string
		args    args
		want    []Device
		wantErr bool
	}{
		{
			name: "2 auth 1 unauth", args: args{stdout: `List of devices attached
19291FDEE0023W  device
9B061FFBA00BC9  device
HT75R0202681    unauthorized`},
			wantErr: false,
			want: []Device{
				{IsAuthorized: true, SerialNo: "19291FDEE0023W"},
				{IsAuthorized: true, SerialNo: "9B061FFBA00BC9"},
				{IsAuthorized: false, SerialNo: "HT75R0202681"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDevices(tt.args.stdout)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDevices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseDevices() = %v, want %v", got, tt.want)
			}
		})
	}
}
