package adb

import (
	_ "embed"
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
			if len(trimmed) > 0 && strings.Contains(trimmed[0], "add device") {
				t.Fatalf("line 0 still contains 'add device': %s", trimmed[0])
			}
		})
	}
}

func TestGroupTouches(t *testing.T) {
	for _, tc := range []struct {
		name  string
		log   string
		count int
	}{{"pixel", pixel, 7}, {"tablet", tablet, 8}} {
		t.Run(tc.name, func(t *testing.T) {
			lines := trimDeviceDescriptors(strings.Split(tc.log, "\n"))
			raws := parseRawEvents(lines)
			if got := len(groupTouches(raws)); got != tc.count {
				t.Fatalf("groupTouches() = %d, want %d", got, tc.count)
			}
		})
	}
}

func TestParseGetEvent_ProducesSwipesAndSleeps(t *testing.T) {
	events := parseGetEvent(pixel)
	if len(events) == 0 {
		t.Fatal("parseGetEvent() produced no events")
	}
	var swipes, sleeps int
	for _, e := range events {
		switch e.Kind {
		case kindSwipe:
			swipes++
		case kindSleep:
			sleeps++
		}
	}
	if swipes == 0 {
		t.Fatal("expected at least one swipe event")
	}
	// Sleeps are interleaved between touches, so with N swipes there are N-1 sleeps.
	if sleeps != swipes-1 {
		t.Fatalf("expected %d sleeps, got %d", swipes-1, sleeps)
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
	// A stream cut mid-recording (no trailing newline) must keep its last line.
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
