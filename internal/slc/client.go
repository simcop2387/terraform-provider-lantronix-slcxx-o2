package slc

import (
	"context"
	"fmt"

	"github.com/simcop2387/terraform-provider-lantronix-slcxx-o2/internal/transport"
)

// Client is the typed Device Port API over a Transport. All reads are scoped
// to a single managed port (#9); writes are batched and always followed by a
// readback whose result is authoritative.
type Client struct {
	tr transport.Transport
}

func NewClient(tr transport.Transport) *Client { return &Client{tr: tr} }

// GetSerial reads the serial settings of one device port.
func (c *Client) GetSerial(ctx context.Context, number int) (Serial, error) {
	out, err := c.tr.Read(ctx, ShowDataCmd(number))
	if err != nil {
		return Serial{}, fmt.Errorf("read port %d data: %w", number, err)
	}
	return ParseSerial(out)
}

// GetAccess reads the network access settings (ip + modem idle timeout).
func (c *Client) GetAccess(ctx context.Context, number int) (Access, error) {
	out, err := c.tr.Read(ctx, ShowIPCmd(number))
	if err != nil {
		return Access{}, fmt.Errorf("read port %d ip: %w", number, err)
	}
	a, err := ParseAccess(out)
	if err != nil {
		return a, err
	}
	modem, err := c.tr.Read(ctx, ShowModemCmd(number))
	if err != nil {
		return a, fmt.Errorf("read port %d modem: %w", number, err)
	}
	a.IdleTimeout, err = ParseIdleTimeout(modem)
	return a, err
}

// ApplySerial issues one batched set for the given parameters and returns the
// readback.
func (c *Client) ApplySerial(ctx context.Context, number int, want Serial) (Serial, error) {
	if err := c.set(ctx, SetCmd(number, SetSerialParams(want))); err != nil {
		return Serial{}, err
	}
	return c.GetSerial(ctx, number)
}

// ApplyAccess issues one batched access set and returns the readback.
func (c *Client) ApplyAccess(ctx context.Context, number int, want Access) (Access, error) {
	if err := c.set(ctx, SetCmd(number, SetAccessParams(want))); err != nil {
		return Access{}, err
	}
	return c.GetAccess(ctx, number)
}

// DisableAccess closes all services on a port (port_access destroy).
func (c *Client) DisableAccess(ctx context.Context, number int) error {
	return c.set(ctx, DisableAccessCmd(number))
}

func (c *Client) set(ctx context.Context, cmd string) error {
	out, err := c.tr.Run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("set failed: %w", err)
	}
	return CheckSuccess(out)
}
