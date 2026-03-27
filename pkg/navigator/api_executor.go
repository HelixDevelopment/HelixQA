// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"digital.vasic.helixqa/pkg/detector"
)

// APIExecutor implements ActionExecutor for REST APIs by
// making HTTP requests. Type sends a POST with a JSON body;
// KeyPress issues a GET to baseURL+"/"+key; Screenshot GETs
// baseURL/health and returns the response body. Pointer-based
// UI methods (Click, Scroll, LongPress, Swipe, Back, Home)
// are no-ops.
type APIExecutor struct {
	baseURL  string
	client   *http.Client
	token    string
	runner   detector.CommandRunner
	mu       sync.Mutex
	lastResp []byte
}

// NewAPIExecutor creates an APIExecutor targeting baseURL.
func NewAPIExecutor(
	baseURL string,
	runner detector.CommandRunner,
) *APIExecutor {
	return &APIExecutor{
		baseURL: baseURL,
		client:  &http.Client{},
		runner:  runner,
	}
}

// SetToken sets a Bearer token that will be sent in the
// Authorization header on all requests.
func (a *APIExecutor) SetToken(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.token = token
}

// LastResponse returns the body of the most recent HTTP
// response received by Type or KeyPress.
func (a *APIExecutor) LastResponse() []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	cp := make([]byte, len(a.lastResp))
	copy(cp, a.lastResp)
	return cp
}

// do performs an HTTP request, stores the response body, and
// returns it. It sets the Authorization header when a token
// is configured.
func (a *APIExecutor) do(
	ctx context.Context, req *http.Request,
) ([]byte, error) {
	a.mu.Lock()
	token := a.token
	a.mu.Unlock()

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := a.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.lastResp = body
	a.mu.Unlock()

	return body, nil
}

// Type sends text as a JSON POST body to baseURL and stores
// the response.
func (a *APIExecutor) Type(
	ctx context.Context, text string,
) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		a.baseURL,
		strings.NewReader(text),
	)
	if err != nil {
		return fmt.Errorf("api type: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	_, err = a.do(ctx, req)
	return err
}

// KeyPress issues a GET request to baseURL+"/"+key, treating
// key as an endpoint path segment, and stores the response.
func (a *APIExecutor) KeyPress(
	ctx context.Context, key string,
) error {
	url := a.baseURL + "/" + key
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, url, nil,
	)
	if err != nil {
		return fmt.Errorf("api keypress: %w", err)
	}

	_, err = a.do(ctx, req)
	return err
}

// Screenshot GETs baseURL/health and returns the response
// body as bytes, capturing the current API state.
func (a *APIExecutor) Screenshot(
	ctx context.Context,
) ([]byte, error) {
	url := a.baseURL + "/health"
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, url, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("api screenshot: %w", err)
	}

	data, err := a.do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("api screenshot: %w", err)
	}
	return data, nil
}

// Click is not applicable for REST APIs — returns nil.
func (a *APIExecutor) Click(
	_ context.Context, _, _ int,
) error {
	return nil
}

// Scroll is not applicable for REST APIs — returns nil.
func (a *APIExecutor) Scroll(
	_ context.Context, _ string, _ int,
) error {
	return nil
}

// LongPress is not applicable for REST APIs — returns nil.
func (a *APIExecutor) LongPress(
	_ context.Context, _, _ int,
) error {
	return nil
}

// Swipe is not applicable for REST APIs — returns nil.
func (a *APIExecutor) Swipe(
	_ context.Context, _, _, _, _ int,
) error {
	return nil
}

// Back is not applicable for REST APIs — returns nil.
func (a *APIExecutor) Back(_ context.Context) error {
	return nil
}

// Home is not applicable for REST APIs — returns nil.
func (a *APIExecutor) Home(_ context.Context) error {
	return nil
}
