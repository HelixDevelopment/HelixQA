// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package perceptual

import (
	"context"
	"errors"
	"fmt"
	"image"
	"net/http"
	"time"

	"digital.vasic.helixqa/pkg/gpu/infer"
)

// LPIPS is the HelixQA tier-3 fallback when DreamSim is not
// deployed: Learned Perceptual Image Patch Similarity (Zhang et
// al. 2018). LPIPS is older than DreamSim but more widely
// deployed — the two cover the same perceptual-similarity niche
// and satisfy the same Comparator contract.
//
// Since M60, LPIPS delegates its HTTP wire to pkg/gpu/infer (the
// generic Triton KServe v2 client). The LPIPS-specific logic is
// just the distance→similarity mapping at the end.
//
// LPIPS output is a DISTANCE in [0, ~2] where 0 = perceptually
// identical. The client inverts this to the canonical Comparator
// [-1, 1] similarity via: similarity = 1 - 2·(distance/maxDist).
type LPIPS struct {
	// Endpoint is the Triton server URL.
	Endpoint string

	// Model defaults to "lpips".
	Model string

	// HTTPClient default 200 ms timeout (same as DreamSim).
	HTTPClient *http.Client

	// MaxDistance clamps the distance → similarity mapping. LPIPS
	// distances above this value map to similarity = -1 (fully
	// dissimilar). Default 1.0 — empirically above this point the
	// images are visually unrelated and the metric saturates.
	MaxDistance float64
}

// NewLPIPS returns a tier-3 fallback Comparator with the given
// endpoint.
func NewLPIPS(endpoint string) *LPIPS {
	return &LPIPS{Endpoint: endpoint}
}

// Sentinel errors — preserved from the pre-M60 shape for errors.Is
// compatibility. Reuses ErrNilImage from ssim.go.
var (
	ErrLPIPSEndpoint = errors.New("helixqa/perceptual/lpips: Endpoint not set")
	ErrLPIPSResponse = errors.New("helixqa/perceptual/lpips: unexpected Triton response shape")
)

// Compare sends (a, b) to the LPIPS sidecar via pkg/gpu/infer and
// returns the canonical similarity in [-1, 1].
func (l *LPIPS) Compare(ctx context.Context, a, b image.Image) (float64, error) {
	if a == nil || b == nil {
		return 0, ErrNilImage
	}
	if l.Endpoint == "" {
		return 0, ErrLPIPSEndpoint
	}
	model := l.Model
	if model == "" {
		model = "lpips"
	}
	httpClient := l.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 200 * time.Millisecond}
	}
	maxDist := l.MaxDistance
	if maxDist <= 0 {
		maxDist = 1.0
	}

	encA, err := encodePNGBase64(a)
	if err != nil {
		return 0, fmt.Errorf("lpips: encode a: %w", err)
	}
	encB, err := encodePNGBase64(b)
	if err != nil {
		return 0, fmt.Errorf("lpips: encode b: %w", err)
	}

	client := &infer.Client{
		Endpoint:   l.Endpoint,
		HTTPClient: httpClient,
	}
	resp, err := client.Infer(ctx, infer.Request{
		Model: model,
		Inputs: []infer.Input{
			{Name: "IMAGE_A", Datatype: "BYTES", Shape: []int{1}, StringData: []string{encA}},
			{Name: "IMAGE_B", Datatype: "BYTES", Shape: []int{1}, StringData: []string{encB}},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("lpips: %w", err)
	}

	distance, err := extractLPIPSDistance(resp)
	if err != nil {
		return 0, err
	}
	return distanceToSimilarity(distance, maxDist), nil
}

// distanceToSimilarity maps LPIPS's [0, ∞) distance range to the
// canonical [-1, 1] Comparator similarity:
//
//	distance = 0        → similarity =  1
//	distance = maxDist  → similarity = -1
//	distance > maxDist  → similarity = -1 (clamped)
//	distance < 0        → similarity =  1 (defensive)
func distanceToSimilarity(distance, maxDist float64) float64 {
	if distance <= 0 {
		return 1
	}
	if distance >= maxDist {
		return -1
	}
	return 1 - 2*(distance/maxDist)
}

// extractLPIPSDistance pulls a scalar distance from the response.
// Tolerates "DISTANCE", "distance", "SIMILARITY" (pre-inverted
// sidecars — we undo the inversion back to distance), or the
// first output with numeric data.
func extractLPIPSDistance(r infer.Response) (float64, error) {
	for _, o := range r.Outputs {
		switch o.Name {
		case "DISTANCE", "distance":
			if v, ok := firstFloat(o); ok {
				return v, nil
			}
			return 0, fmt.Errorf("%w: empty DISTANCE", ErrLPIPSResponse)
		case "SIMILARITY", "similarity":
			// Sidecar pre-inverted; reverse: similarity = 1 -
			// 2·(dist/maxDist), so dist = (1 - similarity)/2 at
			// maxDist=1. The LPIPS.Compare caller re-applies its
			// own maxDist after this.
			if v, ok := firstFloat(o); ok {
				return (1 - v) / 2, nil
			}
			return 0, fmt.Errorf("%w: empty SIMILARITY", ErrLPIPSResponse)
		}
	}
	// Fallback: first output with numeric data.
	for _, o := range r.Outputs {
		if v, ok := firstFloat(o); ok {
			return v, nil
		}
	}
	return 0, fmt.Errorf("%w: no DISTANCE output found", ErrLPIPSResponse)
}

// Compile-time guard.
var _ Comparator = (*LPIPS)(nil)
