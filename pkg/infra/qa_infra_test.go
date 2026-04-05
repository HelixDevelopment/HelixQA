// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package infra

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultQAInfraConfig(t *testing.T) {
	cfg := DefaultQAInfraConfig("")
	assert.Equal(t, "amber.local", cfg.Services[0].Host)
	assert.Len(t, cfg.Services, 3)

	// postgres
	assert.Equal(t, "postgres", cfg.Services[0].Name)
	assert.Equal(t, "5432", cfg.Services[0].Port)
	assert.Equal(t, "tcp", cfg.Services[0].HealthType)
	assert.True(t, cfg.Services[0].Required)

	// redis
	assert.Equal(t, "redis", cfg.Services[1].Name)
	assert.Equal(t, "6379", cfg.Services[1].Port)
	assert.False(t, cfg.Services[1].Required)

	// catalog-api
	assert.Equal(t, "catalog-api", cfg.Services[2].Name)
	assert.Equal(t, "8080", cfg.Services[2].Port)
	assert.Equal(t, "http", cfg.Services[2].HealthType)
	assert.Equal(t, "/health", cfg.Services[2].HealthPath)
	assert.True(t, cfg.Services[2].Required)
}

func TestDefaultQAInfraConfig_CustomHost(t *testing.T) {
	cfg := DefaultQAInfraConfig("myhost.local")
	for _, svc := range cfg.Services {
		assert.Equal(t, "myhost.local", svc.Host)
	}
}

func TestNewQAInfraManager(t *testing.T) {
	cfg := DefaultQAInfraConfig("localhost")
	mgr, err := NewQAInfraManager(cfg)
	require.NoError(t, err)
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.bootMgr)
}

func TestServiceConfig_Fields(t *testing.T) {
	svc := ServiceConfig{
		Name:        "test-db",
		Host:        "db.local",
		Port:        "5432",
		HealthType:  "tcp",
		Required:    true,
		Remote:      true,
		ComposeFile: "docker-compose.yml",
		ServiceName: "postgres",
	}
	assert.Equal(t, "test-db", svc.Name)
	assert.Equal(t, "db.local", svc.Host)
	assert.True(t, svc.Remote)
}

func TestQAInfraConfig_HealthTimeout(t *testing.T) {
	cfg := DefaultQAInfraConfig("")
	assert.Equal(t, 15*time.Second, cfg.HealthTimeout)
}
