package screenshot

import (
	"context"
	"fmt"
	"sync"

	"digital.vasic.helixqa/pkg/config"
)

// Manager is the public face of the screenshot package.
type Manager struct {
	engines map[config.Platform]Engine
	store   Storage
	mu      sync.RWMutex
}

// NewManager creates a new screenshot manager.
func NewManager(store Storage) *Manager {
	return &Manager{
		engines: make(map[config.Platform]Engine),
		store:   store,
	}
}

// RegisterEngine registers a platform engine.
func (m *Manager) RegisterEngine(platform config.Platform, e Engine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.engines[platform] = e
}

// Capture takes a screenshot for the specified platform.
func (m *Manager) Capture(ctx context.Context, platform config.Platform, opts CaptureOptions) (*Result, error) {
	m.mu.RLock()
	engine, ok := m.engines[platform]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no engine registered for platform: %s", platform)
	}
	return engine.Capture(ctx, opts)
}

// CaptureAll captures screenshots for all registered engines.
func (m *Manager) CaptureAll(ctx context.Context, opts CaptureOptions) ([]*Result, error) {
	m.mu.RLock()
	engines := make(map[config.Platform]Engine)
	for k, v := range m.engines {
		engines[k] = v
	}
	m.mu.RUnlock()

	var results []*Result
	for plat, engine := range engines {
		if !engine.Supported(ctx) {
			continue
		}
		res, err := engine.Capture(ctx, opts)
		if err != nil {
			continue
		}
		res.Platform = plat
		results = append(results, res)
	}
	return results, nil
}

// CaptureResponsive captures at multiple breakpoints for the web platform.
func (m *Manager) CaptureResponsive(ctx context.Context, breakpoints []Breakpoint, opts CaptureOptions) ([]*Result, error) {
	m.mu.RLock()
	engine, ok := m.engines[config.PlatformWeb]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no web engine registered")
	}

	var results []*Result
	for _, bp := range breakpoints {
		o := opts
		o.Width = bp.Width
		o.Height = bp.Height
		res, err := engine.Capture(ctx, o)
		if err != nil {
			continue
		}
		res.Breakpoint = bp.Name
		results = append(results, res)
	}
	return results, nil
}

// SupportedPlatforms returns a list of platforms with supported engines.
func (m *Manager) SupportedPlatforms(ctx context.Context) []config.Platform {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []config.Platform
	for plat, engine := range m.engines {
		if engine.Supported(ctx) {
			out = append(out, plat)
		}
	}
	return out
}
