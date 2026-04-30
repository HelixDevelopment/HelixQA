// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Tesseract HTTP client — sibling to whisper_client.go, talking to
// the qa-tesseract container defined in
// scripts/qa_reprocess/containers/tesseract/server.py (Tesseract OCR
// engine, default port 7071, loopback-only bind).
//
// Constitution §11.4 captured-evidence rule: /ocr returns RAW
// Tesseract output (the response is the OCR engine's verbatim
// emission — no client-side post-processing). /ocr-video returns
// per-frame OCR keyed by 1-based zero-padded frame index plus
// metadata (fps, lang, psm) so a reviewer can re-run the same
// extraction deterministically.
//
// Issues.md E2 Phase 2: Go binding for the tesseract container.
//
// NOTE on package name: this file lives in `pkg/audio` next to
// the Whisper client because both are QA-reprocess HTTP-container
// clients sharing identical deployment topology (loopback bind +
// /health probe + multipart upload). The package name is
// deliberately broad — a future refactor may rename it to
// `pkg/qareprocess` or `pkg/qaservices` once a third sibling
// (e.g. ffprobe-as-a-service) lands. Until then, co-location
// keeps cascade overhead low.

package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// DefaultTesseractBaseURL matches the loopback bind documented in
// docs/changelogs/1.1.5-dev-0.0.3.md ("Loopback-only port binds —
// Containers exposed at 127.0.0.1:7070/7071 — never publicly").
const DefaultTesseractBaseURL = "http://127.0.0.1:7071"

// TesseractHealthResponse mirrors the /health JSON the server returns
// (server.py lines 57-72).
type TesseractHealthResponse struct {
	Status           string   `json:"status"`
	TesseractVersion string   `json:"tesseract_version"`
	Langs            []string `json:"langs"`
	DefaultLang      string   `json:"default_lang"`
	DefaultPSM       int      `json:"default_psm"`
}

// OCRVideoResult mirrors the /ocr-video JSON. Frames maps
// "0001"→"OCR text" (1-based zero-padded 4-digit keys, server.py
// line 18).
type OCRVideoResult struct {
	FPS        float64           `json:"fps"`
	Lang       string            `json:"lang"`
	PSM        int               `json:"psm"`
	Frames     map[string]string `json:"frames"`
	FrameCount int               `json:"frame_count"`
}

// TesseractClient talks to the qa-tesseract container. Construct
// with NewTesseractClient. Safe for concurrent use.
type TesseractClient struct {
	baseURL string
	http    *http.Client
}

// NewTesseractClient returns a TesseractClient for the given base
// URL (no trailing slash). Pass an empty string for
// DefaultTesseractBaseURL. The default request timeout is 15
// minutes — /ocr-video on a long video can run multiple minutes
// of CPU. Override via WithHTTPClient if needed.
func NewTesseractClient(baseURL string) *TesseractClient {
	if baseURL == "" {
		baseURL = DefaultTesseractBaseURL
	}
	return &TesseractClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 15 * time.Minute},
	}
}

// WithHTTPClient overrides the default http.Client. Useful for
// httptest or to tighten timeouts for /ocr (single-image) callers.
func (c *TesseractClient) WithHTTPClient(client *http.Client) *TesseractClient {
	if client != nil {
		c.http = client
	}
	return c
}

// Health probes /health and returns the parsed response.
func (c *TesseractClient) Health(ctx context.Context) (*TesseractHealthResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return nil, fmt.Errorf("tesseract: build /health request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tesseract: /health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("tesseract: /health returned %d: %s", resp.StatusCode, string(body))
	}
	var out TesseractHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("tesseract: decode /health: %w", err)
	}
	return &out, nil
}

// OCROptions are the optional knobs for OCR / OCRVideo. Zero values
// mean "use server defaults" (TESSERACT_LANG / TESSERACT_PSM env vars).
type OCROptions struct {
	// Lang is the Tesseract language identifier (e.g. "eng",
	// "eng+rus"). Empty = server default. The server's `+`-decode
	// normalisation means "eng+rus" reaches tesseract correctly
	// regardless of URL encoding (server.py lines 84-89).
	Lang string
	// PSM is the Tesseract page-segmentation-mode integer 0..13.
	// Zero (the Go zero value) is also a valid PSM ("orientation
	// and script detection only"); we therefore use a separate
	// SetPSM bool to distinguish "PSM=0 explicit" from "no PSM
	// override". This avoids the surprising-default trap.
	PSM    int
	SetPSM bool
	// FPS is /ocr-video only — frame sampling rate (frames per
	// second of source video). Zero means "use server default
	// of 1.0 fps".
	FPS float64
}

// OCR uploads the image at imagePath to /ocr and returns the raw
// Tesseract output as a string. Per server.py the response is
// `text/plain; charset=utf-8` — the client does NOT post-process
// (no trim, no whitespace collapse) so callers see exactly what
// Tesseract emitted.
func (c *TesseractClient) OCR(ctx context.Context, imagePath string, opts OCROptions) (string, error) {
	body, contentType, err := streamSingleFileMultipart("image", imagePath)
	if err != nil {
		return "", err
	}

	endpoint := c.baseURL + "/ocr" + buildOCRQuery(opts, false)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return "", fmt.Errorf("tesseract: build /ocr request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("tesseract: /ocr: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20)) // 16 MiB cap
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tesseract: /ocr returned %d: %s", resp.StatusCode, string(respBody))
	}
	return string(respBody), nil
}

// OCRVideo uploads the video at videoPath to /ocr-video and
// returns the parsed OCRVideoResult with per-frame OCR.
func (c *TesseractClient) OCRVideo(ctx context.Context, videoPath string, opts OCROptions) (*OCRVideoResult, error) {
	body, contentType, err := streamSingleFileMultipart("video", videoPath)
	if err != nil {
		return nil, err
	}

	endpoint := c.baseURL + "/ocr-video" + buildOCRQuery(opts, true)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("tesseract: build /ocr-video request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tesseract: /ocr-video: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("tesseract: /ocr-video returned %d: %s", resp.StatusCode, string(body))
	}
	var out OCRVideoResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("tesseract: decode /ocr-video: %w", err)
	}
	return &out, nil
}

// buildOCRQuery composes the query string for /ocr or /ocr-video.
// videoMode = true also emits fps when opts.FPS > 0.
func buildOCRQuery(opts OCROptions, videoMode bool) string {
	q := url.Values{}
	if opts.Lang != "" {
		q.Set("lang", opts.Lang)
	}
	if opts.SetPSM {
		q.Set("psm", strconv.Itoa(opts.PSM))
	}
	if videoMode && opts.FPS > 0 {
		q.Set("fps", strconv.FormatFloat(opts.FPS, 'g', -1, 64))
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}
