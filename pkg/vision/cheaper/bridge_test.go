// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cheaper

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/llm"
)

// visionStub is a VisionProvider that returns a configurable VisionResult or
// error. It is distinct from the stubProvider in registry_test.go so that the
// bridge tests can control Text and Model without modifying the shared stub.
type visionStub struct {
	name   string
	result *VisionResult
	err    error
}

func (v *visionStub) Name() string { return v.name }

func (v *visionStub) Analyze(
	_ context.Context,
	_ image.Image,
	_ string,
) (*VisionResult, error) {
	if v.err != nil {
		return nil, v.err
	}
	return v.result, nil
}

func (v *visionStub) HealthCheck(_ context.Context) error { return nil }

func (v *visionStub) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{}
}

func (v *visionStub) GetCostEstimate(_, _ int) float64 { return 0 }

// encodePNG returns the PNG-encoded bytes of img. It panics on error so that
// test setup failures surface immediately rather than being silently ignored.
func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encodePNG: %v", err)
	}
	return buf.Bytes()
}

// newTestImage returns a 4×4 RGBA image filled with an opaque red colour.
func newTestImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, red)
		}
	}
	return img
}

// TestBridge_Chat_ReturnsError verifies that calling Chat on a BridgeProvider
// always returns a descriptive error and a nil response.
func TestBridge_Chat_ReturnsError(t *testing.T) {
	stub := &visionStub{name: "stub"}
	bridge := NewBridgeProvider(stub)

	resp, err := bridge.Chat(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "hello"},
	})

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cheaper vision provider does not support chat")
}

// TestBridge_Vision_Success verifies the happy path: PNG bytes are decoded,
// forwarded to the underlying VisionProvider, and the VisionResult is converted
// to an llm.Response with matching Content and Model.
func TestBridge_Vision_Success(t *testing.T) {
	stub := &visionStub{
		name: "test-provider",
		result: &VisionResult{
			Text:  "detected: login button",
			Model: "ui-tars-7b",
		},
	}
	bridge := NewBridgeProvider(stub)

	pngBytes := encodePNG(t, newTestImage())

	resp, err := bridge.Vision(context.Background(), pngBytes, "what do you see?")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "detected: login button", resp.Content)
	assert.Equal(t, "ui-tars-7b", resp.Model)
	// Token counts are not reported by cheaper providers.
	assert.Equal(t, 0, resp.InputTokens)
	assert.Equal(t, 0, resp.OutputTokens)
}

// TestBridge_Vision_InvalidImage verifies that Vision returns an error when the
// supplied bytes are not a valid image (e.g. random / corrupted data).
func TestBridge_Vision_InvalidImage(t *testing.T) {
	stub := &visionStub{name: "stub"}
	bridge := NewBridgeProvider(stub)

	garbage := []byte("this is not an image at all \x00\xff\xfe")

	resp, err := bridge.Vision(context.Background(), garbage, "what do you see?")

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cheaper bridge: failed to decode image")
}

// TestBridge_Vision_ProviderError verifies that an error from the underlying
// VisionProvider is wrapped and returned to the caller.
func TestBridge_Vision_ProviderError(t *testing.T) {
	providerErr := errors.New("model inference timeout")
	stub := &visionStub{name: "flaky", err: providerErr}
	bridge := NewBridgeProvider(stub)

	pngBytes := encodePNG(t, newTestImage())

	resp, err := bridge.Vision(context.Background(), pngBytes, "any prompt")

	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cheaper bridge: provider \"flaky\" failed")
	assert.ErrorIs(t, err, providerErr)
}

// TestBridge_Name verifies that Name returns "cheaper-" prepended to the
// underlying provider's name.
func TestBridge_Name(t *testing.T) {
	stub := &visionStub{name: "glm4v"}
	bridge := NewBridgeProvider(stub)

	assert.Equal(t, "cheaper-glm4v", bridge.Name())
}

// TestBridge_SupportsVision verifies that SupportsVision always returns true.
func TestBridge_SupportsVision(t *testing.T) {
	stub := &visionStub{name: "showui"}
	bridge := NewBridgeProvider(stub)

	assert.True(t, bridge.SupportsVision())
}

// TestBridge_ImplementsLLMProvider is a compile-time assertion that
// *BridgeProvider satisfies the llm.Provider interface. If the interface changes
// and the bridge no longer conforms, this test file will fail to compile.
func TestBridge_ImplementsLLMProvider(t *testing.T) {
	stub := &visionStub{name: "any"}
	var _ llm.Provider = NewBridgeProvider(stub)
}
