package screenshot

import (
	"context"
	"fmt"
	"testing"

	"digital.vasic.helixqa/pkg/config"
	"github.com/stretchr/testify/assert"
)

// mockEngine is a test engine that never hangs.
type mockEngine struct {
	name      string
	supported bool
	data      []byte
	err       error
}

func (m *mockEngine) Capture(ctx context.Context, opts CaptureOptions) (*Result, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &Result{Data: m.data, Format: "png", Platform: config.PlatformWeb, Engine: m.name}, nil
}

func (m *mockEngine) Supported(ctx context.Context) bool { return m.supported }
func (m *mockEngine) Name() string                       { return m.name }

func TestManager_RegisterAndCapture(t *testing.T) {
	mgr := NewManager(nil)
	mock := &mockEngine{name: "mock-web", supported: true, data: []byte{0x89, 0x50, 0x4E, 0x47}}
	mgr.RegisterEngine(config.PlatformWeb, mock)

	platforms := mgr.SupportedPlatforms(context.Background())
	assert.Contains(t, platforms, config.PlatformWeb)
}

func TestManager_Capture(t *testing.T) {
	mgr := NewManager(nil)
	mock := &mockEngine{name: "mock-web", supported: true, data: []byte{0x89, 0x50, 0x4E, 0x47}}
	mgr.RegisterEngine(config.PlatformWeb, mock)

	result, err := mgr.Capture(context.Background(), config.PlatformWeb, CaptureOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "mock-web", result.Engine)
}

func TestManager_Capture_UnregisteredPlatform(t *testing.T) {
	mgr := NewManager(nil)
	_, err := mgr.Capture(context.Background(), config.PlatformLinux, CaptureOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no engine registered")
}

func TestManager_CaptureAll(t *testing.T) {
	mgr := NewManager(nil)
	mgr.RegisterEngine(config.PlatformWeb, &mockEngine{name: "mock-web", supported: true, data: []byte{0x89}})
	mgr.RegisterEngine(config.PlatformLinux, &mockEngine{name: "mock-linux", supported: false})

	results, err := mgr.CaptureAll(context.Background(), CaptureOptions{})
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "mock-web", results[0].Engine)
}

func TestManager_CaptureResponsive(t *testing.T) {
	mgr := NewManager(nil)
	mgr.RegisterEngine(config.PlatformWeb, &mockEngine{name: "mock-web", supported: true, data: []byte{0x89}})

	breakpoints := []Breakpoint{
		{Name: "mobile", Width: 375, Height: 667},
		{Name: "desktop", Width: 1440, Height: 900},
	}
	results, err := mgr.CaptureResponsive(context.Background(), breakpoints, CaptureOptions{})
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "mobile", results[0].Breakpoint)
	assert.Equal(t, "desktop", results[1].Breakpoint)
}

func TestManager_CaptureResponsive_NoWebEngine(t *testing.T) {
	mgr := NewManager(nil)
	_, err := mgr.CaptureResponsive(context.Background(), []Breakpoint{{Name: "test", Width: 100, Height: 100}}, CaptureOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no web engine registered")
}

func TestManager_SupportedPlatforms(t *testing.T) {
	mgr := NewManager(nil)
	mgr.RegisterEngine(config.PlatformWeb, &mockEngine{name: "mock-web", supported: true})
	mgr.RegisterEngine(config.PlatformLinux, &mockEngine{name: "mock-linux", supported: false})

	platforms := mgr.SupportedPlatforms(context.Background())
	assert.Len(t, platforms, 1)
	assert.Contains(t, platforms, config.PlatformWeb)
}

func TestEngineInterface(t *testing.T) {
	var _ Engine = &mockEngine{}
}

func TestMockEngine_Name(t *testing.T) {
	m := &mockEngine{name: "test-engine"}
	assert.Equal(t, "test-engine", m.Name())
}

func TestMockEngine_Supported(t *testing.T) {
	m := &mockEngine{supported: true}
	assert.True(t, m.Supported(context.Background()))
}

func TestMockEngine_Capture_Error(t *testing.T) {
	m := &mockEngine{err: fmt.Errorf("capture failed")}
	_, err := m.Capture(context.Background(), CaptureOptions{})
	assert.Error(t, err)
}
