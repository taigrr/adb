// Command injection demonstrates enumerating devices and rebooting each
// authorized one with the adb package.
package main

import (
	"context"
	"fmt"

	"github.com/taigrr/adb"
)

func main() {
	ctx := context.TODO()
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
		if err := dev.Reboot(ctx); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
}
