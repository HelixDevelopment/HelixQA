// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package infra manages QA infrastructure using the
// Containers submodule. It provides a QAInfraManager that
// boots PostgreSQL, Redis, and catalog-api services via
// the BootManager, replacing manual Docker/Podman commands.
package infra

import (
	"context"
	"fmt"
	"time"

	"digital.vasic.containers/pkg/boot"
	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/endpoint"
	"digital.vasic.containers/pkg/health"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/runtime"
)

// ServiceConfig describes a QA infrastructure service.
type ServiceConfig struct {
	Name        string
	Host        string
	Port        string
	HealthType  string // "tcp" or "http"
	HealthPath  string // for http checks
	Required    bool
	Remote      bool   // true = unmanaged external service
	ComposeFile string // for locally managed services
	ServiceName string // compose service name
}

// QAInfraConfig holds the full QA infrastructure layout.
type QAInfraConfig struct {
	// Services to boot and health-check.
	Services []ServiceConfig

	// ProjectDir is the project root (for compose file
	// resolution).
	ProjectDir string

	// HealthTimeout per service.
	HealthTimeout time.Duration
}

// DefaultQAInfraConfig returns the standard Catalogizer QA
// infrastructure: PostgreSQL, Redis, and catalog-api on a
// remote host (amber.local by default).
func DefaultQAInfraConfig(host string) QAInfraConfig {
	if host == "" {
		host = "amber.local"
	}
	return QAInfraConfig{
		Services: []ServiceConfig{
			{
				Name:       "postgres",
				Host:       host,
				Port:       "5432",
				HealthType: "tcp",
				Required:   true,
				Remote:     true,
			},
			{
				Name:       "redis",
				Host:       host,
				Port:       "6379",
				HealthType: "tcp",
				Required:   false,
				Remote:     true,
			},
			{
				Name:       "catalog-api",
				Host:       host,
				Port:       "8080",
				HealthType: "http",
				HealthPath: "/health",
				Required:   true,
				Remote:     true,
			},
		},
		HealthTimeout: 5 * time.Second,
	}
}

// QAInfraManager wraps the Containers BootManager for QA
// infrastructure lifecycle management.
type QAInfraManager struct {
	config  QAInfraConfig
	bootMgr *boot.BootManager
	logger  logging.Logger
}

// NewQAInfraManager creates a manager from the given config.
// It auto-detects the container runtime and sets up health
// checking.
func NewQAInfraManager(
	cfg QAInfraConfig,
) (*QAInfraManager, error) {
	logger := logging.NewSlogAdapter(nil)

	// Build endpoint map from config.
	endpoints := make(map[string]endpoint.ServiceEndpoint)
	for _, svc := range cfg.Services {
		timeout := cfg.HealthTimeout
		if timeout == 0 {
			timeout = 10 * time.Second
		}
		b := endpoint.NewEndpoint().
			WithHost(svc.Host).
			WithPort(svc.Port).
			WithRequired(svc.Required).
			WithEnabled(true).
			WithRemote(svc.Remote).
			WithHealthType(svc.HealthType).
			WithTimeout(timeout).
			WithRetryCount(3)

		if svc.HealthPath != "" {
			b = b.WithHealthPath(svc.HealthPath)
		}
		if svc.ComposeFile != "" {
			b = b.WithComposeFile(svc.ComposeFile).
				WithServiceName(svc.ServiceName)
		}
		endpoints[svc.Name] = b.Build()
	}

	// Auto-detect runtime.
	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second,
	)
	defer cancel()
	rt, err := runtime.AutoDetect(ctx)
	if err != nil {
		// Runtime detection is optional for remote-only
		// services. Log and continue.
		fmt.Printf(
			"  [infra] runtime detection: %v "+
				"(remote-only mode)\n", err,
		)
	}

	// Create orchestrator (optional — only needed for
	// locally managed services).
	var orch compose.ComposeOrchestrator
	if cfg.ProjectDir != "" {
		orch, _ = compose.NewDefaultOrchestrator(
			cfg.ProjectDir, logger,
		)
	}

	hc := health.NewDefaultChecker()

	opts := []boot.BootManagerOption{
		boot.WithHealthChecker(hc),
		boot.WithLogger(logger),
	}
	if rt != nil {
		opts = append(opts, boot.WithRuntime(rt))
	}
	if orch != nil {
		opts = append(opts, boot.WithOrchestrator(orch))
	}

	mgr := boot.NewBootManager(endpoints, opts...)

	return &QAInfraManager{
		config:  cfg,
		bootMgr: mgr,
		logger:  logger,
	}, nil
}

// Boot starts all configured services and runs health
// checks. Returns a summary of which services are up.
func (m *QAInfraManager) Boot(
	ctx context.Context,
) (*boot.BootSummary, error) {
	fmt.Println("[infra] Booting QA infrastructure...")
	summary, err := m.bootMgr.BootAll(ctx)
	if err != nil {
		return summary, fmt.Errorf("infra boot: %w", err)
	}
	fmt.Printf("[infra] %s\n", summary.String())
	return summary, nil
}

// HealthCheck runs health checks on all services.
func (m *QAInfraManager) HealthCheck(
	ctx context.Context,
) map[string]error {
	return m.bootMgr.HealthCheckAll(ctx)
}

// Shutdown gracefully stops all managed services.
func (m *QAInfraManager) Shutdown(
	ctx context.Context,
) error {
	fmt.Println("[infra] Shutting down QA infrastructure...")
	return m.bootMgr.Shutdown(ctx)
}
