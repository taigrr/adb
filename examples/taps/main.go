package main

import (
	"context"
	"fmt"
	"time"

	"github.com/taigrr/adb"
)

var command string

func init() {
	// TODO  allow for any input to be used as the command
	command = "ls"
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
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
		fmt.Printf("Begin tapping on device %s now...\n", dev.SerialNo)
		t, err := dev.CaptureSequence(ctx)
		if err != nil {
			fmt.Printf("Error capturing sequence: %v\n", err)
			return
		}
		fmt.Println("Sequence captured, replaying now...")
		dev.ReplayTapSequence(context.TODO(), t)
	}
}
