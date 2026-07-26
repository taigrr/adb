// Command taps demonstrates capturing a touch sequence from a device and
// replaying it with the adb package.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/taigrr/adb"
)

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
		fmt.Printf("Begin tapping on device %s now...\n", dev.SerialNo)
		t, err := dev.CaptureSequence(ctx)
		if err != nil {
			fmt.Printf("Error capturing sequence: %v\n", err)
			return
		}
		fmt.Println("Sequence captured, replaying now...")
		if err := dev.ReplayTapSequence(context.TODO(), t); err != nil {
			fmt.Printf("Error replaying sequence: %v\n", err)
			return
		}
	}
}
