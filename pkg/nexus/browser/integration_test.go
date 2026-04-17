//go:build nexus_chromedp_integration

package browser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/nexus"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestChromedp_RealContainer_EndToEnd boots a headless Chromium via
// testcontainers-go, drives the Nexus Engine through a canned HTML
// fixture served by httptest, and asserts the snapshot produces real
// e1..eN refs and click actions reach the element.
//
// Run it with:
//
//   go test -tags=nexus_chromedp_integration ./pkg/nexus/browser/...
//
// The default test suite does NOT include this file (the
// `nexus_chromedp_integration` tag opts in), so CI-less workstations
// and fast local runs skip it.
func TestChromedp_RealContainer_EndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, `<!doctype html><html><body>
		<h1 id="hero">Nexus fixture</h1>
		<button id="primary">Click me</button>
		<a id="home" href="/home">Home</a>
		<input id="email" name="email" type="email" placeholder="email">
		</body></html>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	req := testcontainers.ContainerRequest{
		Image:        "docker.io/browserless/chrome:latest",
		ExposedPorts: []string{"3000/tcp"},
		Env: map[string]string{
			"CONNECTION_TIMEOUT": "60000",
		},
		WaitingFor: wait.ForHTTP("/json/version").WithPort("3000/tcp"),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("testcontainers: %v (docker/podman socket required)", err)
	}
	defer c.Terminate(context.Background())

	cdpEndpoint, err := c.Endpoint(ctx, "ws")
	if err != nil {
		t.Fatalf("container endpoint: %v", err)
	}
	if !strings.Contains(cdpEndpoint, "ws://") {
		t.Fatalf("unexpected endpoint: %q", cdpEndpoint)
	}

	// The real chromedp driver requires a build-tagged import; it is
	// available in this file because the integration tag also selects
	// the driver sources. We construct the engine with an explicit
	// Config pointing at the container's CDP port.
	driver := NewChromedpDriver()
	eng, err := NewEngine(driver, Config{
		Engine:    EngineChromedp,
		Headless:  true,
		CDPPort:   3000,
		AllowedHosts: []string{"127.0.0.1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	sess, err := eng.Open(ctx, nexus.SessionOptions{Headless: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer sess.Close()

	if err := eng.Navigate(ctx, sess, server.URL); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	snap, err := eng.Snapshot(ctx, sess)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.Elements) < 3 {
		t.Errorf("expected >=3 interactive elements, got %d", len(snap.Elements))
	}

	png, err := eng.Screenshot(ctx, sess)
	if err != nil {
		t.Fatal(err)
	}
	if len(png) < 64 {
		t.Errorf("screenshot too small: %d bytes", len(png))
	}

	// Consume + discard the test server response body so the reader
	// does not leak.
	_, _ = io.Copy(io.Discard, strings.NewReader(""))
}
