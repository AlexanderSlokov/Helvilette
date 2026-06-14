package systemd

import "testing"

func TestUnitStateConstants(t *testing.T) {
	// Verify constants match expected systemd values
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		// ActiveState
		{"ActiveStateActive", ActiveStateActive, "active"},
		{"ActiveStateInactive", ActiveStateInactive, "inactive"},
		{"ActiveStateFailed", ActiveStateFailed, "failed"},
		{"ActiveStateActivating", ActiveStateActivating, "activating"},
		{"ActiveStateDeactivating", ActiveStateDeactivating, "deactivating"},
		{"ActiveStateReloading", ActiveStateReloading, "reloading"},

		// SubState
		{"SubStateRunning", SubStateRunning, "running"},
		{"SubStateDead", SubStateDead, "dead"},
		{"SubStateExited", SubStateExited, "exited"},
		{"SubStateWaiting", SubStateWaiting, "waiting"},
		{"SubStateMounted", SubStateMounted, "mounted"},

		// LoadState
		{"LoadStateLoaded", LoadStateLoaded, "loaded"},
		{"LoadStateNotFound", LoadStateNotFound, "not-found"},
		{"LoadStateMasked", LoadStateMasked, "masked"},
		{"LoadStateError", LoadStateError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestUnitState_Fields(t *testing.T) {
	state := UnitState{
		Name:        "nginx.service",
		Description: "A high performance web server",
		LoadState:   LoadStateLoaded,
		ActiveState: ActiveStateActive,
		SubState:    SubStateRunning,
	}

	if state.Name != "nginx.service" {
		t.Errorf("Name = %q, want %q", state.Name, "nginx.service")
	}
	if state.ActiveState != "active" {
		t.Errorf("ActiveState = %q, want %q", state.ActiveState, "active")
	}
}

func TestUnitEvent_Fields(t *testing.T) {
	event := UnitEvent{
		Unit: UnitState{
			Name:        "ssh.service",
			ActiveState: ActiveStateActive,
		},
		EventType: "started",
		PrevState: ActiveStateInactive,
	}

	if event.EventType != "started" {
		t.Errorf("EventType = %q, want %q", event.EventType, "started")
	}
	if event.PrevState != "inactive" {
		t.Errorf("PrevState = %q, want %q", event.PrevState, "inactive")
	}
}
