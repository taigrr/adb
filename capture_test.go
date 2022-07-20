package adb

import (
	_ "embed"
	"strings"
	"testing"
)

//go:embed tests/multitouch.log
var multitouch string

//go:embed tests/tablet.log
var tablet string

//go:embed tests/pixel.log
var pixel string

func TestParseInputToEvent(t *testing.T) {
	testCases := []struct {
		name     string
		testFile string
	}{
		{"pixel", pixel},
		{"multitouch", multitouch},
		{"tablet", tablet},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lines := strings.Split(tc.testFile, "\n")
			aD := "add device"
			if !strings.Contains(lines[0], aD) {
				t.Errorf("Line 0 expected to contain `%s`, but we got: %s", aD, lines[0])
			}
			lines = trimDeviceDescriptors(lines)
			if strings.Contains(lines[0], aD) {
				t.Errorf("Line 0 expected to not contain `%s`, but we got: %s", aD, lines[0])
			}
			parseInputToEvent(lines)
		})
	}
}

func TestTrimDeviceDescriptors(t *testing.T) {
	testCases := []struct {
		name     string
		testFile string
	}{
		{"pixel", pixel},
		{"multitouch", multitouch},
		{"tablet", tablet},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lines := strings.Split(tc.testFile, "\n")
			aD := "add device"
			if !strings.Contains(lines[0], aD) {
				t.Errorf("Line 0 expected to contain `%s`, but we got: %s", aD, lines[0])
			}
			lines = trimDeviceDescriptors(lines)
			if strings.Contains(lines[0], aD) {
				t.Errorf("Line 0 expected to not contain `%s`, but we got: %s", aD, lines[0])
			}
		})
	}
}
