// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cheaper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/png" // register PNG decoder

	"digital.vasic.helixqa/pkg/llm"
)

// BridgeProvider wraps a VisionProvider so that it satisfies the llm.Provider
// interface. This allows cheaper vision backends to participate in the existing
// autonomous pipeline without modification. Chat is not supported — only Vision
// calls are forwarded to the underlying VisionProvider.
type BridgeProvider struct {
	vp VisionProvider
}

// NewBridgeProvider creates a BridgeProvider that delegates Vision calls to vp.
// The returned value implements llm.Provider and is safe for concurrent use as
// long as vp is safe for concurrent use.
func NewBridgeProvider(vp VisionProvider) *BridgeProvider {
	return &BridgeProvider{vp: vp}
}

// Chat is not supported by cheaper vision providers.
// It always returns an error so that callers relying on llm.Provider can detect
// the capability mismatch at runtime instead of silently getting no-op output.
func (b *BridgeProvider) Chat(
	_ context.Context,
	_ []llm.Message,
) (*llm.Response, error) {
	return nil, errors.New(
		"cheaper vision provider does not support chat",
	)
}

// Vision decodes imageBytes as a PNG image, forwards it to the underlying
// VisionProvider.Analyze, and converts the VisionResult to an llm.Response.
// Only the Text and Model fields of VisionResult are mapped; token counts are
// left as zero because cheaper providers do not expose token usage.
func (b *BridgeProvider) Vision(
	ctx context.Context,
	imageBytes []byte,
	prompt string,
) (*llm.Response, error) {
	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, fmt.Errorf(
			"cheaper bridge: failed to decode image: %w", err,
		)
	}

	result, err := b.vp.Analyze(ctx, img, prompt)
	if err != nil {
		return nil, fmt.Errorf(
			"cheaper bridge: provider %q failed: %w", b.vp.Name(), err,
		)
	}

	return &llm.Response{
		Content: result.Text,
		Model:   result.Model,
	}, nil
}

// Name returns the bridge's canonical identifier, composed of the
// "cheaper-" prefix and the underlying provider's name.
func (b *BridgeProvider) Name() string {
	return "cheaper-" + b.vp.Name()
}

// SupportsVision always returns true because BridgeProvider exists solely to
// expose vision analysis through the llm.Provider interface.
func (b *BridgeProvider) SupportsVision() bool {
	return true
}
