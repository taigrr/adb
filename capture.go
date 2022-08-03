package adb

import (
	"context"
	"errors"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CaptureSequence allows you to record a series of taps and swipes on the screen to replay later
//
// This function is useful if you need to run an obscure shell command or if
// you require functionality not provided by the exposed functions here.
// Instead of using Shell, please consider submitting a PR with the functionality
// you require.

type SequenceSleep struct {
	Duration time.Duration
}

func (s SequenceSleep) Play(d Device, ctx context.Context) error {
	// TODO check if context is expired
	time.Sleep(s.Duration)
	return nil
}

func (s SequenceSleep) Length() time.Duration {
	return s.Duration
}

func (s SequenceSleep) StartTime() time.Time {
	return time.Time{}
}

func (s SequenceSleep) EndTime() time.Time {
	return time.Time{}
}

type SequenceTap struct {
	X     int
	Y     int
	Start time.Time
	End   time.Time
}

func (s SequenceTap) Play(d Device, ctx context.Context) error {
	return d.Tap(ctx, s.X, s.Y)
}

func (s SequenceTap) Length() time.Duration {
	return 0
}

func (s SequenceTap) StartTime() time.Time {
	return s.Start
}

func (s SequenceTap) EndTime() time.Time {
	return s.End
}

type SequenceSwipe struct {
	X1    int
	Y1    int
	X2    int
	Y2    int
	Start time.Time
	End   time.Time
}

func (s SequenceSwipe) Play(d Device, ctx context.Context) error {
	return d.Swipe(ctx, s.X1, s.Y1, s.X2, s.Y2, s.Length())
}

func (s SequenceSwipe) StartTime() time.Time {
	return s.Start
}

func (s SequenceSwipe) EndTime() time.Time {
	return s.End
}

func (s SequenceSwipe) Length() time.Duration {
	return s.End.Sub(s.Start)
}

type Input interface {
	Play(d Device, ctx context.Context) error
	Length() time.Duration
	StartTime() time.Time
	EndTime() time.Time
}

type TapSequence struct {
	Events     []Input
	Resolution Resolution
}
type Resolution struct {
	Width  int
	Height int
}

// ShortenSleep allows you to shorten all the sleep times between tap and swipe events.
//
// Provide a scalar value to divide the sleeps by. Providing `2` will halve all
// sleep durations in the TapSequence. Swipe durations and tap durations are
// unaffected.

func (t TapSequence) ShortenSleep(scalar int) TapSequence {
	seq := []Input{}
	for _, s := range t.Events {
		switch y := s.(type) {
		case SequenceSleep:
			y.Duration = y.Duration / time.Duration(scalar)
			seq = append(seq, y)
		default:
			seq = append(seq, s)
		}
	}
	t.Events = seq
	return t
}

// GetLength returns the length of all Input events inside of a given TapSequence
//
// This function is useful for devermining how long a context timeout should
// last when calling ReplayTapSequence
func (t TapSequence) GetLength() time.Duration {
	duration := time.Duration(0)
	for _, x := range t.Events {
		duration += x.Length()
	}
	return duration * 110 / 100
}

func (d Device) ReplayTapSequence(ctx context.Context, t TapSequence) error {
	for _, e := range t.Events {
		err := e.Play(d, ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// CaptureSequence allows you to capture and replay screen taps and swipes.
//
// ctx, cancelFunc := context.WithCancel(context.TODO())
//
// go dev.CaptureSequence(ctx)
// time.Sleep(time.Second * 30)
// cancelFunc()
func (d Device) CaptureSequence(ctx context.Context) (t TapSequence, err error) {
	// this command will never finish without ctx expiring. As a result,
	// it will always return error code 130 if successful
	stdout, _, errCode, err := execute(ctx, []string{"shell", "getevent", "-tl"})
	// TODO better error checking here
	if errors.Is(err, ErrUnspecified) {
		err = nil
	}
	if errCode != 130 && errCode != -1 {
		// TODO remove log output here
		log.Printf("Expected error code 130 or -1, but got %d\n", errCode)
	}

	if stdout == "" {
		return TapSequence{}, ErrStdoutEmpty
	}
	t.Events = parseGetEvent(stdout)
	return
}

type event struct {
	TimeStamp  time.Time
	DevicePath string
	Type       string
	Key        string
	Value      string
}

func (e event) isBTNTouch() bool {
	return e.Key == "BTN_TOUCH"
}

func (e event) isEvABS() bool {
	return e.Type == "EV_ABS"
}

func (e event) isPositionY() bool {
	return e.isEvABS() && e.Key == "ABS_MT_POSITION_Y"
}

func (e event) isPositionX() bool {
	return e.isEvABS() && e.Key == "ABS_MT_POSITION_X"
}

func (e event) isBTNUp() bool {
	return e.isBTNTouch() && e.Value == "UP"
}

func (e event) isBTNDown() bool {
	return e.isBTNTouch() && e.Value == "DOWN"
}

func (e event) GetNumeric() (int, error) {
	i, err := strconv.ParseInt(e.Value, 16, 64)
	if err != nil {
		return 0, err
	}
	return int(i), nil
}

func parseGetEvent(input string) (events []Input) {
	lines := strings.Split(input, "\n")
	lines = trimDeviceDescriptors(lines)
	touchEvents := parseInputToEvent(lines)
	touches := getEventSlices(touchEvents)
	events = touchesToInputs(touches)
	events = insertSleeps(events)
	return
}

func touchesToInputs(events []eventSet) []Input {
	inputs := []Input{}
	for _, eventSet := range events {
		i, err := eventSet.ToInput()
		if err == nil {
			inputs = append(inputs, i)
		}
	}
	return inputs
}

type eventSet []event

func insertSleeps(inputs []Input) []Input {
	sleepingInputs := []Input{}
	for i, input := range inputs {
		if i != 0 {
			prev := sleepingInputs[len(sleepingInputs)-1].EndTime()
			curr := input.EndTime()
			var sleep SequenceSleep
			sleep.Duration = curr.Sub(prev)
			sleepingInputs = append(sleepingInputs, sleep)
		}
		sleepingInputs = append(sleepingInputs, input)
	}
	return sleepingInputs
}

// trawls through the list of events in a set.
// the returned Input is always a swipe, as android can automatically
// decide to treat a swipe as a tap if necessary.
// taps are made available to the end user should they be manually set
// but it's safer to just use swipes as there's no chance of accidentally
// treating a swipe as a tap when guessing at the duration and distance
// between the touch down and pick up loci
func (e eventSet) ToInput() (Input, error) {
	var (
		swipe          SequenceSwipe
		startx, starty int64
		xFound, yFound = false, false
		endx, endy     int64
	)
	var err error
	for i := 0; i < len(e); i++ {
		if xFound && yFound {
			break
		}
		if e[i].isPositionX() {
			xFound = true
			startx, err = strconv.ParseInt(e[i].Value, 16, 64)
			if err != nil {
				return nil, err
			}
		}
		if e[i].isPositionY() {
			yFound = true
			starty, err = strconv.ParseInt(e[i].Value, 16, 64)
			if err != nil {
				return nil, err
			}
		}

	}
	if !xFound || !yFound {
		return nil, ErrCoordinatesNotFound
	}
	xFound, yFound = false, false
	for i := len(e) - 1; i >= 0; i-- {
		if xFound && yFound {
			break
		}
		if e[i].isPositionX() {
			xFound = true
			endx, err = strconv.ParseInt(e[i].Value, 16, 64)
			if err != nil {
				return nil, err
			}
		}
		if e[i].isPositionY() {
			yFound = true
			endy, err = strconv.ParseInt(e[i].Value, 16, 64)
			if err != nil {
				return nil, err
			}
		}

	}
	swipe.X1 = int(startx)
	swipe.X2 = int(endx)
	swipe.Y1 = int(starty)
	swipe.Y2 = int(endy)
	swipe.Start = e[0].TimeStamp
	swipe.End = e[len(e)-1].TimeStamp
	return swipe, err
}

// Accepts a slice of events
// returns a slice of eventSets, where an eventSet is a group of events
// guaranteed to start with a DOWN event and end with an UP event,
// containing exactly one of each
func getEventSlices(events []event) []eventSet {
	eventSets := []eventSet{{}}
	current := 0
	foundDown := false
	for _, e := range events {
		if !foundDown {
			if e.isBTNDown() {
				foundDown = true
			} else {
				continue
			}
		}
		eventSets[current] = append(eventSets[current], e)
		if e.isBTNUp() {
			current++
			foundDown = false
			eventSets = append(eventSets, []event{})
		}
	}
	eventSets = eventSets[:len(eventSets)-1]
	return eventSets
}

func parseInputToEvent(input []string) []event {
	var e []event
	r := regexp.MustCompile(`\[\s*(\d+\.\d+)]\s*(.*):\s*(\w*)\s*(\w*)\s*(\w*)`)
	for _, line := range input {
		var l event
		timeStr := r.FindStringSubmatch(line)
		if len(timeStr) != 6 {
			continue
		}
		f, err := strconv.ParseFloat(timeStr[1], 32)
		if err != nil {
			continue
		}
		msec := int64(f * 1000)
		l.TimeStamp = time.UnixMilli(msec)
		l.DevicePath = timeStr[2]
		l.Type = timeStr[3]
		l.Key = timeStr[4]
		l.Value = timeStr[5]
		e = append(e, l)
	}

	return e
}

func trimDeviceDescriptors(input []string) []string {
	start := 0
	for i, line := range input {
		if strings.Contains(line, "DOWN") {
			start = i
			break
		}
	}
	for i := range input {
		input[i] = strings.TrimSpace(input[i])
	}
	return input[start : len(input)-1]
}
