package systemd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/rs/zerolog"

	"helvilette/pkg/log"
)

// Client provides access to systemd via D-Bus
type Client struct {
	conn   *dbus.Conn
	logger zerolog.Logger
}

// NewClient creates a new systemd D-Bus client
func NewClient() (*Client, error) {
	conn, err := dbus.NewWithContext(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to systemd D-Bus: %w", err)
	}

	return &Client{
		conn:   conn,
		logger: log.WithComponent("systemd"),
	}, nil
}

// Close closes the D-Bus connection
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// GetUnitState returns the current state of a unit
func (c *Client) GetUnitState(name string) (*UnitState, error) {
	units, err := c.conn.ListUnitsByNamesContext(context.Background(), []string{name})
	if err != nil {
		return nil, fmt.Errorf("failed to get unit %s: %w", name, err)
	}

	if len(units) == 0 {
		return nil, fmt.Errorf("unit %s not found", name)
	}

	state := toUnitState(units[0], time.Now())
	return &state, nil
}

// ListUnits returns all loaded units
func (c *Client) ListUnits() ([]UnitState, error) {
	units, err := c.conn.ListUnitsContext(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to list units: %w", err)
	}

	result := make([]UnitState, len(units))
	now := time.Now()
	for i, u := range units {
		result[i] = toUnitState(u, now)
	}
	return result, nil
}

// ListServiceUnits returns only .service units
func (c *Client) ListServiceUnits() ([]UnitState, error) {
	units, err := c.ListUnits()
	if err != nil {
		return nil, err
	}

	var services []UnitState
	for _, u := range units {
		if strings.HasSuffix(u.Name, ".service") {
			services = append(services, u)
		}
	}
	return services, nil
}

// IsActive checks if a unit is active
func (c *Client) IsActive(name string) (bool, error) {
	state, err := c.GetUnitState(name)
	if err != nil {
		return false, err
	}
	return state.ActiveState == ActiveStateActive, nil
}

func toUnitState(u dbus.UnitStatus, observedAt time.Time) UnitState {
	return UnitState{
		Name:        u.Name,
		Description: u.Description,
		LoadState:   u.LoadState,
		ActiveState: u.ActiveState,
		SubState:    u.SubState,
		Timestamp:   observedAt,
	}
}
