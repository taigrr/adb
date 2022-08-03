package adb

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
)

type Serial string

type Connection int

const (
	USB Connection = iota
	Network
)

// Create a Device with Connect() or a slice with Devices()
//
// Device contains the information necessary to connect to and
// communicate with a device
type Device struct {
	IsAuthorized bool
	SerialNo     Serial
	ConnType     Connection
	IP           net.IPAddr
	Port         uint
	FileHandle   string // TODO change this to a discrete type
}

// Provides a connection string for Connect()
type ConnOptions struct {
	Address  net.IPAddr
	Port     uint
	SerialNo Serial
}

// Connect to a device by IP:port.
//
// This will return a Device struct, which can be used to call other methods.
// If the connection fails or cannot complete on time, Connect will return an error.
func Connect(ctx context.Context, opts ConnOptions) (Device, error) {
	if opts.Port == 0 {
		opts.Port = 5555
	}
	return Device{}, nil
}

func (d Device) ConnString() string {
	if d.Port == 0 {
		d.Port = 5555
	}
	return d.IP.String() + ":" + strconv.Itoa(int(d.Port))
}

// Connect to a previously discovered device.
//
// This function is helpful when connecting to a device found from the Devices call
// or when reconnecting to a previously connected device.
func (d Device) Reconnect(ctx context.Context) (Device, error) {
	if d.ConnType == USB {
		return d, ErrConnUSB
	}
	cmd := []string{"connect", d.ConnString()}
	stdout, stderr, errcode, err := execute(ctx, cmd)
	if err != nil {
		return d, err
	}
	if errcode != 0 {
		return d, ErrUnspecified
	}
	_, _ = stdout, stderr
	// TODO capture and store serial number into d before returning
	return d, nil
}

// Equivalent to running `adb devices`.
//
// This function returns a list of discovered devices, but note that they may not be connected.
// It is recommended to call IsConnected() against the device you're interested in using and connect
// if not already connected before proceeding.
func Devices(ctx context.Context) ([]Device, error) {
	cmd := []string{"devices"}
	stdout, _, errcode, err := execute(ctx, cmd)
	devs := []Device{}
	if err != nil {
		return devs, err
	}
	if errcode != 0 {
		return devs, ErrUnspecified
	}

	return parseDevices(stdout)
}

func parseDevices(stdout string) ([]Device, error) {
	devs := []Device{}
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		words := strings.Fields(line)
		if len(words) != 2 {
			continue
		}
		d := Device{
			SerialNo:     Serial(words[0]),
			IsAuthorized: words[1] == "device",
		}
		devs = append(devs, d)
	}

	return devs, nil
}

// Disconnect from a device.
//
// If a device is already disconnected or otherwise not found, returns an error.
func (d Device) Disconnect(ctx context.Context) error {
	return nil
}

// Kill the ADB Server
//
// Warning, this function call may cause inconsostency if not used properly.
// Killing the ADB server shouldn't ever technically be necessary, but if you do
// decide to use this function. note that it may invalidate all existing device structs.
// Older versions of Android don't play nicely with kill-server, and some may
// refuse following connection attempts if you don't disconnect from them before
// calling this function.
func KillServer(ctx context.Context) error {
	return nil
}

// Push a file to a Device.
//
// Returns an error if src does not exist or there is an error copying the file.
func (d Device) Push(ctx context.Context, src, dest string) error {
	_, err := os.Stat(src)
	if err != nil {
		return err
	}
	stdout, stderr, errcode, err := execute(ctx, []string{"push", src, dest})
	if err != nil {
		return err
	}
	if errcode != 0 {
		return ErrUnspecified
	}
	_, _ = stdout, stderr
	// TODO check the return strings of the output to determine if the file copy succeeded
	return nil
}

// Pulls a file from a Device
//
// Returns an error if src does not exist, or if dest already exists or cannot be created
func (d Device) Pull(ctx context.Context, src, dest string) error {
	_, err := os.Stat(src)
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	stdout, stderr, errcode, err := execute(ctx, []string{"pull", src, dest})
	if err != nil {
		return err
	}
	if errcode != 0 {
		return ErrUnspecified
	}
	_, _ = stdout, stderr
	// TODO check the return strings of the output to determine if the file copy succeeded
	return nil
}

// Attempts to reboot the device
//
// Once the device reboots, you must manually reconnect.
// Returns an error if the device cannot be contacted
func (d Device) Reboot(ctx context.Context) error {
	stdout, stderr, errcode, err := execute(ctx, []string{"reboot"})
	if err != nil {
		return err
	}
	if errcode != 0 {
		return ErrUnspecified
	}
	_, _ = stdout, stderr
	// TODO check the return strings of the output to determine if the file copy succeeded
	return nil
}

// Attempt to relaunch adb as root on the Device.
//
// Note, this may not be possible on most devices.
// Returns an error if it can't be done.
// The device connection will stay established.
// Once adb is relaunched as root, it will stay root until rebooted.
// returns true if the device successfully relaunched as root
func (d Device) Root(ctx context.Context) (success bool, err error) {
	stdout, stderr, errcode, err := execute(ctx, []string{"root"})
	if err != nil {
		return false, err
	}
	if errcode != 0 {
		return false, ErrUnspecified
	}
	_, _ = stdout, stderr
	// TODO check the return strings of the output to determine if the file copy succeeded
	return true, nil
}
