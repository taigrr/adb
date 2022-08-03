package main

import (
	"context"
	"fmt"

	"github.com/taigrr/adb"
)

var command string

func init() {
	// TODO  allow for any input to be used as the command
	command = "ls"
}

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
		w, h, err := dev.GetScreenResolution(ctx)
		if err != nil {
			// handle error here
			fmt.Printf("Error: %v\n", err)
		}
		fmt.Printf("%s screen resolution: %dx%d\n", dev.SerialNo, w, h)
	}
}
