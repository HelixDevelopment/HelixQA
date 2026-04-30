// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package audio provides HTTP clients for the QA-reprocess audio
// containers — currently the Whisper ASR server defined in
// scripts/qa_reprocess/containers/whisper/server.py (faster-whisper
// backend, default port 7070, loopback-only bind).
//
// Constitution §11.4 captured-evidence rule: every Transcribe
// response carries the structured Result (text + per-segment
// timing + per-word probability + SRT) so a human reviewer can
// verify against the source video frame-by-frame. The client
// surfaces these fields verbatim — it does NOT reduce the response
// to a string.
//
// Issues.md E2 Phase 1: Go binding for the whisper container.
package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// DefaultWhisperBaseURL matches the loopback bind documented in
// docs/changelogs/1.1.5-dev-0.0.3.md ("Loopback-only port binds —
// Containers exposed at 127.0.0.1:7070/7071 — never publicly").
const DefaultWhisperBaseURL = "http://127.0.0.1:7070"

// HealthResponse mirrors the /health JSON the server returns.
// Fields are documented at server.py lines 88-99.
type HealthResponse struct {
	Status       string   `json:"status"`
	Backend      string   `json:"backend"`
	DefaultModel string   `json:"default_model"`
	LoadedModels []string `json:"loaded_models"`
	ComputeType  string   `json:"compute_type"`
	ModelsDir    string   `json:"models_dir"`
}

// Word is a single word with timing + confidence — the §11.4
// captured evidence for ASR. probability comes from
// faster-whisper's WordSegment.probability.
type Word struct {
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Word        string  `json:"word"`
	Probability float64 `json:"probability"`
}

// Segment is one ASR segment with confidence stats. avg_logprob /
// no_speech_prob / compression_ratio together let a reviewer flag
// hallucinations without listening to every clip.
type Segment struct {
	ID               int     `json:"id"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	AvgLogProb       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
	Words            []Word  `json:"words"`
}

// TranscribeResult is the full /transcribe JSON. The SRT field is
// the same content as Segments rendered as SubRip — useful for
// dropping straight into a video player as an external track for
// reviewer playback.
type TranscribeResult struct {
	Model               string    `json:"model"`
	Backend             string    `json:"backend"`
	Language            string    `json:"language"`
	LanguageProbability float64   `json:"language_probability"`
	Duration            float64   `json:"duration"`
	Text                string    `json:"text"`
	Segments            []Segment `json:"segments"`
	SRT                 string    `json:"srt"`
	InputFilename       string    `json:"input_filename"`
	InputSizeBytes      int64     `json:"input_size_bytes"`
}

// WhisperClient talks to the qa-whisper container. Construct with
// NewWhisperClient. Safe for concurrent use — the underlying
// http.Client handles request multiplexing.
type WhisperClient struct {
	baseURL string
	http    *http.Client
}

// NewWhisperClient returns a WhisperClient for the given base URL
// (no trailing slash). Pass an empty string for DefaultWhisperBaseURL.
// The default request timeout is 30 minutes — transcription of long
// videos is slow on CPU-only int8 quantization. Override via
// WithHTTPClient if you need a different timeout.
func NewWhisperClient(baseURL string) *WhisperClient {
	if baseURL == "" {
		baseURL = DefaultWhisperBaseURL
	}
	return &WhisperClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Minute},
	}
}

// WithHTTPClient overrides the default http.Client. Use this when
// you need a custom timeout or transport (e.g. for httptest).
func (c *WhisperClient) WithHTTPClient(client *http.Client) *WhisperClient {
	if client != nil {
		c.http = client
	}
	return c
}

// Health probes /health. Returns the parsed response or an error
// if the container is unreachable, returns non-200, or the body
// fails to parse as the documented schema.
func (c *WhisperClient) Health(ctx context.Context) (*HealthResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return nil, fmt.Errorf("whisper: build /health request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("whisper: /health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("whisper: /health returned %d: %s", resp.StatusCode, string(body))
	}
	var out HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("whisper: decode /health: %w", err)
	}
	return &out, nil
}

// TranscribeOptions are the optional knobs for Transcribe. Zero
// values mean "use server defaults" (DEFAULT_MODEL, language=auto).
type TranscribeOptions struct {
	// Model is the faster-whisper model name (e.g. "base", "medium",
	// "large-v3"). Empty = server default.
	Model string
	// Language is the ISO-639-1 code (e.g. "en", "ru") or "auto".
	// Empty = server default ("auto" if unset on server side).
	Language string
}

// Transcribe uploads the file at path to /transcribe and returns
// the full structured result. The mime part is named "video" to
// match the server's primary expected name (server.py line 109);
// the server also accepts "audio" but using one consistent name
// keeps debugging simpler.
//
// The context controls the upload + transcription time budget.
// For long videos, plan for several minutes of server CPU time on
// the int8 backend.
func (c *WhisperClient) Transcribe(ctx context.Context, path string, opts TranscribeOptions) (*TranscribeResult, error) {
	body, contentType, err := streamSingleFileMultipart("video", path)
	if err != nil {
		// streamSingleFileMultipart returns an "audio:" prefix; rewrap
		// for caller debuggability (whisper-specific error path).
		return nil, fmt.Errorf("whisper: %w", err)
	}

	endpoint := c.baseURL + "/transcribe"
	if opts.Model != "" || opts.Language != "" {
		q := url.Values{}
		if opts.Model != "" {
			q.Set("model", opts.Model)
		}
		if opts.Language != "" {
			q.Set("language", opts.Language)
		}
		endpoint += "?" + q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("whisper: build /transcribe request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("whisper: /transcribe: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("whisper: /transcribe returned %d: %s", resp.StatusCode, string(body))
	}
	var out TranscribeResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("whisper: decode /transcribe: %w", err)
	}
	return &out, nil
}
