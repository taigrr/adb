package adb

import (
	"context"
	"net"
	"sync"
)

type Serial string

type Connection int

const (
	USB Connection = iota
	Network
)

type Device struct {
	IsConnected bool
	SerialNo    Serial
	ConnType    Connection
	IP          net.IPAddr
	FileHandle  string // TODO change this to a discrete type
	Lock        sync.Mutex
}

type ConnOptions struct {
	Address  net.IPAddr
	SerialNo Serial
}

// Connect to a device by serial number or IP.
//
// This will return a Device struct, which can be used to call other methods.
// If the connection fails or cannot complete on time, Connect will return an error.
func Connect(ctx context.Context, opts ConnOptions) (Device, error) {
	return Device{}, nil
}

// Connect to a previously discovered device.
//
// This function is helpful when connecting to a device found from the Devices call
// or when reconnecting to a previously connected device.
func (d Device) Connect(ctx context.Context) (Device, error) {
	return d, nil
}

// Equivalent to running `adb devices`.
//
// This function returns a list of discovered devices, but note that they may not be connected.
// It is recommended to call IsConnected() against the device you're interested in using and connect
// if not already connected before proceeding.
func Devices(ctx context.Context) ([]Device, error) {
	return []Device{}, nil
}

// Disconnect from a device.
//
// If a device is already disconnected or otherwise not found, returns an error.
func (d Device) Disconnect(ctx context.Context) error {
	return nil
}
