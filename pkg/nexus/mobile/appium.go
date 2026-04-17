package mobile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"digital.vasic.helixqa/pkg/nexus"
)

// PlatformType names a mobile platform.
type PlatformType string

const (
	PlatformAndroid   PlatformType = "android"
	PlatformAndroidTV PlatformType = "androidtv"
	PlatformIOS       PlatformType = "ios"
)

// AppiumClient is a minimal Appium 2.0 WebDriver HTTP client. It covers
// the subset of commands required for Nexus session lifecycle, actions,
// and accessibility dumps. More advanced commands are added in later
// phases (gestures, file upload, sensor emulation).
type AppiumClient struct {
	baseURL string
	http    *http.Client

	mu        sync.Mutex
	sessionID string
}

// NewAppiumClient returns a client bound to the supplied Appium hub URL
// (usually http://127.0.0.1:4723). The http client is a simple http.Client
// with a 30-second default timeout; override via WithHTTPClient.
func NewAppiumClient(baseURL string) *AppiumClient {
	return &AppiumClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// WithHTTPClient lets tests inject a custom http.Client (for example the
// one returned by httptest.Server).
func (c *AppiumClient) WithHTTPClient(h *http.Client) *AppiumClient {
	c.http = h
	return c
}

// SessionID returns the current session id or empty when not connected.
func (c *AppiumClient) SessionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionID
}

// NewSession creates an Appium W3C session with the given capabilities
// and records the session id for follow-up commands.
func (c *AppiumClient) NewSession(ctx context.Context, caps Capabilities) error {
	body := map[string]any{
		"capabilities": map[string]any{
			"alwaysMatch": caps.toMap(),
		},
	}
	var resp sessionResponse
	if err := c.postJSON(ctx, "/session", body, &resp); err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	if resp.Value.SessionID == "" {
		return errors.New("appium: empty session id in response")
	}
	c.mu.Lock()
	c.sessionID = resp.Value.SessionID
	c.mu.Unlock()
	return nil
}

// DeleteSession ends the current session. Safe to call when no session is
// active.
func (c *AppiumClient) DeleteSession(ctx context.Context) error {
	sid := c.SessionID()
	if sid == "" {
		return nil
	}
	if err := c.deleteJSON(ctx, "/session/"+sid, nil); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	c.mu.Lock()
	c.sessionID = ""
	c.mu.Unlock()
	return nil
}

// FindElement issues a W3C findElement request and returns the element id.
// Strategy is one of xpath, id, accessibility id, class name, css selector.
func (c *AppiumClient) FindElement(ctx context.Context, strategy, value string) (string, error) {
	sid := c.SessionID()
	if sid == "" {
		return "", errors.New("appium: no session; call NewSession first")
	}
	body := map[string]string{"using": strategy, "value": value}
	var resp findElementResponse
	if err := c.postJSON(ctx, "/session/"+sid+"/element", body, &resp); err != nil {
		return "", fmt.Errorf("find element (%s=%s): %w", strategy, value, err)
	}
	// W3C returns the element id under the element-6066-11e4-a52e-4f735466cecf key.
	for _, v := range resp.Value {
		if v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("appium: no element id in response for %s=%s", strategy, value)
}

// Click sends a click command to the supplied element id.
func (c *AppiumClient) Click(ctx context.Context, elementID string) error {
	return c.postJSON(ctx, c.elPath(elementID, "click"), map[string]any{}, nil)
}

// SendKeys types text into the element.
func (c *AppiumClient) SendKeys(ctx context.Context, elementID, text string) error {
	return c.postJSON(ctx, c.elPath(elementID, "value"), map[string]any{"text": text}, nil)
}

// PageSource returns the current page source (XML for native, HTML for web).
func (c *AppiumClient) PageSource(ctx context.Context) (string, error) {
	sid := c.SessionID()
	if sid == "" {
		return "", errors.New("appium: no session")
	}
	var resp valueStringResponse
	if err := c.getJSON(ctx, "/session/"+sid+"/source", &resp); err != nil {
		return "", fmt.Errorf("page source: %w", err)
	}
	return resp.Value, nil
}

// Screenshot returns the current screen as PNG bytes.
func (c *AppiumClient) Screenshot(ctx context.Context) ([]byte, error) {
	sid := c.SessionID()
	if sid == "" {
		return nil, errors.New("appium: no session")
	}
	var resp valueStringResponse
	if err := c.getJSON(ctx, "/session/"+sid+"/screenshot", &resp); err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}
	return []byte(resp.Value), nil
}

// ExecuteScript runs a mobile: or driver-specific script command.
func (c *AppiumClient) ExecuteScript(ctx context.Context, script string, args any) (any, error) {
	sid := c.SessionID()
	if sid == "" {
		return nil, errors.New("appium: no session")
	}
	body := map[string]any{"script": script, "args": args}
	var resp valueAnyResponse
	if err := c.postJSON(ctx, "/session/"+sid+"/execute/sync", body, &resp); err != nil {
		return nil, fmt.Errorf("execute script %q: %w", script, err)
	}
	return resp.Value, nil
}

// Platform derives the PlatformType from a Capabilities snapshot. Useful
// for adapters that want to report nexus.Platform correctly.
func (c *AppiumClient) Platform(caps Capabilities) nexus.Platform {
	switch caps.Platform {
	case PlatformAndroid:
		return nexus.PlatformAndroidAppium
	case PlatformAndroidTV:
		return nexus.PlatformAndroidTVAppium
	case PlatformIOS:
		return nexus.PlatformIOSAppium
	default:
		return nexus.PlatformAndroidAppium
	}
}

// ----- HTTP helpers -----

func (c *AppiumClient) elPath(elID, action string) string {
	return "/session/" + c.SessionID() + "/element/" + elID + "/" + action
}

func (c *AppiumClient) postJSON(ctx context.Context, path string, body any, out any) error {
	return c.doJSON(ctx, http.MethodPost, path, body, out)
}
func (c *AppiumClient) deleteJSON(ctx context.Context, path string, out any) error {
	return c.doJSON(ctx, http.MethodDelete, path, nil, out)
}
func (c *AppiumClient) getJSON(ctx context.Context, path string, out any) error {
	return c.doJSON(ctx, http.MethodGet, path, nil, out)
}
func (c *AppiumClient) doJSON(ctx context.Context, method, path string, in, out any) error {
	var buf io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		buf = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, buf)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, string(raw))
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decode response: %w: body=%s", err, string(raw))
		}
	}
	return nil
}

// ----- W3C response shapes -----

type sessionResponse struct {
	Value struct {
		SessionID    string         `json:"sessionId"`
		Capabilities map[string]any `json:"capabilities"`
	} `json:"value"`
}

type findElementResponse struct {
	Value map[string]string `json:"value"`
}

type valueStringResponse struct {
	Value string `json:"value"`
}

type valueAnyResponse struct {
	Value any `json:"value"`
}
