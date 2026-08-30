package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// HealthResponse mirrors the expected JSON response from health endpoints
type HealthResponse struct {
	Status string `json:"status"`
}

func TestHealthzEndpoint_ReturnsOK(t *testing.T) {
	server := NewServer()

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
}

func TestReadyzEndpoint_ReturnsOK(t *testing.T) {
	server := NewServer()

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
}

func TestReadyzEndpoint_NotReady_Returns503(t *testing.T) {
	server := NewServer()
	// Simulate the server not being ready (e.g. during shutdown)
	server.SetReady(false)

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 when not ready, got %d", w.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "not_ready" {
		t.Errorf("expected status 'not_ready', got %q", resp.Status)
	}
}

func TestHealthzEndpoint_MethodNotAllowed(t *testing.T) {
	server := NewServer()

	req := httptest.NewRequest("POST", "/healthz", nil)
	w := httptest.NewRecorder()

	server.Router().ServeHTTP(w, req)

	// gorilla/mux returns 405 for wrong method on registered route
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405 for POST on /healthz, got %d", w.Code)
	}
}

func TestReadyzEndpoint_MethodNotAllowed(t *testing.T) {
	server := NewServer()

	req := httptest.NewRequest("POST", "/readyz", nil)
	w := httptest.NewRecorder()

	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405 for POST on /readyz, got %d", w.Code)
	}
}

func TestHealthEndpoints_IndependentOfJobs(t *testing.T) {
	// Health endpoints should work regardless of job state
	server := NewServerWithJob(Job{JobID: "test-job", RepoURL: "git://git-server:9418/test", PlaybookPath: "playbook.yml"})

	// /healthz should still work
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/healthz should return 200 regardless of job state, got %d", w.Code)
	}

	// /readyz should still work
	req = httptest.NewRequest("GET", "/readyz", nil)
	w = httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/readyz should return 200 regardless of job state, got %d", w.Code)
	}
}
