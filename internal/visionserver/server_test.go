// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package visionserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/vision/cheaper"
)

// freePort asks the OS for an available TCP port on localhost and returns it.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "freePort: listen failed")
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	return port
}

// TestServer_StartStop starts the server on a random port, probes the /health
// endpoint, then shuts it down gracefully.
func TestServer_StartStop(t *testing.T) {
	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	cfg := &Config{ListenAddr: addr}
	h := NewHandler(
		&stubExecutor{},
		cheaper.NewRegistry(),
		cheaper.NewMetrics("test_server_startstop"),
	)
	srv := NewServer(cfg, h)

	require.NoError(t, srv.Start())

	// Poll until the server is ready (up to 2 s).
	healthURL := fmt.Sprintf("http://%s/health", addr)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	var lastErr error
	for i := 0; i < 20; i++ {
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			lastErr = nil
			break
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	require.NoError(t, lastErr, "server never became ready")

	// Verify the health response body.
	resp, err := client.Get(healthURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "healthy", body["status"])

	// Graceful shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	assert.NoError(t, srv.Stop(ctx))

	// After shutdown the port should be closed.
	_, err = client.Get(healthURL)
	assert.Error(t, err, "expected connection refused after shutdown")
}

// TestServer_StopWithoutStart verifies that Stop on a never-started server
// returns no error (Shutdown on an unstarted http.Server is benign).
func TestServer_StopWithoutStart(t *testing.T) {
	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	h := NewHandler(
		&stubExecutor{},
		cheaper.NewRegistry(),
		cheaper.NewMetrics("test_server_stoponly"),
	)
	srv := NewServer(cfg, h)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// Shutdown on a never-started server should not block or panic.
	assert.NoError(t, srv.Stop(ctx))
}

// TestServer_RouteWiring verifies that all five routes respond on the started
// server (not just /health).
func TestServer_RouteWiring(t *testing.T) {
	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	cfg := &Config{ListenAddr: addr}
	h := NewHandler(
		&stubExecutor{result: &cheaper.VisionResult{Text: "ok"}},
		cheaper.NewRegistry(),
		cheaper.NewMetrics("test_server_routes"),
	)
	srv := NewServer(cfg, h)
	require.NoError(t, srv.Start())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	})

	client := &http.Client{Timeout: time.Second}
	base := fmt.Sprintf("http://%s", addr)

	// Wait for server readiness.
	require.Eventually(t, func() bool {
		resp, err := client.Get(base + "/health")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 2*time.Second, 50*time.Millisecond, "server not ready")

	routes := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodGet, "/health", http.StatusOK},
		{http.MethodGet, "/providers", http.StatusOK},
		{http.MethodGet, "/learning/stats", http.StatusOK},
		{http.MethodPost, "/learning/clear", http.StatusOK},
		// /analyze with no body → bad request, but the route exists.
		{http.MethodPost, "/analyze", http.StatusBadRequest},
	}

	for _, tc := range routes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var resp *http.Response
			var err error
			switch tc.method {
			case http.MethodGet:
				resp, err = client.Get(base + tc.path)
			case http.MethodPost:
				resp, err = client.Post(base+tc.path, "application/json", nil)
			}
			require.NoError(t, err)
			resp.Body.Close()
			assert.Equal(t, tc.want, resp.StatusCode)
		})
	}
}
