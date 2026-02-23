package main

import (
	"context"
	"fmt"

	"github.com/taigrr/adb"
)

func main() {
	ctx := context.TODO()
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
		err := dev.Reboot(ctx)
		if err != nil {
			// handle error here
			fmt.Printf("Error: %v\n", err)
		}
	}
}
