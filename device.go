package adb

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

// Transport describes how a device is attached to the host.
type Transport int

const (
	// UnknownTransport is the zero value, used by an uninitialized Device.
	UnknownTransport Transport = iota
	// USB indicates a device attached over USB.
	USB
	// Network indicates a device attached over TCP/IP.
	Network
)

// String returns a human-readable name for the transport.
func (t Transport) String() string {
	switch t {
	case Network:
		return "network"
	case USB:
		return "usb"
	default:
		return "unknown"
	}
}

// Device is a handle to a single adb device. Values are immutable; obtain them
// from [Client.Devices] or [Client.Connect]. The zero value is not usable.
type Device struct {
	client     *Client
	serial     string
	transport  Transport
	addr       netip.AddrPort
	hasAddr    bool
	authorized bool
}

// Serial returns the adb serial that addresses the device. For network devices
// this is the host:port string.
func (d Device) Serial() string { return d.serial }

// Transport reports whether the device is attached over USB or the network.
func (d Device) Transport() Transport { return d.transport }

// Addr returns the device's network address. The boolean is false for USB
// devices or when no address is known.
func (d Device) Addr() (netip.AddrPort, bool) { return d.addr, d.hasAddr }

// Authorized reports whether the device has authorized debugging.
func (d Device) Authorized() bool { return d.authorized }

// Devices returns the devices known to adb, equivalent to `adb devices`.
//
// Returned devices may not be connected/authorized; check [Device.Authorized]
// before use.
func (c *Client) Devices(ctx context.Context) ([]Device, error) {
	res, err := c.run(ctx, "devices")
	if err != nil {
		return nil, err
	}
	return c.parseDevices(res.StdoutString()), nil
}

// Device returns a handle to a device addressed by the given adb serial
// (a USB serial or a network host:port). No I/O is performed and the device is
// not verified to exist; the returned handle is assumed authorized. Use
// [Client.Devices] to discover serials and authorization state.
func (c *Client) Device(serial string) Device {
	d := Device{
		client:     c,
		serial:     serial,
		transport:  USB,
		authorized: true,
	}
	if addr, err := netip.ParseAddrPort(serial); err == nil {
		d.transport = Network
		d.addr = addr
		d.hasAddr = true
	}
	return d
}

func (c *Client) parseDevices(stdout string) []Device {
	devs := []Device{}
	for line := range strings.SplitSeq(stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" ||
			strings.HasPrefix(trimmed, "List of devices") ||
			strings.HasPrefix(trimmed, "*") { // daemon status messages
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		// The state may be multiple words (e.g. "no permissions (...)"); only
		// the exact "device" state is authorized.
		d := Device{
			client:     c,
			serial:     fields[0],
			transport:  USB,
			authorized: fields[1] == "device",
		}
		if addr, err := netip.ParseAddrPort(fields[0]); err == nil {
			d.transport = Network
			d.addr = addr
			d.hasAddr = true
		}
		devs = append(devs, d)
	}
	return devs
}

// Connect attaches to a network device at addr ("host" or "host:port"). When
// the port is omitted it defaults to 5555. Hostnames are accepted and passed
// through to adb. It is equivalent to `adb connect`.
func (c *Client) Connect(ctx context.Context, addr string) (Device, error) {
	target, err := normalizeAddr(addr)
	if err != nil {
		return Device{}, err
	}
	res, err := c.run(ctx, "connect", target)
	if err != nil {
		return Device{}, err
	}
	// `adb connect` reports failures on stdout while exiting 0. Map known
	// causes to their sentinels, otherwise a generic failure.
	if !strings.Contains(res.StdoutString(), "connected to ") {
		cause := filterStderr(res.StdoutString())
		if cause == nil {
			cause = ErrCommandFailed
		}
		return Device{}, &CommandError{
			Args:   []string{"connect", target},
			Code:   res.Code,
			Stderr: res.StderrString(),
			Err:    cause,
		}
	}
	dev := Device{
		client:     c,
		serial:     target,
		transport:  Network,
		authorized: true,
	}
	if ap, err := netip.ParseAddrPort(target); err == nil {
		dev.addr = ap
		dev.hasAddr = true
	}
	return dev, nil
}

// Disconnect drops the connection to a network device, equivalent to
// `adb disconnect`. It returns [ErrNotNetworkDevice] for USB devices.
func (d Device) Disconnect(ctx context.Context) error {
	if d.transport != Network {
		return ErrNotNetworkDevice
	}
	_, err := d.client.run(ctx, "disconnect", d.serial)
	return err
}

// Pair pairs with a device for secure wireless debugging (Android 11+),
// equivalent to `adb pair HOST[:PORT] [CODE]`. Pass an empty code for devices
// that do not require one.
func (c *Client) Pair(ctx context.Context, addr, code string) error {
	args := []string{"pair", addr}
	if code != "" {
		args = append(args, code)
	}
	res, err := c.run(ctx, args...)
	if err != nil {
		return err
	}
	if !strings.Contains(res.StdoutString(), "Successfully paired") &&
		!strings.Contains(res.StderrString(), "Successfully paired") {
		return &CommandError{Args: args, Code: res.Code, Stderr: res.StderrString(), Err: ErrCommandFailed}
	}
	return nil
}

// StartServer ensures the adb server is running, equivalent to
// `adb start-server`.
func (c *Client) StartServer(ctx context.Context) error {
	_, err := c.run(ctx, "start-server")
	return err
}

// KillServer terminates the adb server, equivalent to `adb kill-server`.
//
// This invalidates existing [Device] handles until they are reconnected.
func (c *Client) KillServer(ctx context.Context) error {
	_, err := c.run(ctx, "kill-server")
	return err
}

// normalizeAddr accepts "host", "host:port", "ip", or "ip:port" and returns a
// canonical address, defaulting the port to 5555. Hostnames (non-IP) are
// accepted since adb resolves them itself.
func normalizeAddr(addr string) (string, error) {
	if addr == "" {
		return "", fmt.Errorf("invalid device address %q", addr)
	}
	// IP or IP:port.
	if ap, err := netip.ParseAddrPort(addr); err == nil {
		if ap.Port() == 0 {
			return "", fmt.Errorf("invalid device address %q", addr)
		}
		return ap.String(), nil
	}
	if ip, err := netip.ParseAddr(addr); err == nil {
		return netip.AddrPortFrom(ip, 5555).String(), nil
	}
	// Hostname, optionally with a port.
	if host, port, err := net.SplitHostPort(addr); err == nil {
		if host == "" || port == "" {
			return "", fmt.Errorf("invalid device address %q", addr)
		}
		if n, err := strconv.ParseUint(port, 10, 16); err != nil || n == 0 {
			return "", fmt.Errorf("invalid device address %q", addr)
		}
		return addr, nil
	}
	if strings.Contains(addr, ":") {
		return "", fmt.Errorf("invalid device address %q", addr)
	}
	return net.JoinHostPort(addr, "5555"), nil
}
