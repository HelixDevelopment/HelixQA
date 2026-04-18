// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun_LocalOnly(t *testing.T) {
	var buf bytes.Buffer
	err := run(context.Background(), &buf, nil)
	require.NoError(t, err)

	var out probeOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	require.NotNil(t, out.Local)
	require.NotEmpty(t, out.Local.OS)
}
