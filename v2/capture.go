package adb

import (
	"context"
	"encoding/json"
	"iter"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Resolution is a device screen resolution in pixels.
type Resolution struct {
	Width  int
	Height int
}

// Sequence is a recorded series of taps, swipes, and pauses that can be
// replayed against a device. It is created by [Device.Record], serialized with
// [Sequence.MarshalJSON], and restored with [ParseSequence].
type Sequence struct {
	resolution Resolution
	events     []event
}

// eventKind identifies the kind of a recorded event.
type eventKind int

const (
	kindSwipe eventKind = iota
	kindSleep
)

// event is a single replayable action. Taps are represented as zero-length
// swipes because Android promotes short swipes to taps automatically.
type event struct {
	Kind     eventKind
	X1, Y1   int
	X2, Y2   int
	Duration time.Duration
	Start    time.Time
	End      time.Time
}

// Resolution returns the screen resolution captured with the sequence.
func (s Sequence) Resolution() Resolution { return s.resolution }

// Len returns the number of recorded events.
func (s Sequence) Len() int { return len(s.events) }

// Duration returns the total wall-clock time the sequence takes to replay,
// including a 10% safety margin.
func (s Sequence) Duration() time.Duration {
	var total time.Duration
	for _, e := range s.events {
		total += e.length()
	}
	// Add the margin as total/10 to avoid overflowing the intermediate
	// product for very long recordings.
	return total + total/10
}

func (e event) length() time.Duration {
	switch e.Kind {
	case kindSleep:
		return e.Duration
	default:
		return e.End.Sub(e.Start)
	}
}

// ShortenSleeps divides every pause between events by factor, leaving swipe
// durations untouched. A factor <= 0 returns the sequence unchanged.
func (s Sequence) ShortenSleeps(factor int) Sequence {
	if factor <= 0 {
		return s
	}
	out := Sequence{resolution: s.resolution, events: make([]event, len(s.events))}
	copy(out.events, s.events)
	for i := range out.events {
		if out.events[i].Kind == kindSleep {
			out.events[i].Duration /= time.Duration(factor)
		}
	}
	return out
}

// sequenceJSON is the on-disk representation of a Sequence.
type sequenceJSON struct {
	Resolution Resolution  `json:"resolution"`
	Events     []eventJSON `json:"events"`
}

type eventJSON struct {
	Kind     string        `json:"kind"`
	X1       int           `json:"x1,omitempty"`
	Y1       int           `json:"y1,omitempty"`
	X2       int           `json:"x2,omitempty"`
	Y2       int           `json:"y2,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`
}

// MarshalJSON implements [json.Marshaler].
func (s Sequence) MarshalJSON() ([]byte, error) {
	out := sequenceJSON{Resolution: s.resolution, Events: make([]eventJSON, len(s.events))}
	for i, e := range s.events {
		ej := eventJSON{X1: e.X1, Y1: e.Y1, X2: e.X2, Y2: e.Y2}
		switch e.Kind {
		case kindSleep:
			ej.Kind = "sleep"
			ej.Duration = e.Duration
		default:
			ej.Kind = "swipe"
			ej.Duration = e.End.Sub(e.Start)
		}
		out.Events[i] = ej
	}
	return json.Marshal(out)
}

// ParseSequence restores a [Sequence] previously produced by
// [Sequence.MarshalJSON].
func ParseSequence(data []byte) (Sequence, error) {
	var in sequenceJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return Sequence{}, err
	}
	s := Sequence{resolution: in.Resolution, events: make([]event, 0, len(in.Events))}
	for _, ej := range in.Events {
		e := event{X1: ej.X1, Y1: ej.Y1, X2: ej.X2, Y2: ej.Y2, Duration: ej.Duration}
		if ej.Kind == "sleep" {
			e.Kind = kindSleep
		} else {
			e.Kind = kindSwipe
			e.End = time.Time{}.Add(ej.Duration)
		}
		s.events = append(s.events, e)
	}
	return s, nil
}

// Record captures a series of screen touches until ctx is cancelled,
// equivalent to `adb shell getevent -tl`. The command only ends when the
// context expires, so callers should pass a context with a timeout or cancel
// it when done recording.
func (d Device) Record(ctx context.Context) (Sequence, error) {
	resolution, err := d.ScreenResolution(ctx)
	if err != nil {
		return Sequence{}, err
	}
	// getevent never exits on its own; context cancellation is the expected
	// stop condition and is not treated as an error. Any other failure (device
	// not found, unauthorized, offline) must surface. exec is used directly so
	// the client's default timeout does not silently cut the recording short.
	res, err := d.client.exec(ctx, "-s", d.serial, "shell", "getevent", "-tl")
	if err != nil && ctx.Err() == nil {
		return Sequence{}, err
	}
	if len(res.Stdout) == 0 {
		if ctx.Err() != nil {
			return Sequence{}, ctx.Err()
		}
		return Sequence{}, ErrStdoutEmpty
	}
	return Sequence{resolution: resolution, events: parseGetEvent(res.StdoutString())}, nil
}

// Replay plays every event in the sequence against the device in order,
// stopping on the first error. Pauses honor ctx cancellation.
func (d Device) Replay(ctx context.Context, s Sequence) error {
	for _, e := range s.events {
		if err := e.play(ctx, d); err != nil {
			return err
		}
	}
	return nil
}

func (e event) play(ctx context.Context, d Device) error {
	if e.Kind == kindSleep {
		timer := time.NewTimer(e.Duration)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}
	return d.Swipe(ctx, e.X1, e.Y1, e.X2, e.Y2, e.length())
}

// --- getevent parsing ---

type rawEvent struct {
	timestamp time.Time
	kind      string
	key       string
	value     string
}

func (r rawEvent) isBTNTouch() bool  { return r.key == "BTN_TOUCH" }
func (r rawEvent) isBTNUp() bool     { return r.isBTNTouch() && r.value == "UP" }
func (r rawEvent) isBTNDown() bool   { return r.isBTNTouch() && r.value == "DOWN" }
func (r rawEvent) isPositionX() bool { return r.kind == "EV_ABS" && r.key == "ABS_MT_POSITION_X" }
func (r rawEvent) isPositionY() bool { return r.kind == "EV_ABS" && r.key == "ABS_MT_POSITION_Y" }

func parseGetEvent(input string) []event {
	lines := trimDeviceDescriptors(strings.Split(input, "\n"))
	raws := parseRawEvents(lines)
	touches := groupTouches(raws)
	events := touchesToEvents(touches)
	return insertSleeps(events)
}

var reGetEvent = regexp.MustCompile(`\[\s*(\d+\.\d+)]\s*(.*):\s*(\w*)\s*(\w*)\s*(\w*)`)

func parseRawEvents(lines []string) []rawEvent {
	var raws []rawEvent
	for _, line := range lines {
		m := reGetEvent.FindStringSubmatch(line)
		if len(m) != 6 {
			continue
		}
		f, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		raws = append(raws, rawEvent{
			timestamp: time.UnixMilli(int64(f * 1000)),
			kind:      m[3],
			key:       m[4],
			value:     m[5],
		})
	}
	return raws
}

// trimDeviceDescriptors drops the leading `add device`/descriptor lines that
// precede the first touch and trims surrounding whitespace. It does not mutate
// its input. If no touch (DOWN) is present it returns nil.
func trimDeviceDescriptors(input []string) []string {
	start := -1
	for i, line := range input {
		if strings.Contains(line, "DOWN") {
			start = i
			break
		}
	}
	if start < 0 {
		return nil
	}
	out := make([]string, 0, len(input)-start)
	for _, line := range input[start:] {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// groupTouches splits raw events into groups that each start with a DOWN and
// end with the following UP.
func groupTouches(raws []rawEvent) [][]rawEvent {
	var groups [][]rawEvent
	var current []rawEvent
	foundDown := false
	for _, r := range raws {
		if !foundDown {
			if r.isBTNDown() {
				foundDown = true
			} else {
				continue
			}
		}
		current = append(current, r)
		if r.isBTNUp() {
			groups = append(groups, current)
			current = nil
			foundDown = false
		}
	}
	return groups
}

func touchesToEvents(groups [][]rawEvent) []event {
	events := []event{}
	for _, g := range groups {
		if e, ok := groupToEvent(g); ok {
			events = append(events, e)
		}
	}
	return events
}

// groupToEvent extracts the first and last X/Y positions from a touch group and
// builds a swipe event. Taps are represented as swipes with equal endpoints.
func groupToEvent(g []rawEvent) (event, bool) {
	startX, startY, okStart := firstPosition(slices.Values(g))
	if !okStart {
		return event{}, false
	}
	endX, endY, okEnd := firstPosition(backward(g))
	if !okEnd {
		return event{}, false
	}
	return event{
		Kind: kindSwipe,
		X1:   startX, Y1: startY,
		X2: endX, Y2: endY,
		Start: g[0].timestamp,
		End:   g[len(g)-1].timestamp,
	}, true
}

// backward yields the elements of g from last to first.
func backward(g []rawEvent) iter.Seq[rawEvent] {
	return func(yield func(rawEvent) bool) {
		for i := len(g) - 1; i >= 0; i-- {
			if !yield(g[i]) {
				return
			}
		}
	}
}

// firstPosition scans the events in seq order and returns the first X and Y
// touch coordinates found (hex-decoded), if both are present.
func firstPosition(seq iter.Seq[rawEvent]) (x, y int, ok bool) {
	var xFound, yFound bool
	for r := range seq {
		if !xFound && r.isPositionX() {
			if v, err := strconv.ParseInt(r.value, 16, 64); err == nil {
				x, xFound = int(v), true
			}
		}
		if !yFound && r.isPositionY() {
			if v, err := strconv.ParseInt(r.value, 16, 64); err == nil {
				y, yFound = int(v), true
			}
		}
		if xFound && yFound {
			break
		}
	}
	return x, y, xFound && yFound
}

func insertSleeps(events []event) []event {
	out := []event{}
	for i, e := range events {
		if i != 0 {
			prevEnd := out[len(out)-1].End
			gap := max(e.Start.Sub(prevEnd), 0)
			out = append(out, event{Kind: kindSleep, Duration: gap})
		}
		out = append(out, e)
	}
	return out
}
