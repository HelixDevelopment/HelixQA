//go:build linux

// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Linux half of the §11.4.81 desktop-screenshot split: the X11 `import`
// path streams PNG bytes on stdout, so it is exercised with the mock runner.

func TestX11Executor_Screenshot(t *testing.T) {
	runner := newMockRunner()
	runner.response = []byte("X11-SCREENSHOT")
	exec := NewX11Executor(":0", runner)

	data, err := exec.Screenshot(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []byte("X11-SCREENSHOT"), data)
}

func TestX11Executor_Screenshot_Error(t *testing.T) {
	runner := newMockRunner()
	runner.failOn["import"] = fmt.Errorf("no display")
	exec := NewX11Executor(":0", runner)

	_, err := exec.Screenshot(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "x11 screenshot")
}
