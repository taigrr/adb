package adb

import (
	"context"
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

type SequenceTap struct {
	X int
	Y int
}

func (s SequenceTap) Play(d Device, ctx context.Context) error {
	return d.Tap(ctx, s.X, s.Y)
}

func (s SequenceTap) Length() time.Duration {
	return 0
}

type SequenceSwipe struct {
	X1       int
	Y1       int
	X2       int
	Y2       int
	Duration time.Duration
}

func (s SequenceSwipe) Play(d Device, ctx context.Context) error {
	return d.Swipe(ctx, s.X1, s.Y1, s.X2, s.Y2, s.Duration)
}

func (s SequenceSwipe) Length() time.Duration {
	return s.Duration
}

type Input interface {
	Play(d Device, ctx context.Context) error
	Length() time.Duration
}

type TapSequence struct {
	Events []Input
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

//CaptureSequence allows you to capture and replay screen taps and swipes.
//
// ctx, cancelFunc := context.WithCancel(context.TODO())
//
// go dev.CaptureSequence(ctx)
// time.Sleep(time.Second * 30)
// cancelFunc()
func (d Device) CaptureSequence(ctx context.Context) (t TapSequence, err error) {
	// this command will never finish, and always returns error code 130 if successful
	stdout, _, errCode, err := execute(ctx, []string{"shell", "getevent", "-tl"})
	if errCode != 130 {
		log.Printf("Expected error code 130, but got %d\n", errCode)
	}
	if stdout == "" {
		return TapSequence{}, ErrStdoutEmpty
	}
	t.Events = parseGetEvent(stdout)
	return TapSequence{}, nil
}

type Event struct {
	TimeStamp  time.Time
	DevicePath string
	Type       string
	Key        string
	Value      string
}

func parseGetEvent(input string) (events []Input) {
	lines := strings.Split(input, "\n")
	lines = trimDeviceDescriptors(lines)
	// Trim off the beginning with device descriptors
	return
}

func parseInputToEvent(input []string) []Event {
	var e []Event
	r := regexp.MustCompile(`\[\W*(\d+\.\d+)]`)
	for _, line := range input {
		var l Event
		timeStr := r.FindStringSubmatch(line)
		if len(timeStr) != 2 {
			continue
		}
		f, err := strconv.ParseFloat(timeStr[1], 32)
		if err != nil {
			continue
		}
		msec := int64(f * 1000)
		l.TimeStamp = time.UnixMilli(msec)
		e = append(e, l)
	}
	// fmt.Println(e[0].TimeStamp.Sub(e[len(e)-1].TimeStamp))

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
	return input[start:]
}
