package systemd

import (
	"context"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/rs/zerolog"

	"helvilette/pkg/log"
)

// Watcher watches for systemd unit state changes
type Watcher struct {
	client     *Client
	watchUnits map[string]bool
	logger     zerolog.Logger
}

// NewWatcher creates a new systemd unit watcher
func NewWatcher(client *Client, units []string) *Watcher {
	watchMap := make(map[string]bool)
	for _, u := range units {
		watchMap[u] = true
	}

	return &Watcher{
		client:     client,
		watchUnits: watchMap,
		logger:     log.WithComponent("systemd-watcher"),
	}
}

// Watch starts watching for unit state changes and returns events on a channel
// If units is empty, watches all units
func (w *Watcher) Watch(ctx context.Context) (<-chan UnitEvent, error) {
	eventChan := make(chan UnitEvent, 100)

	// Subscribe to systemd signals
	if err := w.client.conn.Subscribe(); err != nil {
		return nil, err
	}

	// Set up the signal channel
	signalChan, errChan := w.client.conn.SubscribeUnitsCustom(
		time.Second, // interval for checking
		0,           // buffer size
		func(u1, u2 *dbus.UnitStatus) bool {
			// Return true if unit changed
			if u1 == nil || u2 == nil {
				return true
			}
			return u1.ActiveState != u2.ActiveState || u1.SubState != u2.SubState
		},
		func(name string) bool {
			// Filter: if watchUnits is empty, watch all; otherwise filter
			if len(w.watchUnits) == 0 {
				return true
			}
			return w.watchUnits[name]
		},
	)

	go func() {
		defer close(eventChan)

		for {
			select {
			case <-ctx.Done():
				w.logger.Info().Msg("watcher stopping due to context cancellation")
				return

			case changes := <-signalChan:
				for name, status := range changes {
					if status == nil {
						continue
					}

					event := UnitEvent{
						Unit: UnitState{
							Name:        name,
							LoadState:   status.LoadState,
							ActiveState: status.ActiveState,
							SubState:    status.SubState,
							Timestamp:   time.Now(),
						},
						EventType: determineEventType(status.ActiveState, status.SubState),
					}

					w.logger.Info().
						Str("unit", name).
						Str("active_state", status.ActiveState).
						Str("sub_state", status.SubState).
						Str("event_type", event.EventType).
						Msg("unit state changed")

					select {
					case eventChan <- event:
					default:
						w.logger.Warn().Str("unit", name).Msg("event channel full, dropping event")
					}
				}

			case err := <-errChan:
				w.logger.Error().Err(err).Msg("error from systemd subscription")
			}
		}
	}()

	return eventChan, nil
}

// determineEventType infers event type from state
func determineEventType(activeState, subState string) string {
	switch activeState {
	case ActiveStateActive:
		if subState == SubStateRunning {
			return "started"
		}
		return "active"
	case ActiveStateInactive:
		return "stopped"
	case ActiveStateFailed:
		return "failed"
	case ActiveStateActivating:
		return "starting"
	case ActiveStateDeactivating:
		return "stopping"
	case ActiveStateReloading:
		return "reloading"
	default:
		return "unknown"
	}
}
