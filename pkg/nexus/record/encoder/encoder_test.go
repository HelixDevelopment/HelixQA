// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package encoder_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/record/encoder"
)

// testEncoder is a simple in-test Encoder for factory registration tests.
type testEncoder struct {
	encoded int
	closed  bool
}

func (e *testEncoder) Encode(_ contracts.Frame) error { e.encoded++; return nil }
func (e *testEncoder) Close() error                   { e.closed = true; return nil }

// TestEncoder_FactoryRegisterAndNew verifies that a custom factory registered
// under a test kind is discoverable via Kinds() and New() returns a non-nil
// Encoder.
func TestEncoder_FactoryRegisterAndNew(t *testing.T) {
	const kind = "test-encoder-kind"
	enc := &testEncoder{}
	encoder.Register(kind, func() encoder.Encoder { return enc })

	kinds := encoder.Kinds()
	assert.Contains(t, kinds, kind, "registered kind must appear in Kinds()")

	got, err := encoder.New(kind)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.NoError(t, got.Encode(contracts.Frame{Seq: 1}))
	assert.Equal(t, 1, enc.encoded)
}

// TestEncoder_UnknownKind verifies that New() returns a descriptive error for
// an unregistered kind.
func TestEncoder_UnknownKind(t *testing.T) {
	_, err := encoder.New("definitely-not-registered-xyz")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "definitely-not-registered-xyz")
}
