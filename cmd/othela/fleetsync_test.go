package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"helvilette/pkg/log"
	"helvilette/pkg/playbook"
)

// syncBuffer is the log destination used by these tests. It needs its own lock
// because the fleet sync goroutine writes to it while the test body reads.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// waitFor blocks until the buffer contains want, or fails the test. The fleet
// sync loop reports itself only through logs, so this is the seam that can
// observe it.
func (b *syncBuffer) waitFor(t *testing.T, want string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(b.String(), want) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("log line %q never appeared within 2s; got:\n%s", want, b.String())
}

// captureLogs redirects log output for the duration of run.
func captureLogs(t *testing.T, run func(*syncBuffer)) string {
	t.Helper()

	sink := &syncBuffer{}
	previous := log.SetOutput(sink)
	defer log.SetOutput(previous)

	run(sink)

	return sink.String()
}

// unreachableFleet points at a path that is not a repository, so the first sync
// fails immediately without touching the network.
func unreachableFleet(t *testing.T, interval time.Duration) FleetSyncConfig {
	t.Helper()

	return FleetSyncConfig{
		Repo:     filepath.Join(t.TempDir(), "no-such-repo"),
		Branch:   "main",
		CacheDir: filepath.Join(t.TempDir(), "cache"),
		Interval: interval,
	}
}

// loadedFleet stands in for playbooks a previous sync had already published.
func loadedFleet() []playbook.Playbook {
	return []playbook.Playbook{{ID: "aaaa", Name: "baseline", Path: "baseline"}}
}

// TestStartFleetSync_StopsOnContextCancel covers the lifecycle gap found while
// fixing issue #32: the poll loop had no exit and outlived graceful shutdown.
func TestStartFleetSync_StopsOnContextCancel(t *testing.T) {
	server := NewServer()
	ctx, cancel := context.WithCancel(t.Context())

	output := captureLogs(t, func(sink *syncBuffer) {
		server.StartFleetSync(ctx, unreachableFleet(t, 10*time.Millisecond))
		cancel()
		sink.waitFor(t, "fleet sync loop stopped")
	})

	if !strings.Contains(output, "failed to sync fleet repository") {
		t.Errorf("an unreachable fleet repo produced no error log:\n%s", output)
	}
}

// TestStartFleetSync_FailureKeepsPreviousPlaybooks asserts the sync loop never
// empties the fleet on a transient Git outage. Dropping to zero playbooks would
// silently stop dispatching to every node.
func TestStartFleetSync_FailureKeepsPreviousPlaybooks(t *testing.T) {
	server := NewServer()
	server.SetPlaybooks(loadedFleet())

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	captureLogs(t, func(_ *syncBuffer) {
		server.StartFleetSync(ctx, unreachableFleet(t, time.Hour))
	})

	if got := len(server.GetPlaybooks()); got != 1 {
		t.Errorf("playbook count = %d after a failed sync, want 1 — the previous fleet must survive", got)
	}
	if server.LastCommit() != "" {
		t.Errorf("LastCommit = %q after a failed sync, want empty", server.LastCommit())
	}
}

// TestEnsureLoader_ConcurrentWithGetLoader guards the data race found while
// fixing issue #32: StartFleetSync wrote s.loader with no lock while GetLoader
// read it. Run with -race.
func TestEnsureLoader_ConcurrentWithGetLoader(t *testing.T) {
	server := NewServer()
	cacheDir := t.TempDir()

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if n%2 == 0 {
				if _, err := server.ensureLoader(cacheDir); err != nil {
					t.Errorf("ensureLoader failed: %v", err)
				}
				return
			}
			server.GetLoader()
			server.LastCommit()
		}(i)
	}
	wg.Wait()

	loader := server.GetLoader()
	if loader == nil {
		t.Fatal("GetLoader returned nil after ensureLoader")
	}
	if loader.BaseDir() != mustAbs(t, cacheDir) {
		t.Errorf("BaseDir = %s, want %s", loader.BaseDir(), cacheDir)
	}
}

// TestPublishFleet_LogsOnlyWhenTheCommitMoves keeps the poll loop quiet at info
// level while nothing changes, and audible the moment it does. Before this, a
// successful sync logged at debug only, so an operator could not tell a working
// loop from a dead one.
func TestPublishFleet_LogsOnlyWhenTheCommitMoves(t *testing.T) {
	server := NewServer()
	fleet := loadedFleet()

	first := captureLogs(t, func(_ *syncBuffer) { server.publishFleet(fleet, "abc123") })
	if !strings.Contains(first, `"message":"fleet updated"`) {
		t.Errorf("first publish did not log at info:\n%s", first)
	}
	if !strings.Contains(first, `"fleet_commit":"abc123"`) {
		t.Errorf("first publish did not name the commit:\n%s", first)
	}

	second := captureLogs(t, func(_ *syncBuffer) { server.publishFleet(fleet, "abc123") })
	if strings.Contains(second, `"message":"fleet updated"`) {
		t.Errorf("an unchanged commit logged as an update:\n%s", second)
	}

	third := captureLogs(t, func(_ *syncBuffer) { server.publishFleet(fleet, "def456") })
	if !strings.Contains(third, `"previous_commit":"abc123"`) {
		t.Errorf("a moved commit did not name its predecessor:\n%s", third)
	}
	if server.LastCommit() != "def456" {
		t.Errorf("LastCommit = %q, want def456", server.LastCommit())
	}
}

// TestE2EComposeFlagsExistOnTheCLI is the guard against issue #33 recurring.
// docker-compose.e2e.yaml passed --fleet-repo while the CLI only defined
// --playbook-dir, and nothing failed until someone started the stack by hand.
func TestE2EComposeFlagsExistOnTheCLI(t *testing.T) {
	for _, flagName := range othelaComposeFlags(t) {
		if rootCmd.Flags().Lookup(flagName) == nil {
			t.Errorf("docker-compose.e2e.yaml passes --%s, which othela does not define", flagName)
		}
	}
}

// othelaComposeFlags reads the long flag names from the othela service's command
// list in docker-compose.e2e.yaml.
func othelaComposeFlags(t *testing.T) []string {
	t.Helper()

	// Services are decoded one at a time: git-server writes its command as a
	// single shell string, which does not fit the list shape othela uses.
	var compose struct {
		Services map[string]yaml.Node `yaml:"services"`
	}

	raw, err := os.ReadFile(filepath.Join("..", "..", "docker-compose.e2e.yaml"))
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}
	if err := yaml.Unmarshal(raw, &compose); err != nil {
		t.Fatalf("parse compose file: %v", err)
	}

	node, ok := compose.Services["othela"]
	if !ok {
		t.Fatal("docker-compose.e2e.yaml has no othela service")
	}

	var othela struct {
		Command []string `yaml:"command"`
	}
	if err := node.Decode(&othela); err != nil {
		t.Fatalf("decode othela service: %v", err)
	}

	var names []string
	for _, arg := range othela.Command {
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		names = append(names, strings.SplitN(strings.TrimPrefix(arg, "--"), "=", 2)[0])
	}

	if len(names) == 0 {
		t.Fatal("othela service passes no long flags; the guard would pass vacuously")
	}
	return names
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()

	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("abs %s: %v", path, err)
	}
	return abs
}
