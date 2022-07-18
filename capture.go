package adb

import (
	"context"
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

// GetLength returns the length of all Input events inside of a given TapSequence
//
// This function is useful for devermining how long a context timeout should
// last when calling ReplayTapSequence
func (t TapSequence) GetLength() time.Duration {
	duration := time.Duration(0)
	for _, x := range t.Events {
		duration += x.Length()
	}
	return duration
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

func (d Device) CaptureSequence(ctx context.Context) (t TapSequence, err error) {
	return TapSequence{}, nil
}
