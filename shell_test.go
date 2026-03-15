package adb

import (
	"reflect"
	"testing"
)

func Test_parseScreenResolution(t *testing.T) {
	type args struct {
		in string
	}
	tests := []struct {
		name    string
		args    args
		wantRes Resolution
		wantErr bool
	}{
		{name: "Pixel 4XL", args: args{in: "Physical size: 1440x3040"}, wantRes: Resolution{Width: 1440, Height: 3040}, wantErr: false},
		{name: "Pixel XL", args: args{in: "Physical size: 1440x2560"}, wantRes: Resolution{Width: 1440, Height: 2560}, wantErr: false},
		{name: "garbage", args: args{in: "asdfhjkla"}, wantRes: Resolution{Width: -1, Height: -1}, wantErr: true},
		{name: "empty string", args: args{in: ""}, wantRes: Resolution{Width: -1, Height: -1}, wantErr: true},
		{name: "Samsung S21", args: args{in: "Physical size: 1080x2400"}, wantRes: Resolution{Width: 1080, Height: 2400}, wantErr: false},
		{name: "override size", args: args{in: "Physical size: 1440x3120\nOverride size: 1080x2340"}, wantRes: Resolution{Width: 1440, Height: 3120}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRes, err := parseScreenResolution(tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseScreenResolution() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("Device.GetScreenResolution() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}
