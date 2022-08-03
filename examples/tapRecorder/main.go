package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/taigrr/adb"
)

var (
	command string
	file    string
)

func main() {
	flag.StringVar(&command, "command", "rec", "rec or play")
	flag.StringVar(&file, "file", "taps.json", "Name of the file to save taps to or to play from")
	flag.Parse()
	sigChan := make(chan os.Signal)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-sigChan
		cancel()
	}()
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	devs, err := adb.Devices(ctx)
	if err != nil {
		fmt.Printf("Error enumerating devices: %v\n", err)
		return
	}
	for _, dev := range devs {
		if !dev.IsAuthorized {
			fmt.Printf("Dev `%s` is not authorized, authorize it to continue.\n", dev.SerialNo)
			continue
		}
		//w, h, err := dev.GetScreenResolution(ctx)
		//if err != nil {
		//	// handle error here
		//	fmt.Printf("Error: %v\n", err)
		//}
		switch command {
		case "rec":
			fmt.Println("Recording taps now. Hit ctrl+c to stop.")
			t, err := dev.CaptureSequence(ctx)
			if err != nil {
				fmt.Printf("Error capturing sequence: %v\n", err)
				return
			}
			b, _ := json.Marshal(t)
			f, err := os.Create(file)
			if err != nil {
				fmt.Printf("Error creating tap file %s: %v", file, err)
				return
			}
			defer f.Close()
			f.Write(b)
		case "play":
			fmt.Println("Replaying taps now. Hit ctrl+c to stop.")
			f, err := os.Open(file)
			if err != nil {
				fmt.Printf("Error opening tap file %s: %v", file, err)
				return
			}
			defer f.Close()
			var j map[string]interface{}
			var t adb.TapSequence
			var b bytes.Buffer
			b.ReadFrom(f)
			err = json.Unmarshal(b.Bytes(), &j)
			if err != nil {
				fmt.Printf("Error parsing tap file %s: %v", file, err)
				return
			}
			if events, ok := j["Events"]; ok {
				if sliceEvent, ok := events.([]interface{}); ok {
					for _, e := range sliceEvent {
						if mapEvent, ok := e.(map[string]interface{}); ok {
							if eventType, ok := mapEvent["Type"]; ok {
								if et, ok := eventType.(float64); ok {
									switch int(et) {
									case int(adb.SeqSleep):
										t.Events = append(t.Events, adb.SequenceSleep{})
									case int(adb.SeqSwipe):
										t.Events = append(t.Events, adb.SequenceSwipe{})
									case int(adb.SeqTap):
										t.Events = append(t.Events, adb.SequenceTap{})
									}
								} else {
									fmt.Printf("Could not parse %v (%T) into JSON! 1\n", eventType, eventType)
								}
							} else {
								fmt.Println("Could not parse JSON! 2")
							}
						} else {
							fmt.Println("Could not parse JSON! 3")
						}
					}
				} else {
					fmt.Println("Could not parse JSON! 4")
				}
			} else {
				fmt.Println("Could not parse JSON! 5")
			}
			dev.ReplayTapSequence(ctx, t)
			err = json.Unmarshal(b.Bytes(), &t)
			if err != nil {
				fmt.Printf("struct: %v\n",t)
				fmt.Printf("bytes: %v\n",b.String())
				fmt.Printf("Error parsing tap file %s: %v", file, err)
				return
			}

		default:
		}
	}
}
