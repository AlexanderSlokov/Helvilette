package systemd

import "testing"

func TestDetermineEventType(t *testing.T) {
	tests := []struct {
		name        string
		activeState string
		subState    string
		expected    string
	}{
		{"active running", ActiveStateActive, SubStateRunning, "started"},
		{"active other", ActiveStateActive, SubStateExited, "active"},
		{"inactive", ActiveStateInactive, SubStateDead, "stopped"},
		{"failed", ActiveStateFailed, SubStateDead, "failed"},
		{"activating", ActiveStateActivating, SubStateWaiting, "starting"},
		{"deactivating", ActiveStateDeactivating, "", "stopping"},
		{"reloading", ActiveStateReloading, "", "reloading"},
		{"unknown state", "some-unknown-state", "", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineEventType(tt.activeState, tt.subState)
			if result != tt.expected {
				t.Errorf("determineEventType(%q, %q) = %q, want %q",
					tt.activeState, tt.subState, result, tt.expected)
			}
		})
	}
}

func TestNewWatcher(t *testing.T) {
	// Test with nil client (should not panic)
	units := []string{"nginx.service", "ssh.service"}
	watcher := NewWatcher(nil, units)

	if watcher == nil {
		t.Fatal("NewWatcher returned nil")
	}

	if len(watcher.watchUnits) != 2 {
		t.Errorf("expected 2 watch units, got %d", len(watcher.watchUnits))
	}

	if !watcher.watchUnits["nginx.service"] {
		t.Error("nginx.service should be in watch list")
	}

	if !watcher.watchUnits["ssh.service"] {
		t.Error("ssh.service should be in watch list")
	}
}

func TestNewWatcher_EmptyUnits(t *testing.T) {
	watcher := NewWatcher(nil, []string{})

	if watcher == nil {
		t.Fatal("NewWatcher returned nil")
	}

	if len(watcher.watchUnits) != 0 {
		t.Errorf("expected 0 watch units for empty input, got %d", len(watcher.watchUnits))
	}
}
