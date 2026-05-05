//go:build nexus_appium_integration

package mobile

import (
	"context"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/nexus"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestAppium_RealContainer_EndToEnd boots an Appium 2.0 hub via
// testcontainers-go and drives the Nexus mobile Engine against a
// headless Android emulator image. Opt in with:
//
//	go test -tags=nexus_appium_integration ./pkg/nexus/mobile/...
//
// The default suite skips this file. Operators need Docker/Podman +
// KVM on the host; when the container cannot start the test is
// skipped rather than failed so the harness itself does not become a
// flake source.
func TestAppium_RealContainer_EndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req := testcontainers.ContainerRequest{
		Image:        "docker.io/budtmo/docker-android:emulator_13.0",
		ExposedPorts: []string{"4723/tcp", "6080/tcp"},
		Env: map[string]string{
			"APPIUM":          "true",
			"EMULATOR_DEVICE": "Samsung Galaxy S10",
			"WEB_VNC":         "false",
		},
		WaitingFor: wait.ForHTTP("/status").WithPort("4723/tcp").WithStartupTimeout(4 * time.Minute),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("testcontainers: %v (docker/podman + KVM required)", err)
	}
	defer c.Terminate(context.Background())

	endpoint, err := c.PortEndpoint(ctx, "4723/tcp", "http")
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}

	caps := NewAndroidCaps("emulator-5554", "com.android.settings", ".Settings")
	caps.OSVersion = "13.0"
	caps.NewCommandTimeoutSec = 120

	engine, err := NewEngine(endpoint, caps)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sess, err := engine.Open(ctx, nexus.SessionOptions{Headless: true})
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	defer sess.Close()

	snap, err := engine.Snapshot(ctx, sess)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snap == nil || len(snap.Tree) == 0 {
		t.Error("snapshot tree empty")
	}
}
