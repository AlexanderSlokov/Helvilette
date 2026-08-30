package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestGracefulShutdown(t *testing.T) {
	server := NewServer()

	// Start the server on a random port
	httpServer := server.NewHTTPServer(":0")

	// Capture the actual address after starting
	errChan := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- fmt.Errorf("unexpected server error: %v", err)
		}
		errChan <- nil
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Verify server is running by hitting /healthz
	// We need the actual address - use the Addr field
	// Since :0 picks a random port, we need to get the actual listener address.
	// We'll use a known port for this test instead.
	t.Run("shutdown_sets_not_ready", func(t *testing.T) {
		if !server.IsReady() {
			t.Fatal("server should be ready before shutdown")
		}

		// Simulate the graceful shutdown sequence from main.go:
		// 1. Mark not-ready so /readyz returns 503 (drain signal)
		server.SetReady(false)
		if server.IsReady() {
			t.Error("server should NOT be ready after SetReady(false)")
		}

		// 2. Then shutdown the HTTP server to stop accepting new connections
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			t.Fatalf("graceful shutdown failed: %v", err)
		}

		// After full shutdown, server should still be not-ready
		if server.IsReady() {
			t.Error("server should NOT be ready after shutdown")
		}

		// Wait for server goroutine to finish
		if err := <-errChan; err != nil {
			t.Fatalf("server goroutine error: %v", err)
		}
	})
}

func TestNewHTTPServer_Configuration(t *testing.T) {
	server := NewServer()
	httpSrv := server.NewHTTPServer(":9090")

	if httpSrv.Addr != ":9090" {
		t.Errorf("expected addr :9090, got %s", httpSrv.Addr)
	}

	if httpSrv.ReadTimeout != 10*time.Second {
		t.Errorf("expected ReadTimeout 10s, got %v", httpSrv.ReadTimeout)
	}

	if httpSrv.WriteTimeout != 10*time.Second {
		t.Errorf("expected WriteTimeout 10s, got %v", httpSrv.WriteTimeout)
	}

	if httpSrv.IdleTimeout != 60*time.Second {
		t.Errorf("expected IdleTimeout 60s, got %v", httpSrv.IdleTimeout)
	}
}

func TestSetReady_ToggleState(t *testing.T) {
	server := NewServer()

	// Should start as ready
	if !server.IsReady() {
		t.Fatal("server should start as ready")
	}

	server.SetReady(false)
	if server.IsReady() {
		t.Error("expected not ready after SetReady(false)")
	}

	server.SetReady(true)
	if !server.IsReady() {
		t.Error("expected ready after SetReady(true)")
	}
}
