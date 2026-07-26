// Package adb provides idiomatic, context-aware Go bindings for the Android
// Debug Bridge (adb) command-line tool. The adb binary must be installed and
// available on PATH.
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

// Serial is the unique identifier adb uses to address a device. For USB
// devices this is the hardware serial; for network devices it is the
// host:port string.
type Serial string

// Connection describes how a device is attached to the host.
type Connection int

const (
	// USB indicates a device attached over USB.
	USB Connection = iota
	// Network indicates a device attached over TCP/IP.
	Network
)

// Device contains the information necessary to connect to and communicate with
// a device. Create one with [Connect], or obtain a slice with [Devices].
type Device struct {
	IsAuthorized bool
	SerialNo     Serial
	ConnType     Connection
	IP           net.IPAddr
	Port         uint
	FileHandle   string // TODO change this to a discrete type
}

// ConnOptions provides the connection parameters used by [Connect].
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

	device.applyConnectedDevice(stdout)

	return device, nil
}

// ConnString returns the host:port string used to address a network device,
// defaulting the port to 5555 when unset.
func (d Device) ConnString() string {
	port := d.Port
	if port == 0 {
		port = 5555
	}
	return net.JoinHostPort(d.IP.String(), strconv.Itoa(int(port)))
}

// Reconnect re-establishes a connection to a previously discovered device.
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
	_, _ = stderr, stdout
	d.applyConnectedDevice(stdout)
	return d, nil
}

// Devices returns the list of devices known to adb, equivalent to running
// `adb devices`.
//
// Note that the returned devices may not be connected. It is recommended to
// check the device you intend to use and connect if necessary before
// proceeding.
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
	for line := range strings.SplitSeq(stdout, "\n") {
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

func (d *Device) applyConnectedDevice(stdout string) {
	connectedDevice, err := parseConnectedDevice(stdout)
	if err != nil {
		return
	}
	if connectedDevice.SerialNo != "" {
		d.SerialNo = connectedDevice.SerialNo
	}
	d.ConnType = connectedDevice.ConnType
	d.IP = connectedDevice.IP
	d.Port = connectedDevice.Port
	d.IsAuthorized = connectedDevice.IsAuthorized
}

func parseConnectedDevice(stdout string) (Device, error) {
	for line := range strings.SplitSeq(stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if serial, ok := strings.CutPrefix(trimmed, "connected to "); ok {
			return parseNetworkDevice(serial)
		}
		if serial, ok := strings.CutPrefix(trimmed, "already connected to "); ok {
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
