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
	"image/png"
	"io"
	"net/http"
	"time"

	"encoding/base64"
)

// DreamSim is the tier-3 perceptual Comparator — a REST client against a
// Triton-hosted DreamSim (Sundar 2023) model. DreamSim is trained on
// human perceptual-similarity judgments and achieves ~96% agreement with
// human raters on structural-similarity tasks; see
// https://dreamsim-nights.github.io/ for the model.
//
// Why a REST client rather than embedding the model:
//
//   - DreamSim is a ViT-based model; running it inside the HelixQA Go
//     host would require either ONNX Runtime CGO bindings or a tensor
//     library port, both of which violate the CGO-free discipline.
//   - Triton Inference Server hosts the model on the GPU host (thinker.
//     local per OpenClawing4-Phase2-Kickoff.md §8) and exposes HTTP+gRPC
//     endpoints. The Go client only needs net/http + encoding/json.
//
// Wire format (Triton KServe v2):
//
//	POST {endpoint}/v2/models/{model}/infer
//	{
//	  "inputs": [
//	    {"name": "IMAGE_A", "datatype": "BYTES", "shape": [1],
//	     "data": ["<base64-encoded PNG>"]},
//	    {"name": "IMAGE_B", "datatype": "BYTES", "shape": [1],
//	     "data": ["<base64-encoded PNG>"]}
//	  ]
//	}
//	→ 200 OK {
//	  "outputs": [
//	    {"name": "SIMILARITY", "datatype": "FP32", "shape": [1],
//	     "data": [0.87]}
//	  ]
//	}
//
// Similarity in DreamSim is on [0, 1] where 1 = perceptually identical.
// The Comparator contract returns [-1, 1] — we map 2*raw-1 into the
// canonical range so 1→1, 0.5→0, 0→-1.
type DreamSim struct {
	// Endpoint is the base URL of the Triton server, e.g.
	// "http://thinker.local:8000". Required.
	Endpoint string

	// Model is the model name configured in Triton. Default:
	// "dreamsim".
	Model string

	// HTTPClient is the underlying transport; default is a
	// 200-millisecond client (matching the Phase 2 budget in
	// OpenClawing4.md §5.8).
	HTTPClient *http.Client
}

// NewDreamSim returns a tier-3 Comparator with the given endpoint.
// The model defaults to "dreamsim" and the HTTP timeout to 200 ms.
func NewDreamSim(endpoint string) *DreamSim {
	return &DreamSim{
		Endpoint: endpoint,
		Model:    "dreamsim",
		HTTPClient: &http.Client{
			Timeout: 200 * time.Millisecond,
		},
	}
}

// Sentinel errors specific to the DreamSim client.
var (
	ErrDreamSimEndpoint = errors.New("helixqa/perceptual/dreamsim: Endpoint not set")
	ErrDreamSimResponse = errors.New("helixqa/perceptual/dreamsim: unexpected Triton response shape")
)

// tritonInput / tritonOutput mirror the KServe v2 inference-request shape.
type tritonInput struct {
	Name     string   `json:"name"`
	Datatype string   `json:"datatype"`
	Shape    []int    `json:"shape"`
	Data     []string `json:"data"`
}

type tritonRequest struct {
	Inputs []tritonInput `json:"inputs"`
}

type tritonOutput struct {
	Name     string    `json:"name"`
	Datatype string    `json:"datatype"`
	Shape    []int     `json:"shape"`
	Data     []float64 `json:"data"`
}

type tritonResponse struct {
	Outputs []tritonOutput `json:"outputs"`
}

// Compare encodes a and b as PNGs, sends them to the configured Triton
// endpoint, and returns the DreamSim similarity mapped to [-1, 1].
func (d *DreamSim) Compare(ctx context.Context, a, b image.Image) (float64, error) {
	if a == nil || b == nil {
		return 0, ErrNilImage
	}
	if d.Endpoint == "" {
		return 0, ErrDreamSimEndpoint
	}
	model := d.Model
	if model == "" {
		model = "dreamsim"
	}
	client := d.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 200 * time.Millisecond}
	}

	encA, err := encodePNGBase64(a)
	if err != nil {
		return 0, fmt.Errorf("dreamsim: encode a: %w", err)
	}
	encB, err := encodePNGBase64(b)
	if err != nil {
		return 0, fmt.Errorf("dreamsim: encode b: %w", err)
	}

	body, err := json.Marshal(tritonRequest{
		Inputs: []tritonInput{
			{Name: "IMAGE_A", Datatype: "BYTES", Shape: []int{1}, Data: []string{encA}},
			{Name: "IMAGE_B", Datatype: "BYTES", Shape: []int{1}, Data: []string{encB}},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("dreamsim: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v2/models/%s/infer", d.Endpoint, model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("dreamsim: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("dreamsim: call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("dreamsim: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var out tritonResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, fmt.Errorf("dreamsim: decode response: %w", err)
	}

	similarity, err := extractSimilarity(out)
	if err != nil {
		return 0, err
	}
	// DreamSim raw output is in [0, 1]; canonical Comparator is [-1, 1].
	return 2*similarity - 1, nil
}

// extractSimilarity pulls the scalar similarity from the Triton response.
// Returns ErrDreamSimResponse on any shape mismatch.
func extractSimilarity(r tritonResponse) (float64, error) {
	for _, o := range r.Outputs {
		if o.Name == "SIMILARITY" || o.Name == "similarity" || o.Name == "output" {
			if len(o.Data) == 0 {
				return 0, fmt.Errorf("%w: empty SIMILARITY data", ErrDreamSimResponse)
			}
			return o.Data[0], nil
		}
	}
	// Fallback: first output with non-empty Data.
	for _, o := range r.Outputs {
		if len(o.Data) > 0 {
			return o.Data[0], nil
		}
	}
	return 0, fmt.Errorf("%w: no SIMILARITY output found", ErrDreamSimResponse)
}

// encodePNGBase64 encodes img as PNG and returns the base64-encoded
// bytes. Triton's BYTES datatype wraps arbitrary blobs in base64.
func encodePNGBase64(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// Compile-time guard.
var _ Comparator = (*DreamSim)(nil)
