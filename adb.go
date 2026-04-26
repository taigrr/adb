package adb

import (
	"context"
	"errors"
	"fmt"
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
// TODO
func Connect(ctx context.Context, opts ConnOptions) (Device, error) {
	device := Device{
		ConnType: Network,
		IP:       opts.Address,
		Port:     opts.Port,
		SerialNo: opts.SerialNo,
	}
	if device.Port == 0 {
		device.Port = 5555
	}

	stdout, _, errcode, err := execute(ctx, []string{"connect", device.ConnString()})
	if err != nil {
		return Device{}, err
	}
	if errcode != 0 {
		return Device{}, ErrUnspecified
	}

	connectedDevice, parseErr := parseConnectedDevice(stdout)
	if parseErr == nil {
		if connectedDevice.SerialNo != "" {
			device.SerialNo = connectedDevice.SerialNo
		}
		device.ConnType = connectedDevice.ConnType
		device.IP = connectedDevice.IP
		device.Port = connectedDevice.Port
		device.IsAuthorized = connectedDevice.IsAuthorized
	}

	return device, nil
}

func (d Device) ConnString() string {
	port := d.Port
	if port == 0 {
		port = 5555
	}
	return net.JoinHostPort(d.IP.String(), strconv.Itoa(int(port)))
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

// TODO add support for connected network devices
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
		if networkDevice, err := parseNetworkDevice(words[0]); err == nil {
			d.ConnType = Network
			d.IP = networkDevice.IP
			d.Port = networkDevice.Port
		} else {
			d.ConnType = USB
		}
		devs = append(devs, d)
	}

	return devs, nil
}

func parseConnectedDevice(stdout string) (Device, error) {
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "connected to ") {
			serial := strings.TrimPrefix(trimmed, "connected to ")
			return parseNetworkDevice(serial)
		}
		if strings.HasPrefix(trimmed, "already connected to ") {
			serial := strings.TrimPrefix(trimmed, "already connected to ")
			return parseNetworkDevice(serial)
		}
	}
	return Device{}, fmt.Errorf("unable to parse connected device from %q", stdout)
}

func parseNetworkDevice(serial string) (Device, error) {
	host, portStr, err := net.SplitHostPort(serial)
	if err != nil {
		return Device{}, err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return Device{}, fmt.Errorf("invalid IP address %q", host)
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return Device{}, err
	}
	return Device{
		SerialNo:     Serial(serial),
		IsAuthorized: true,
		ConnType:     Network,
		IP:           net.IPAddr{IP: ip},
		Port:         uint(port),
	}, nil
}

// Disconnect from a device.
//
// If a device is already disconnected or otherwise not found, returns an error.
func (d Device) Disconnect(ctx context.Context) error {
	if d.ConnType != Network {
		return ErrConnUSB
	}
	_, _, _, err := execute(ctx, []string{"disconnect", d.ConnString()})
	return err
}

// KillServer kills the ADB server.
//
// Warning: this function call may cause inconsistency if not used properly.
// Killing the ADB server shouldn't ever technically be necessary, but if you do
// decide to use this function, note that it may invalidate all existing device structs.
// Older versions of Android don't play nicely with kill-server, and some may
// refuse following connection attempts if you don't disconnect from them before
// calling this function.
func KillServer(ctx context.Context) error {
	_, _, _, err := execute(ctx, []string{"kill-server"})
	return err
}

// Push a file to a Device.
//
// Returns an error if src does not exist or there is an error copying the file.
func (d Device) Push(ctx context.Context, src, dest string) error {
	_, err := os.Stat(src)
	if err != nil {
		return err
	}
	_, _, errcode, err := execute(ctx, []string{"-s", string(d.SerialNo), "push", src, dest})
	if err != nil {
		return err
	}
	if errcode != 0 {
		return ErrUnspecified
	}
	return nil
}

// Pull a file from a Device.
//
// Returns an error if dest already exists or the file cannot be pulled.
func (d Device) Pull(ctx context.Context, src, dest string) error {
	_, err := os.Stat(dest)
	if err == nil {
		return ErrDestExists
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, _, errcode, err := execute(ctx, []string{"-s", string(d.SerialNo), "pull", src, dest})
	if err != nil {
		return err
	}
	if errcode != 0 {
		return ErrUnspecified
	}
	return nil
}

// Reboot attempts to reboot the device.
//
// Once the device reboots, you must manually reconnect.
// Returns an error if the device cannot be contacted.
func (d Device) Reboot(ctx context.Context) error {
	_, _, errcode, err := execute(ctx, []string{"-s", string(d.SerialNo), "reboot"})
	if err != nil {
		return err
	}
	if errcode != 0 {
		return ErrUnspecified
	}
	return nil
}

// Root attempts to relaunch adb as root on the Device.
//
// Note, this may not be possible on most devices.
// Returns an error if it can't be done.
// The device connection will stay established.
// Once adb is relaunched as root, it will stay root until rebooted.
// Returns true if the device successfully relaunched as root.
func (d Device) Root(ctx context.Context) (success bool, err error) {
	stdout, _, errcode, err := execute(ctx, []string{"-s", string(d.SerialNo), "root"})
	if err != nil {
		return false, err
	}
	if errcode != 0 {
		return false, ErrUnspecified
	}
	if strings.Contains(stdout, "adbd is already running as root") ||
		strings.Contains(stdout, "restarting adbd as root") {
		return true, nil
	}
	return false, nil
}
