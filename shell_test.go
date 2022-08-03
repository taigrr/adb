package adb

import "testing"

func Test_parseScreenResolution(t *testing.T) {
	type args struct {
		in string
	}
	tests := []struct {
		name       string
		args       args
		wantWidth  int
		wantLength int
		wantErr    bool
	}{
		{name: "Pixel 4XL", args: args{in: "Physical size: 1440x3040"}, wantWidth: 1440, wantLength: 3040, wantErr: false},
		{name: "Pixel XL", args: args{in: "Physical size: 1440x2560"}, wantWidth: 1440, wantLength: 2560, wantErr: false},
		{name: "garbage", args: args{in: "asdfhjkla"}, wantWidth: -1, wantLength: -1, wantErr: true},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, l, err := parseScreenResolution(tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseScreenResolution() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if w != tt.wantWidth {
				t.Errorf("parseScreenResolution() got = %v, want %v", w, tt.wantWidth)
			}
			if l != tt.wantLength {
				t.Errorf("parseScreenResolution() got1 = %v, want %v", l, tt.wantLength)
			}
		})
	}
}
