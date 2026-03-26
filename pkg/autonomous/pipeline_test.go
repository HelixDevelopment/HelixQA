// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/llm"
	"digital.vasic.helixqa/pkg/memory"
)

// stubLLM is a minimal LLM provider that returns a canned
// JSON array containing one PlannedTest.
type stubLLM struct{}

func (s *stubLLM) Chat(
	_ context.Context, _ []llm.Message,
) (*llm.Response, error) {
	payload := `[{
		"id": "GEN-001",
		"name": "Verify login",
		"description": "Check login works",
		"category": "functional",
		"priority": 1,
		"platforms": ["web"],
		"screen": "login",
		"steps": ["open login page", "enter credentials"],
		"expected": "user is logged in"
	}]`
	return &llm.Response{
		Content:      payload,
		Model:        "stub",
		InputTokens:  10,
		OutputTokens: 50,
	}, nil
}

func (s *stubLLM) Vision(
	_ context.Context, _ []byte, _ string,
) (*llm.Response, error) {
	return &llm.Response{Content: "ok"}, nil
}

func (s *stubLLM) Name() string          { return "stub" }
func (s *stubLLM) SupportsVision() bool   { return false }

func TestSessionPipeline_Run(t *testing.T) {
	// Set up a temporary project directory with minimal
	// structure so BuildKnowledgeBase succeeds.
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))

	// Memory store in temp dir.
	dbPath := filepath.Join(tmpDir, "data", "memory.db")
	store, err := memory.NewStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	outputDir := filepath.Join(tmpDir, "output")

	cfg := &PipelineConfig{
		ProjectRoot: tmpDir,
		Platforms:   []string{"web"},
		OutputDir:   outputDir,
		Timeout:     30 * time.Second,
		PassNumber:  1,
	}

	pipeline := NewSessionPipeline(
		cfg, &stubLLM{}, store,
	)

	result, err := pipeline.Run(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, StatusComplete, result.Status)
	assert.NotEmpty(t, result.SessionID)
	assert.Equal(t, 1, result.TestsPlanned)
	assert.Equal(t, 1, result.TestsRun)
	assert.Equal(t, 0, result.IssuesFound)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.InDelta(t, 100.0, result.CoveragePct, 0.1)

	// WriteReport should succeed.
	require.NoError(t, pipeline.WriteReport(result))

	reportPath := filepath.Join(
		outputDir, "pipeline-report.json",
	)
	data, err := os.ReadFile(reportPath)
	require.NoError(t, err)

	var decoded PipelineResult
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, StatusComplete, decoded.Status)
	assert.Equal(t, 1, decoded.TestsPlanned)
}

func TestSessionPipeline_EmptyProject(t *testing.T) {
	// An empty directory should still produce a valid
	// result (graceful degradation).
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "data", "memory.db")
	store, err := memory.NewStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	cfg := &PipelineConfig{
		ProjectRoot: tmpDir,
		Platforms:   []string{"android"},
		OutputDir:   filepath.Join(tmpDir, "output"),
		Timeout:     15 * time.Second,
		PassNumber:  1,
	}

	pipeline := NewSessionPipeline(
		cfg, &stubLLM{}, store,
	)

	result, err := pipeline.Run(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should complete without error even on an empty
	// project.
	assert.Equal(t, StatusComplete, result.Status)
	assert.Empty(t, result.Error)
	assert.NotEmpty(t, result.SessionID)
}
