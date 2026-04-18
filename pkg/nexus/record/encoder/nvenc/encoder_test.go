// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package nvenc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/record/encoder"
	_ "digital.vasic.helixqa/pkg/nexus/record/encoder/nvenc"
)

// TestNVENC_FactoryRegistered verifies the init() registers "nvenc" in the
// parent encoder factory.
func TestNVENC_FactoryRegistered(t *testing.T) {
	kinds := encoder.Kinds()
	assert.Contains(t, kinds, "nvenc", "nvenc must be registered via init()")
}

// TestNVENC_ProductionReturnsErrNotWired verifies the production stub returns
// ErrNotWired from Encode() in P5 (remote dispatch to thinker.local lands in
// P5.5).
func TestNVENC_ProductionReturnsErrNotWired(t *testing.T) {
	enc, err := encoder.New("nvenc")
	require.NoError(t, err)
	require.NotNil(t, enc)

	err = enc.Encode(contracts.Frame{Seq: 0})
	require.ErrorIs(t, err, encoder.ErrNotWired)
}

// TestNVENC_Close_AlwaysSucceeds verifies Close() never errors on the stub.
func TestNVENC_Close_AlwaysSucceeds(t *testing.T) {
	enc, err := encoder.New("nvenc")
	require.NoError(t, err)
	require.NoError(t, enc.Close())
}
