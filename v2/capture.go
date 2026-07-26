package adb

import (
	"context"
	"encoding/json"
	"maps"
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

// Sequence is a series of taps, swipes, and pauses that can be replayed
// against a device. Create one by recording ([Device.Record]), building it
// programmatically ([NewSequence]), or restoring JSON ([ParseSequence] /
// [Sequence.UnmarshalJSON]).
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

// event is a single replayable action. Taps are represented as zero-distance
// swipes because Android promotes short swipes to taps automatically.
type event struct {
	Kind     eventKind
	X1, Y1   int
	X2, Y2   int
	Duration time.Duration
	Start    time.Time
	End      time.Time
}

// EventKind identifies the kind of an [Event].
type EventKind int

const (
	// SwipeEvent is a swipe (a tap is a swipe with equal endpoints).
	SwipeEvent EventKind = iota
	// SleepEvent is a pause between actions.
	SleepEvent
)

// Event is a single replayable action in a [Sequence] as seen through the
// public API. Obtain events via [Sequence.Events] and build them with
// [NewTap], [NewSwipe], and [NewSleep].
type Event struct {
	Kind     EventKind
	X1, Y1   int
	X2, Y2   int
	Duration time.Duration
}

// NewTap returns an Event that taps the point (x, y).
func NewTap(x, y int) Event {
	return Event{Kind: SwipeEvent, X1: x, Y1: y, X2: x, Y2: y}
}

// NewSwipe returns an Event that swipes from (x1, y1) to (x2, y2) over d.
func NewSwipe(x1, y1, x2, y2 int, d time.Duration) Event {
	return Event{Kind: SwipeEvent, X1: x1, Y1: y1, X2: x2, Y2: y2, Duration: d}
}

// NewSleep returns an Event that pauses for d.
func NewSleep(d time.Duration) Event {
	return Event{Kind: SleepEvent, Duration: d}
}

// NewSequence builds a Sequence from the given events. The resolution is
// advisory metadata describing the screen the events were authored for.
func NewSequence(resolution Resolution, events ...Event) Sequence {
	s := Sequence{resolution: resolution, events: make([]event, len(events))}
	for i, e := range events {
		s.events[i] = e.toInternal()
	}
	return s
}

func (e Event) toInternal() event {
	if e.Kind == SleepEvent {
		return event{Kind: kindSleep, Duration: e.Duration}
	}
	return event{
		Kind: kindSwipe,
		X1:   e.X1, Y1: e.Y1, X2: e.X2, Y2: e.Y2,
		Start: time.Time{},
		End:   time.Time{}.Add(e.Duration),
	}
}

func (e event) toPublic() Event {
	if e.Kind == kindSleep {
		return Event{Kind: SleepEvent, Duration: e.Duration}
	}
	return Event{
		Kind: SwipeEvent,
		X1:   e.X1, Y1: e.Y1, X2: e.X2, Y2: e.Y2,
		Duration: e.length(),
	}
}

// Events returns a snapshot of the sequence's events.
func (s Sequence) Events() []Event {
	out := make([]Event, len(s.events))
	for i, e := range s.events {
		out[i] = e.toPublic()
	}
	return out
}

// Resolution returns the screen resolution associated with the sequence.
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

// UnmarshalJSON implements [json.Unmarshaler], the inverse of
// [Sequence.MarshalJSON], so a Sequence round-trips through encoding/json.
func (s *Sequence) UnmarshalJSON(data []byte) error {
	parsed, err := ParseSequence(data)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
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
	// A cancelled recording that captured no touches is a valid empty result,
	// not an error. Only surface ErrStdoutEmpty when the command ended on its
	// own (not via cancellation) with nothing captured.
	if len(res.Stdout) == 0 && ctx.Err() == nil {
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
	// Only a zero-distance, zero-duration contact is an instantaneous tap.
	// A stationary contact with a real duration is a press/long-press and must
	// preserve its duration, which `input swipe` with equal endpoints does
	// (a zero-duration swipe would be degenerate, hence the tap fast-path).
	if e.X1 == e.X2 && e.Y1 == e.Y2 && e.length() <= 0 {
		return d.Tap(ctx, e.X1, e.Y1)
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
	events := tracksToEvents(raws)
	return insertSleeps(events)
}

var reGetEvent = regexp.MustCompile(`\[\s*(\d+\.\d+)]\s*(?:[^:]*:)?\s*(\w+)\s+(\w+)\s+(\w+)`)

func parseRawEvents(lines []string) []rawEvent {
	var raws []rawEvent
	for _, line := range lines {
		m := reGetEvent.FindStringSubmatch(line)
		if len(m) != 5 {
			continue
		}
		f, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		raws = append(raws, rawEvent{
			// getevent timestamps are monotonic uptime, not wall-clock; only
			// relative differences are used, so encoding them as a duration
			// offset from the zero time is sufficient and preserves precision.
			timestamp: time.Time{}.Add(time.Duration(f * float64(time.Second))),
			kind:      m[2],
			key:       m[3],
			value:     m[4],
		})
	}
	return raws
}

// reTimestamp matches an event line's leading `[ uptime]` timestamp, which is
// present only on event lines, never on the device-capability descriptor block
// that getevent prints first.
var reTimestamp = regexp.MustCompile(`^\[\s*\d+\.\d+\]`)

// trimDeviceDescriptors drops the leading device-capability descriptor block
// that getevent prints before the event stream, keeping only timestamped event
// lines (trimmed of surrounding whitespace). It does not mutate its input and
// returns nil when there are no event lines.
func trimDeviceDescriptors(input []string) []string {
	out := make([]string, 0, len(input))
	for _, line := range input {
		trimmed := strings.TrimSpace(line)
		if reTimestamp.MatchString(trimmed) {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// contact tracks a single finger's touch as it moves across the screen.
type contact struct {
	active   bool
	hasStart bool
	hasCur   bool
	trackID  string
	curX     int
	curY     int
	startX   int
	startY   int
	lastX    int
	lastY    int
	startT   time.Time
	lastT    time.Time
}

// tracksToEvents converts the raw evdev stream into one swipe event per finger
// contact. It understands the protocol-B multitouch model:
//   - ABS_MT_SLOT selects the active finger slot,
//   - ABS_MT_TRACKING_ID (a value other than the -1 sentinel ffffffff) begins a
//     contact and ffffffff ends it,
//   - devices that omit tracking-id teardown instead delimit with BTN_TOUCH
//     DOWN/UP, which is handled as a fallback.
//
// A tap is a contact whose start and end coincide. Events are returned ordered
// by their start time.
func tracksToEvents(raws []rawEvent) []event {
	// Protocol B streams carry ABS_MT_SLOT and per-slot tracking ids; protocol
	// A streams (SYN_MT_REPORT, no slots) reuse tracking ids per frame, so
	// tracking-id transitions are only meaningful when slots are in use.
	usesSlots := false
	for _, r := range raws {
		if r.kind == "EV_ABS" && r.key == "ABS_MT_SLOT" {
			usesSlots = true
			break
		}
	}

	slots := map[int]*contact{}
	curSlot := 0
	var out []event

	get := func(slot int) *contact {
		c := slots[slot]
		if c == nil {
			c = &contact{}
			slots[slot] = c
		}
		return c
	}
	commit := func(c *contact, t time.Time) {
		if !c.active || !c.hasCur {
			return
		}
		c.lastX, c.lastY, c.lastT = c.curX, c.curY, t
		if !c.hasStart {
			c.startX, c.startY, c.startT = c.curX, c.curY, t
			c.hasStart = true
		}
	}
	closeSlot := func(slot int, t time.Time) {
		c := slots[slot]
		if c == nil || !c.active {
			return
		}
		commit(c, t)
		if c.hasStart {
			out = append(out, event{
				Kind: kindSwipe,
				X1:   c.startX, Y1: c.startY,
				X2: c.lastX, Y2: c.lastY,
				Start: c.startT,
				End:   c.lastT,
			})
		}
		delete(slots, slot)
	}

	for _, r := range raws {
		switch {
		case r.kind == "EV_ABS" && r.key == "ABS_MT_SLOT":
			curSlot = hexToInt(r.value)
		case r.kind == "EV_ABS" && r.key == "ABS_MT_TRACKING_ID":
			if !usesSlots {
				// Protocol A: tracking ids are per-frame, not per-contact;
				// contacts are delimited by BTN_TOUCH instead.
				continue
			}
			if strings.EqualFold(r.value, "ffffffff") {
				closeSlot(curSlot, r.timestamp)
			} else {
				// A *different* tracking id on a slot that is still active means
				// the previous finger was replaced without an ffffffff
				// teardown; close the old contact before beginning the new one.
				// The same id repeated is a continuation.
				if c := slots[curSlot]; c != nil && c.hasStart && c.trackID != r.value {
					closeSlot(curSlot, r.timestamp)
				}
				c := get(curSlot)
				c.active = true
				c.trackID = r.value
			}
		case r.isPositionX():
			c := get(curSlot)
			c.active, c.hasCur, c.curX = true, true, hexToInt(r.value)
		case r.isPositionY():
			c := get(curSlot)
			c.active, c.hasCur, c.curY = true, true, hexToInt(r.value)
		case r.isBTNDown():
			get(curSlot).active = true
		case r.isBTNUp():
			// No tracking-id teardown on this device: BTN_TOUCH UP ends every
			// active contact. Iterate slots in a stable order for determinism.
			for _, slot := range slices.Sorted(maps.Keys(slots)) {
				closeSlot(slot, r.timestamp)
			}
		case r.kind == "EV_SYN":
			for _, c := range slots {
				commit(c, r.timestamp)
			}
		}
	}
	// Flush contacts still active at end-of-stream (e.g. recording cancelled
	// mid-gesture) so the final touch is not silently dropped.
	for _, slot := range slices.Sorted(maps.Keys(slots)) {
		closeSlot(slot, lastTimestamp(raws))
	}

	// Order by start time; break ties by start coordinate so simultaneous
	// contacts (same SYN frame) are deterministic regardless of map iteration.
	slices.SortStableFunc(out, func(a, b event) int {
		if c := a.Start.Compare(b.Start); c != 0 {
			return c
		}
		if a.X1 != b.X1 {
			return a.X1 - b.X1
		}
		return a.Y1 - b.Y1
	})
	return out
}

func lastTimestamp(raws []rawEvent) time.Time {
	if len(raws) == 0 {
		return time.Time{}
	}
	return raws[len(raws)-1].timestamp
}

func hexToInt(s string) int {
	v, _ := strconv.ParseInt(s, 16, 64)
	return int(v)
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
