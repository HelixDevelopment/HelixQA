// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package x264_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/record/encoder"
	_ "digital.vasic.helixqa/pkg/nexus/record/encoder/x264"
)

// TestX264_FactoryRegistered verifies the init() registers "x264" in the
// parent encoder factory.
func TestX264_FactoryRegistered(t *testing.T) {
	kinds := encoder.Kinds()
	assert.Contains(t, kinds, "x264", "x264 must be registered via init()")
}

// TestX264_ProductionReturnsErrNotWired verifies that the production stub
// returns ErrNotWired from Encode().
func TestX264_ProductionReturnsErrNotWired(t *testing.T) {
	enc, err := encoder.New("x264")
	require.NoError(t, err)
	require.NotNil(t, enc)

	err = enc.Encode(contracts.Frame{Seq: 0})
	require.ErrorIs(t, err, encoder.ErrNotWired)
}

// TestX264_Close_AlwaysSucceeds verifies Close() never errors on the stub.
func TestX264_Close_AlwaysSucceeds(t *testing.T) {
	enc, err := encoder.New("x264")
	require.NoError(t, err)
	require.NoError(t, enc.Close())
}
