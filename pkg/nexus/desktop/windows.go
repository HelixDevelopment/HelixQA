package desktop

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
)

// WindowsEngine drives Microsoft's WinAppDriver over HTTP. Targeted at
// UWP / WPF / WinForms apps. The driver defaults to
// http://127.0.0.1:4723 (the official WinAppDriver port) and never
// binds to 0.0.0.0.
type WindowsEngine struct {
	baseURL string
	http    *http.Client

	mu        sync.Mutex
	sessionID string
}

// NewWindowsEngine returns an engine bound to the supplied base URL.
func NewWindowsEngine(baseURL string) *WindowsEngine {
	return &WindowsEngine{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// WithHTTPClient injects a test-owned http.Client.
func (w *WindowsEngine) WithHTTPClient(c *http.Client) *WindowsEngine {
	w.http = c
	return w
}

// Platform returns PlatformWindows.
func (*WindowsEngine) Platform() Platform { return PlatformWindows }

// Launch opens an app session for the given exe path or UWP app id.
func (w *WindowsEngine) Launch(ctx context.Context, appPath string, args []string) error {
	caps := map[string]any{
		"capabilities": map[string]any{
			"firstMatch": []map[string]any{
				{
					"appium:app":   appPath,
					"platformName": "Windows",
				},
			},
		},
	}
	if len(args) > 0 {
		caps["capabilities"].(map[string]any)["firstMatch"].([]map[string]any)[0]["appium:appArguments"] = strings.Join(args, " ")
	}
	var resp struct {
		Value struct {
			SessionID string `json:"sessionId"`
		} `json:"value"`
	}
	if err := w.doJSON(ctx, http.MethodPost, "/session", caps, &resp); err != nil {
		return fmt.Errorf("launch: %w", err)
	}
	w.mu.Lock()
	w.sessionID = resp.Value.SessionID
	w.mu.Unlock()
	return nil
}

// Attach binds to an existing session id.
func (w *WindowsEngine) Attach(_ context.Context, identifier string) error {
	if identifier == "" {
		return errors.New("windows: empty session id")
	}
	w.mu.Lock()
	w.sessionID = identifier
	w.mu.Unlock()
	return nil
}

// Close ends the current session.
func (w *WindowsEngine) Close(ctx context.Context) error {
	sid := w.session()
	if sid == "" {
		return nil
	}
	if err := w.doJSON(ctx, http.MethodDelete, "/session/"+sid, nil, nil); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	w.mu.Lock()
	w.sessionID = ""
	w.mu.Unlock()
	return nil
}

// FindByName locates an element by accessibility name.
func (w *WindowsEngine) FindByName(ctx context.Context, name string) (Element, error) {
	return w.find(ctx, "name", name)
}

// FindByRole locates the first element whose class matches role.
func (w *WindowsEngine) FindByRole(ctx context.Context, role string) (Element, error) {
	return w.find(ctx, "class name", role)
}

// Click performs a click.
func (w *WindowsEngine) Click(ctx context.Context, el Element) error {
	return w.doJSON(ctx, http.MethodPost, w.elPath(el.Handle, "click"), map[string]any{}, nil)
}

// Type enters text.
func (w *WindowsEngine) Type(ctx context.Context, el Element, text string) error {
	return w.doJSON(ctx, http.MethodPost, w.elPath(el.Handle, "value"), map[string]any{"text": text}, nil)
}

// Screenshot captures the session's window as a PNG buffer.
func (w *WindowsEngine) Screenshot(ctx context.Context) ([]byte, error) {
	sid := w.session()
	if sid == "" {
		return nil, errors.New("windows: no session")
	}
	var resp struct {
		Value string `json:"value"`
	}
	if err := w.doJSON(ctx, http.MethodGet, "/session/"+sid+"/screenshot", nil, &resp); err != nil {
		return nil, err
	}
	return []byte(resp.Value), nil
}

// PickMenu navigates a menu path such as ["File", "Open"].
func (w *WindowsEngine) PickMenu(ctx context.Context, path []string) error {
	for _, item := range path {
		el, err := w.FindByName(ctx, item)
		if err != nil {
			return fmt.Errorf("menu %s: %w", item, err)
		}
		if err := w.Click(ctx, el); err != nil {
			return fmt.Errorf("menu %s click: %w", item, err)
		}
	}
	return nil
}

// Shortcut sends a keyboard shortcut.
func (w *WindowsEngine) Shortcut(ctx context.Context, keys []string) error {
	sid := w.session()
	if sid == "" {
		return errors.New("windows: no session")
	}
	return w.doJSON(ctx, http.MethodPost, "/session/"+sid+"/actions", map[string]any{
		"actions": []any{
			map[string]any{
				"type": "key",
				"id":   "keyboard",
				"actions": func() []any {
					out := make([]any, 0, len(keys)*2)
					for _, k := range keys {
						out = append(out, map[string]any{"type": "keyDown", "value": k})
					}
					for _, k := range keys {
						out = append(out, map[string]any{"type": "keyUp", "value": k})
					}
					return out
				}(),
			},
		},
	}, nil)
}

func (w *WindowsEngine) find(ctx context.Context, using, value string) (Element, error) {
	sid := w.session()
	if sid == "" {
		return Element{}, errors.New("windows: no session")
	}
	var resp struct {
		Value map[string]string `json:"value"`
	}
	if err := w.doJSON(ctx, http.MethodPost, "/session/"+sid+"/element",
		map[string]string{"using": using, "value": value}, &resp); err != nil {
		return Element{}, fmt.Errorf("find %s=%s: %w", using, value, err)
	}
	for _, v := range resp.Value {
		if v != "" {
			return Element{Handle: v, Name: value, Role: using}, nil
		}
	}
	return Element{}, fmt.Errorf("windows: element %s=%s not found", using, value)
}

func (w *WindowsEngine) elPath(handle, action string) string {
	return "/session/" + w.session() + "/element/" + handle + "/" + action
}

func (w *WindowsEngine) session() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.sessionID
}

func (w *WindowsEngine) doJSON(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, w.baseURL+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := w.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, string(raw))
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decode: %w", err)
		}
	}
	return nil
}

var _ Engine = (*WindowsEngine)(nil)
