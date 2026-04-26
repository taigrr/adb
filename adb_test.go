package adb

import (
	"net"
	"reflect"
	"testing"
	"time"
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
			name: "no devices", args: args{stdout: `List of devices attached`},
			wantErr: false,
			want:    []Device{},
		},
		{
			name: "1 auth dev", args: args{stdout: `List of devices attached
19291FDEE0023W  device`},
			wantErr: false,
			want: []Device{
				{IsAuthorized: true, SerialNo: "19291FDEE0023W", ConnType: USB},
			},
		},
		{
			name: "1 unauth dev", args: args{stdout: `List of devices attached
HT75R0202681    unauthorized`},
			wantErr: false,
			want: []Device{
				{IsAuthorized: false, SerialNo: "HT75R0202681", ConnType: USB},
			},
		},
		{
			name: "2 auth 1 unauth", args: args{stdout: `List of devices attached
19291FDEE0023W  device
9B061FFBA00BC9  device
HT75R0202681    unauthorized`},
			wantErr: false,
			want: []Device{
				{IsAuthorized: true, SerialNo: "19291FDEE0023W", ConnType: USB},
				{IsAuthorized: true, SerialNo: "9B061FFBA00BC9", ConnType: USB},
				{IsAuthorized: false, SerialNo: "HT75R0202681", ConnType: USB},
			},
		},
		{
			name:    "empty string",
			args:    args{stdout: ""},
			wantErr: false,
			want:    []Device{},
		},
		{
			name: "offline device",
			args: args{stdout: `List of devices attached
ABCD1234	offline`},
			wantErr: false,
			want: []Device{
				{IsAuthorized: false, SerialNo: "ABCD1234", ConnType: USB},
			},
		},
		{
			name: "network device",
			args: args{stdout: `List of devices attached
192.168.1.10:5555  device`},
			wantErr: false,
			want: []Device{
				{IsAuthorized: true, SerialNo: "192.168.1.10:5555", ConnType: Network, IP: net.IPAddr{IP: net.ParseIP("192.168.1.10")}, Port: 5555},
			},
		},
		{
			name: "extra whitespace lines",
			args: args{stdout: `List of devices attached

19291FDEE0023W  device

`},
			wantErr: false,
			want: []Device{
				{IsAuthorized: true, SerialNo: "19291FDEE0023W", ConnType: USB},
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

func TestDevice_ConnString(t *testing.T) {
	tests := []struct {
		name string
		dev  Device
		want string
	}{
		{
			name: "default port",
			dev:  Device{IP: net.IPAddr{IP: net.ParseIP("192.168.1.100")}},
			want: "192.168.1.100:5555",
		},
		{
			name: "custom port",
			dev:  Device{IP: net.IPAddr{IP: net.ParseIP("10.0.0.5")}, Port: 5556},
			want: "10.0.0.5:5556",
		},
		{
			name: "ipv6",
			dev:  Device{IP: net.IPAddr{IP: net.ParseIP("::1")}, Port: 5555},
			want: "[::1]:5555",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dev.ConnString()
			if got != tt.want {
				t.Errorf("ConnString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_parseConnectedDevice(t *testing.T) {
	tests := []struct {
		name    string
		stdout  string
		want    Device
		wantErr bool
	}{
		{
			name:   "connected",
			stdout: "connected to 192.168.1.10:5555\n",
			want:   Device{SerialNo: "192.168.1.10:5555", IsAuthorized: true, ConnType: Network, IP: net.IPAddr{IP: net.ParseIP("192.168.1.10")}, Port: 5555},
		},
		{
			name:   "already connected",
			stdout: "already connected to 192.168.1.10:5555\n",
			want:   Device{SerialNo: "192.168.1.10:5555", IsAuthorized: true, ConnType: Network, IP: net.IPAddr{IP: net.ParseIP("192.168.1.10")}, Port: 5555},
		},
		{
			name:    "unparseable output",
			stdout:  "unable to connect to 192.168.1.10:5555\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConnectedDevice(tt.stdout)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseConnectedDevice() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseConnectedDevice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTapSequence_ShortenSleep(t *testing.T) {
	seq := TapSequence{
		Events: []Input{
			SequenceTap{X: 100, Y: 200, Type: SeqTap},
			SequenceSleep{Duration: time.Second * 4, Type: SeqSleep},
			SequenceTap{X: 300, Y: 400, Type: SeqTap},
		},
	}
	shortened := seq.ShortenSleep(2)
	if len(shortened.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(shortened.Events))
	}
	sleep, ok := shortened.Events[1].(SequenceSleep)
	if !ok {
		t.Fatal("expected second event to be SequenceSleep")
	}
	if sleep.Duration != time.Second*2 {
		t.Errorf("expected sleep duration 2s, got %v", sleep.Duration)
	}
}

func TestTapSequence_GetLength(t *testing.T) {
	now := time.Now()
	seq := TapSequence{
		Events: []Input{
			SequenceSleep{Duration: time.Second * 10, Type: SeqSleep},
			SequenceSwipe{
				X1: 0, Y1: 0, X2: 100, Y2: 100,
				Start: now, End: now.Add(time.Second * 5),
				Type: SeqSwipe,
			},
		},
	}
	got := seq.GetLength()
	// 15s * 110/100 = 16.5s
	want := time.Second * 15 * 110 / 100
	if got != want {
		t.Errorf("GetLength() = %v, want %v", got, want)
	}
}

func TestTapSequence_JSONRoundTrip(t *testing.T) {
	now := time.UnixMilli(1700000000000)
	original := TapSequence{
		Resolution: Resolution{Width: 1080, Height: 2340},
		Events: []Input{
			SequenceSwipe{
				X1: 10, Y1: 20, X2: 30, Y2: 40,
				Start: now, End: now.Add(time.Millisecond * 500),
				Type: SeqSwipe,
			},
		},
	}
	jsonBytes := original.ToJSON()
	roundTripped, err := TapSequenceFromJSON(jsonBytes)
	if err != nil {
		t.Fatalf("TapSequenceFromJSON() error = %v", err)
	}
	if roundTripped.Resolution != original.Resolution {
		t.Errorf("Resolution mismatch: got %v, want %v", roundTripped.Resolution, original.Resolution)
	}
	if len(roundTripped.Events) != len(original.Events) {
		t.Fatalf("Events length mismatch: got %d, want %d", len(roundTripped.Events), len(original.Events))
	}
}

func TestSequenceImporter_ToInput(t *testing.T) {
	now := time.UnixMilli(1700000000000)
	tests := []struct {
		name     string
		importer SequenceImporter
		wantType SeqType
	}{
		{
			name:     "sleep",
			importer: SequenceImporter{Type: SeqSleep, Duration: time.Second},
			wantType: SeqSleep,
		},
		{
			name:     "tap",
			importer: SequenceImporter{Type: SeqTap, X: 10, Y: 20, Start: now, End: now},
			wantType: SeqTap,
		},
		{
			name:     "swipe",
			importer: SequenceImporter{Type: SeqSwipe, X1: 10, Y1: 20, X2: 30, Y2: 40, Start: now, End: now.Add(time.Second)},
			wantType: SeqSwipe,
		},
		{
			name:     "unknown defaults to sleep",
			importer: SequenceImporter{Type: SeqType(99)},
			wantType: SeqSleep,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.importer.ToInput()
			switch tt.wantType {
			case SeqSleep:
				if _, ok := input.(SequenceSleep); !ok {
					t.Errorf("expected SequenceSleep, got %T", input)
				}
			case SeqTap:
				if _, ok := input.(SequenceTap); !ok {
					t.Errorf("expected SequenceTap, got %T", input)
				}
			case SeqSwipe:
				if _, ok := input.(SequenceSwipe); !ok {
					t.Errorf("expected SequenceSwipe, got %T", input)
				}
			}
		})
	}
}

func TestInsertSleeps(t *testing.T) {
	now := time.UnixMilli(1000)
	inputs := []Input{
		SequenceTap{X: 1, Y: 2, Start: now, End: now.Add(time.Millisecond * 100), Type: SeqTap},
		SequenceTap{X: 3, Y: 4, Start: now.Add(time.Millisecond * 500), End: now.Add(time.Millisecond * 600), Type: SeqTap},
	}
	result := insertSleeps(inputs)
	// Should be: tap, sleep, tap
	if len(result) != 3 {
		t.Fatalf("expected 3 events, got %d", len(result))
	}
	sleep, ok := result[1].(SequenceSleep)
	if !ok {
		t.Fatal("expected second event to be SequenceSleep")
	}
	// Sleep should be from end of first (100ms) to end of second (600ms) = 500ms
	if sleep.Duration != time.Millisecond*500 {
		t.Errorf("expected sleep duration 500ms, got %v", sleep.Duration)
	}
}
