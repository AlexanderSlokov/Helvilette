package systemd

import "time"

// UnitState represents the current state of a systemd unit
type UnitState struct {
	Name        string    `json:"name"`         // e.g., "nginx.service"
	Description string    `json:"description"`  // Unit description
	LoadState   string    `json:"load_state"`   // loaded, not-found, masked, error
	ActiveState string    `json:"active_state"` // active, inactive, failed, activating, deactivating
	SubState    string    `json:"sub_state"`    // running, dead, exited, waiting, etc.
	Timestamp   time.Time `json:"timestamp"`    // When this state was observed
}

// UnitEvent represents a state change event for a systemd unit
type UnitEvent struct {
	Unit      UnitState `json:"unit"`
	EventType string    `json:"event_type"` // "started", "stopped", "failed", "reloaded"
	PrevState string    `json:"prev_state"` // Previous ActiveState (if known)
}

// Common ActiveState values
const (
	ActiveStateActive       = "active"
	ActiveStateInactive     = "inactive"
	ActiveStateFailed       = "failed"
	ActiveStateActivating   = "activating"
	ActiveStateDeactivating = "deactivating"
	ActiveStateReloading    = "reloading"
)

// Common SubState values
const (
	SubStateRunning = "running"
	SubStateDead    = "dead"
	SubStateExited  = "exited"
	SubStateWaiting = "waiting"
	SubStateMounted = "mounted"
)

// Common LoadState values
const (
	LoadStateLoaded   = "loaded"
	LoadStateNotFound = "not-found"
	LoadStateMasked   = "masked"
	LoadStateError    = "error"
)
