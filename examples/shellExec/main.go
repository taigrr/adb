package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/taigrr/adb"
)

func main() {
	command := "ls"
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
		stdout, stderr, errcode, err := dev.Shell(ctx, command)
		_ = stderr
		_ = errcode
		switch {
		case err == nil:
		case errors.Is(err, adb.ErrUnspecified):
		default:
			fmt.Printf("Error running shell command `%s` on dev `%s`: %v\n", command, dev.SerialNo, err)
			continue
		}
		fmt.Printf("%s:\n\n%s\n", dev.SerialNo, stdout)
	}
}
