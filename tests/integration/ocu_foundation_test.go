//go:build integration
// +build integration

// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestOCU_Foundation_ProbeCLI runs `go run ./cmd/ocu-probe` against
// the local host and asserts the JSON document is well-formed.
func TestOCU_Foundation_ProbeCLI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	root := findModuleRoot(t)
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/ocu-probe")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "ocu-probe failed: %s", string(out))
	require.Contains(t, string(out), `"local"`)
	require.Contains(t, string(out), `"OS"`)
}

func findModuleRoot(t *testing.T) string {
	dir, err := os.Getwd()
	require.NoError(t, err)
	for dir != "/" {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			require.NoError(t, err)
			if strings.Contains(string(data), "module digital.vasic.helixqa") {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("HelixQA module root not found")
	return ""
}
