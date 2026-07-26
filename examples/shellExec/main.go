// Command shellExec demonstrates running a shell command on every authorized
// device with the adb package.
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
		res, err := dev.Shell(ctx, "ls")
		if err != nil {
			fmt.Printf("Error running shell command on dev `%s`: %v\n", dev.Serial(), err)
			continue
		}
		fmt.Printf("%s:\n\n%s\n", dev.Serial(), res.StdoutString())
	}
}
