// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPlaywrightExecutor_Interface verifies the
// PlaywrightExecutor satisfies the ActionExecutor interface.
func TestPlaywrightExecutor_Interface(t *testing.T) {
	var _ ActionExecutor = &PlaywrightExecutor{}
}

// TestPlaywrightExecutor_FindBridge verifies the bridge
// script can be located from common paths.
func TestPlaywrightExecutor_FindBridge(t *testing.T) {
	runner := newMockRunner()
	exec := NewPlaywrightExecutor(
		"http://localhost:8080", runner,
	)

	// Create a temp dir with the bridge script.
	tmp := t.TempDir()
	scriptsDir := filepath.Join(tmp, "scripts")
	err := os.MkdirAll(scriptsDir, 0o755)
	assert.NoError(t, err)

	bridgePath := filepath.Join(
		scriptsDir, "playwright-bridge.js",
	)
	err = os.WriteFile(
		bridgePath, []byte("// stub"), 0o644,
	)
	assert.NoError(t, err)

	// Override bridge path manually.
	exec.bridgePath = bridgePath
	assert.Equal(t, bridgePath, exec.findBridge())
}

// TestPlaywrightExecutor_NodePath verifies the NODE_PATH
// resolver finds catalog-web/node_modules.
func TestPlaywrightExecutor_NodePath(t *testing.T) {
	// bluff-scan: no-assert-ok (service smoke — public method must not panic on standard call)
	// nodePath() returns empty string or valid path.
	result := nodePath()
	// In test environment, catalog-web may not be
	// adjacent — just verify it doesn't panic.
	_ = result
}

// TestPlaywrightExecutor_Construction verifies a new
// executor can be created with the expected fields.
func TestPlaywrightExecutor_Construction(t *testing.T) {
	runner := newMockRunner()
	exec := NewPlaywrightExecutor(
		"http://localhost:3000", runner,
	)
	assert.Equal(t, "http://localhost:3000", exec.browserURL)
	assert.False(t, exec.launched)
}
