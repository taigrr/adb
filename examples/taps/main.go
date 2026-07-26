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
	client, err := adb.New()
	if err != nil {
		fmt.Printf("Error creating adb client: %v\n", err)
		return
	}
	devs, err := client.Devices(ctx)
	if err != nil {
		fmt.Printf("Error enumerating devices: %v\n", err)
		return
	}
	for _, dev := range devs {
		if !dev.Authorized() {
			fmt.Printf("Dev `%s` is not authorized, authorize it to continue.\n", dev.Serial())
			continue
		}
		fmt.Printf("Begin tapping on device %s now...\n", dev.Serial())
		seq, err := dev.Record(ctx)
		if err != nil {
			fmt.Printf("Error capturing sequence: %v\n", err)
			return
		}
		fmt.Println("Sequence captured, replaying now...")
		if err := dev.Replay(context.TODO(), seq); err != nil {
			fmt.Printf("Error replaying sequence: %v\n", err)
			return
		}
	}
}
