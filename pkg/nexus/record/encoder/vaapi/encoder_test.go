// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package vaapi_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/record/encoder"
	_ "digital.vasic.helixqa/pkg/nexus/record/encoder/vaapi"
)

// TestVAAPI_FactoryRegistered verifies the init() registers "vaapi" in the
// parent encoder factory.
func TestVAAPI_FactoryRegistered(t *testing.T) {
	kinds := encoder.Kinds()
	assert.Contains(t, kinds, "vaapi", "vaapi must be registered via init()")
}

// TestVAAPI_ProductionReturnsErrNotWired verifies the production stub returns
// ErrNotWired from Encode() in P5.
func TestVAAPI_ProductionReturnsErrNotWired(t *testing.T) {
	enc, err := encoder.New("vaapi")
	require.NoError(t, err)
	require.NotNil(t, enc)

	err = enc.Encode(contracts.Frame{Seq: 0})
	require.ErrorIs(t, err, encoder.ErrNotWired)
}

// TestVAAPI_Close_AlwaysSucceeds verifies Close() never errors on the stub.
func TestVAAPI_Close_AlwaysSucceeds(t *testing.T) {
	enc, err := encoder.New("vaapi")
	require.NoError(t, err)
	require.NoError(t, enc.Close())
}
