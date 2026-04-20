// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package perceptual

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"time"
)

// LPIPS is the HelixQA tier-3 fallback when DreamSim is not
// deployed: Learned Perceptual Image Patch Similarity (Zhang et
// al. 2018, "The Unreasonable Effectiveness of Deep Features as a
// Perceptual Metric"). LPIPS is older than DreamSim but more
// widely deployed — the two cover the same
// "human-perceptual-similarity" niche and satisfy the same
// Comparator contract.
//
// Wire format (Triton KServe v2, identical to DreamSim.Compare so
// sidecars can multiplex both models on the same endpoint):
//
//	POST {endpoint}/v2/models/{model}/infer
//	  inputs:
//	    IMAGE_A: BYTES shape=[1] data=<base64 PNG>
//	    IMAGE_B: BYTES shape=[1] data=<base64 PNG>
//	→ 200 OK outputs:
//	    DISTANCE: FP32 shape=[1] data=[<perceptual distance>]
//
// LPIPS output is a DISTANCE (not a similarity) in [0, ~2] where
// 0 = perceptually identical and larger = more different. The
// client inverts this to the canonical Comparator [-1, 1]
// similarity via: similarity = 1 - min(1, distance).
type LPIPS struct {
	// Endpoint is the base URL of the Triton server.
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

// Sentinel errors specific to LPIPS. Reuses ErrNilImage from
// ssim.go / dreamsim.go.
var (
	ErrLPIPSEndpoint = errors.New("helixqa/perceptual/lpips: Endpoint not set")
	ErrLPIPSResponse = errors.New("helixqa/perceptual/lpips: unexpected Triton response shape")
)

// Compare sends (a, b) to the LPIPS sidecar and returns the
// canonical similarity in [-1, 1]. 1 = identical, 0 = halfway,
// -1 = maximally dissimilar (or beyond MaxDistance).
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
	client := l.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 200 * time.Millisecond}
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

	body, err := json.Marshal(lpipsRequest{
		Inputs: []lpipsInput{
			{Name: "IMAGE_A", Datatype: "BYTES", Shape: []int{1}, Data: []string{encA}},
			{Name: "IMAGE_B", Datatype: "BYTES", Shape: []int{1}, Data: []string{encB}},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("lpips: marshal: %w", err)
	}

	url := fmt.Sprintf("%s/v2/models/%s/infer", l.Endpoint, model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("lpips: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("lpips: call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("lpips: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var out lpipsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, fmt.Errorf("lpips: decode: %w", err)
	}

	distance, err := extractLPIPSDistance(out)
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
//
// Negative distance (impossible but defensively handled) → 1.
func distanceToSimilarity(distance, maxDist float64) float64 {
	if distance <= 0 {
		return 1
	}
	if distance >= maxDist {
		return -1
	}
	return 1 - 2*(distance/maxDist)
}

// extractLPIPSDistance pulls the scalar distance from the Triton
// response. Tolerates "DISTANCE", "distance", "similarity" (in
// case the sidecar already inverted), or the first non-empty
// output.
func extractLPIPSDistance(r lpipsResponse) (float64, error) {
	for _, o := range r.Outputs {
		switch o.Name {
		case "DISTANCE", "distance":
			if len(o.Data) == 0 {
				return 0, fmt.Errorf("%w: empty DISTANCE", ErrLPIPSResponse)
			}
			return o.Data[0], nil
		case "SIMILARITY", "similarity":
			// Sidecar pre-inverted — reverse the mapping so the
			// caller still receives a distance.
			if len(o.Data) == 0 {
				return 0, fmt.Errorf("%w: empty SIMILARITY", ErrLPIPSResponse)
			}
			// similarity = 1 - 2*(dist/maxDist)  →  distance =
			// maxDist*(1-similarity)/2; default maxDist=1 here, the
			// caller-configured maxDist gets reapplied later.
			return (1 - o.Data[0]) / 2, nil
		}
	}
	for _, o := range r.Outputs {
		if len(o.Data) > 0 {
			return o.Data[0], nil
		}
	}
	return 0, fmt.Errorf("%w: no DISTANCE output found", ErrLPIPSResponse)
}

// ---------------------------------------------------------------------------
// Wire structs — intentionally local (not reused from dreamsim.go) so
// sidecars can rename fields independently.
// ---------------------------------------------------------------------------

type lpipsInput struct {
	Name     string   `json:"name"`
	Datatype string   `json:"datatype"`
	Shape    []int    `json:"shape"`
	Data     []string `json:"data"`
}

type lpipsRequest struct {
	Inputs []lpipsInput `json:"inputs"`
}

type lpipsOutput struct {
	Name     string    `json:"name"`
	Datatype string    `json:"datatype"`
	Shape    []int     `json:"shape"`
	Data     []float64 `json:"data"`
}

type lpipsResponse struct {
	Outputs []lpipsOutput `json:"outputs"`
}

// encodePNGBase64 is shared with dreamsim.go (same package).

// Compile-time guard.
var _ Comparator = (*LPIPS)(nil)
