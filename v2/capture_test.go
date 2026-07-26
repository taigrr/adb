package adb

import (
	"context"
	_ "embed"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

//go:embed tests/multitouch.log
var multitouch string

//go:embed tests/tablet.log
var tablet string

//go:embed tests/pixel.log
var pixel string

func TestTrimDeviceDescriptors(t *testing.T) {
	for _, tc := range []struct {
		name string
		log  string
	}{{"pixel", pixel}, {"multitouch", multitouch}, {"tablet", tablet}} {
		t.Run(tc.name, func(t *testing.T) {
			lines := strings.Split(tc.log, "\n")
			if !strings.Contains(lines[0], "add device") {
				t.Fatalf("line 0 expected to contain 'add device': %s", lines[0])
			}
			trimmed := trimDeviceDescriptors(lines)
			if len(trimmed) == 0 {
				t.Fatal("trimDeviceDescriptors returned no event lines")
			}
			for i, line := range trimmed {
				if !reTimestamp.MatchString(line) {
					t.Fatalf("kept non-event line %d: %q", i, line)
				}
			}
		})
	}
}

func TestTracksToEvents_SingleFingerCounts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		log   string
		count int
	}{{"pixel", pixel, 7}, {"tablet", tablet, 9}} {
		t.Run(tc.name, func(t *testing.T) {
			raws := parseRawEvents(trimDeviceDescriptors(strings.Split(tc.log, "\n")))
			if got := len(tracksToEvents(raws)); got != tc.count {
				t.Fatalf("tracksToEvents() = %d swipes, want %d", got, tc.count)
			}
		})
	}
}

func TestTracksToEvents_MultitouchSplitsPerFinger(t *testing.T) {
	// The multitouch fixture has one BTN_TOUCH DOWN/UP pair but three fingers
	// (slots 0/1/2). The old BTN_TOUCH-delimited parser collapsed these into a
	// single swipe with cross-finger coordinates; the slot-aware parser must
	// emit one event per finger, each with self-consistent coordinates.
	events := tracksToEvents(parseRawEvents(trimDeviceDescriptors(strings.Split(multitouch, "\n"))))
	if len(events) != 3 {
		t.Fatalf("multitouch produced %d events, want 3 (one per finger)", len(events))
	}
	// Finger 1 (slot 0) starts at 0x241,0x90b = 577,2315.
	if events[0].X1 != 0x241 || events[0].Y1 != 0x90b {
		t.Fatalf("finger 1 start = (%d,%d), want (577,2315)", events[0].X1, events[0].Y1)
	}
	// Every event must be ordered by start time.
	for i := 1; i < len(events); i++ {
		if events[i].Start.Before(events[i-1].Start) {
			t.Fatalf("events not ordered by start time at %d", i)
		}
	}
}

func TestParseGetEvent_ProducesSwipesAndSleeps(t *testing.T) {
	events := parseGetEvent(pixel)
	if len(events) == 0 {
		t.Fatal("parseGetEvent() produced no events")
	}
	var swipes, sleeps int
	var firstSwipe *event
	for i, e := range events {
		switch e.Kind {
		case kindSwipe:
			swipes++
			if firstSwipe == nil {
				firstSwipe = &events[i]
			}
		case kindSleep:
			sleeps++
		}
	}
	if swipes != 7 {
		t.Fatalf("expected 7 swipes from pixel fixture, got %d", swipes)
	}
	// Sleeps are interleaved between touches, so with N swipes there are N-1 sleeps.
	if sleeps != swipes-1 {
		t.Fatalf("expected %d sleeps, got %d", swipes-1, sleeps)
	}
	// pixel's first touch begins at 0x450,0x669 (line 11/12 of the fixture).
	if firstSwipe.X1 != 0x450 || firstSwipe.Y1 != 0x669 {
		t.Fatalf("first swipe start = (%d,%d), want (%d,%d)", firstSwipe.X1, firstSwipe.Y1, 0x450, 0x669)
	}
	// Events must alternate swipe, sleep, swipe, ...
	for i, e := range events {
		wantSleep := i%2 == 1
		if wantSleep && e.Kind != kindSleep {
			t.Fatalf("event %d expected sleep", i)
		}
		if !wantSleep && e.Kind != kindSwipe {
			t.Fatalf("event %d expected swipe", i)
		}
	}
}

func TestSequenceJSONRoundTrip(t *testing.T) {
	now := time.UnixMilli(1700000000000)
	original := Sequence{
		resolution: Resolution{Width: 1080, Height: 2340},
		events: []event{
			{Kind: kindSwipe, X1: 10, Y1: 20, X2: 30, Y2: 40, Start: now, End: now.Add(500 * time.Millisecond)},
			{Kind: kindSleep, Duration: 2 * time.Second},
		},
	}
	data, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}
	got, err := ParseSequence(data)
	if err != nil {
		t.Fatalf("ParseSequence() error = %v", err)
	}
	if got.resolution != original.resolution {
		t.Fatalf("resolution = %v, want %v", got.resolution, original.resolution)
	}
	if got.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", got.Len())
	}
	if got.events[1].Duration != 2*time.Second {
		t.Fatalf("sleep duration = %v, want 2s", got.events[1].Duration)
	}
	// The swipe's replay length must survive the round trip.
	if got.events[0].length() != 500*time.Millisecond {
		t.Fatalf("swipe length = %v, want 500ms", got.events[0].length())
	}
}

func TestShortenSleeps(t *testing.T) {
	s := Sequence{events: []event{
		{Kind: kindSwipe, X1: 1, Y1: 2, X2: 1, Y2: 2},
		{Kind: kindSleep, Duration: 4 * time.Second},
	}}
	got := s.ShortenSleeps(2)
	if got.events[1].Duration != 2*time.Second {
		t.Fatalf("sleep = %v, want 2s", got.events[1].Duration)
	}
	// Original must be unchanged.
	if s.events[1].Duration != 4*time.Second {
		t.Fatalf("original mutated: %v", s.events[1].Duration)
	}
	// factor <= 0 is a no-op.
	if s.ShortenSleeps(0).events[1].Duration != 4*time.Second {
		t.Fatal("ShortenSleeps(0) should be a no-op")
	}
}

func TestInsertSleeps_GapIsStartMinusPrevEnd(t *testing.T) {
	now := time.UnixMilli(0)
	swipes := []event{
		{Kind: kindSwipe, Start: now, End: now.Add(100 * time.Millisecond)},
		{Kind: kindSwipe, Start: now.Add(500 * time.Millisecond), End: now.Add(700 * time.Millisecond)},
	}
	out := insertSleeps(swipes)
	if len(out) != 3 {
		t.Fatalf("expected 3 events, got %d", len(out))
	}
	if out[1].Kind != kindSleep {
		t.Fatal("expected middle event to be a sleep")
	}
	// Gap = start2 (500ms) - end1 (100ms) = 400ms, not end2-end1 (600ms).
	if out[1].Duration != 400*time.Millisecond {
		t.Fatalf("sleep duration = %v, want 400ms", out[1].Duration)
	}
}

func TestTrimDeviceDescriptors_NoTrailingNewline(t *testing.T) {
	// Descriptor lines (no timestamp) are dropped; every timestamped event
	// line is kept, including the last even without a trailing newline.
	input := []string{"add device 1: /dev/input/event0", "  name: x", "[ 1.0] EV_KEY BTN_TOUCH DOWN", "[ 1.1] EV_KEY BTN_TOUCH UP"}
	got := trimDeviceDescriptors(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(got), got)
	}
	if got[len(got)-1] != "[ 1.1] EV_KEY BTN_TOUCH UP" {
		t.Fatalf("last line dropped: %v", got)
	}
}

func TestTrimDeviceDescriptors_NoTouch(t *testing.T) {
	if got := trimDeviceDescriptors([]string{"add device 1", "name: x"}); got != nil {
		t.Fatalf("expected nil for no touch, got %v", got)
	}
}

func TestReplay_TapVsLongPressRouting(t *testing.T) {
	tests := []struct {
		name string
		ev   event
		want []string
	}{
		{
			name: "instant tap uses input tap",
			ev:   event{Kind: kindSwipe, X1: 10, Y1: 20, X2: 10, Y2: 20},
			want: []string{"-s", "S", "shell", "input", "tap", "10", "20"},
		},
		{
			name: "stationary long-press uses same-point swipe with duration",
			ev: event{
				Kind: kindSwipe, X1: 10, Y1: 20, X2: 10, Y2: 20,
				Start: time.Time{}, End: time.Time{}.Add(600 * time.Millisecond),
			},
			want: []string{"-s", "S", "shell", "input", "swipe", "10", "20", "10", "20", "600"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, argsFile := fakeADB(t, "", "", 0)
			if err := tt.ev.play(context.Background(), fakeDevice(c, "S", Network)); err != nil {
				t.Fatalf("play() error = %v", err)
			}
			if got := readArgs(t, argsFile); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("play() args = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewSequenceAndEventsRoundTrip(t *testing.T) {
	seq := NewSequence(Resolution{Width: 1080, Height: 2340},
		NewTap(10, 20),
		NewSleep(time.Second),
		NewSwipe(0, 0, 100, 200, 300*time.Millisecond),
	)
	events := seq.Events()
	if len(events) != 3 {
		t.Fatalf("Events() len = %d, want 3", len(events))
	}
	if events[0] != (Event{Kind: SwipeEvent, X1: 10, Y1: 20, X2: 10, Y2: 20}) {
		t.Fatalf("tap event = %#v", events[0])
	}
	if events[1] != (Event{Kind: SleepEvent, Duration: time.Second}) {
		t.Fatalf("sleep event = %#v", events[1])
	}
	if events[2] != (Event{Kind: SwipeEvent, X2: 100, Y2: 200, Duration: 300 * time.Millisecond}) {
		t.Fatalf("swipe event = %#v", events[2])
	}
}

func TestSequenceEncodingJSONRoundTrip(t *testing.T) {
	original := NewSequence(Resolution{Width: 1080, Height: 2340},
		NewSwipe(10, 20, 30, 40, 500*time.Millisecond),
		NewSleep(2*time.Second),
	)
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	var got Sequence
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if got.Resolution() != original.Resolution() {
		t.Fatalf("resolution = %v, want %v", got.Resolution(), original.Resolution())
	}
	if !reflect.DeepEqual(got.Events(), original.Events()) {
		t.Fatalf("events = %v, want %v", got.Events(), original.Events())
	}
}

func TestSequenceDuration(t *testing.T) {
	now := time.UnixMilli(0)
	s := Sequence{events: []event{
		{Kind: kindSleep, Duration: 10 * time.Second},
		{Kind: kindSwipe, Start: now, End: now.Add(5 * time.Second)},
	}}
	// (10s + 5s) * 110/100
	want := 15 * time.Second * 110 / 100
	if got := s.Duration(); got != want {
		t.Fatalf("Duration() = %v, want %v", got, want)
	}
}
