package mobile

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeAppium is a tiny W3C-compatible Appium hub used by every mobile
// unit test. It records received requests and replies with canned
// responses keyed by method + path.
func fakeAppium(t *testing.T) (*AppiumClient, *httptest.Server, *[]recordedRequest) {
	t.Helper()
	recorded := make([]recordedRequest, 0, 16)
	mux := http.NewServeMux()
	mux.HandleFunc("/session", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		recorded = append(recorded, recordedRequest{Method: r.Method, Path: r.URL.Path, Body: string(body)})
		_, _ = w.Write([]byte(`{"value":{"sessionId":"SID-123"}}`))
	})
	mux.HandleFunc("/session/SID-123", func(w http.ResponseWriter, r *http.Request) {
		recorded = append(recorded, recordedRequest{Method: r.Method, Path: r.URL.Path})
		_, _ = w.Write([]byte(`{"value":{}}`))
	})
	mux.HandleFunc("/session/SID-123/element", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		recorded = append(recorded, recordedRequest{Method: r.Method, Path: r.URL.Path, Body: string(body)})
		_, _ = w.Write([]byte(`{"value":{"element-6066-11e4-a52e-4f735466cecf":"EL-1"}}`))
	})
	mux.HandleFunc("/session/SID-123/element/EL-1/click", func(w http.ResponseWriter, r *http.Request) {
		recorded = append(recorded, recordedRequest{Method: r.Method, Path: r.URL.Path})
		_, _ = w.Write([]byte(`{"value":null}`))
	})
	mux.HandleFunc("/session/SID-123/element/EL-1/value", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		recorded = append(recorded, recordedRequest{Method: r.Method, Path: r.URL.Path, Body: string(body)})
		_, _ = w.Write([]byte(`{"value":null}`))
	})
	mux.HandleFunc("/session/SID-123/source", func(w http.ResponseWriter, r *http.Request) {
		recorded = append(recorded, recordedRequest{Method: r.Method, Path: r.URL.Path})
		_, _ = w.Write([]byte(`{"value":"<hierarchy><node class=\"android.widget.Button\" text=\"OK\" clickable=\"true\"/></hierarchy>"}`))
	})
	mux.HandleFunc("/session/SID-123/screenshot", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"value":"PNGBYTES"}`))
	})
	mux.HandleFunc("/session/SID-123/execute/sync", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		recorded = append(recorded, recordedRequest{Method: r.Method, Path: r.URL.Path, Body: string(body)})
		_, _ = w.Write([]byte(`{"value":"OK"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewAppiumClient(srv.URL).WithHTTPClient(srv.Client()), srv, &recorded
}

type recordedRequest struct {
	Method string
	Path   string
	Body   string
}

func TestAppium_NewSessionCapturesID(t *testing.T) {
	c, _, _ := fakeAppium(t)
	caps := NewAndroidCaps("Pixel", "com.example.app", ".MainActivity")
	if err := c.NewSession(context.Background(), caps); err != nil {
		t.Fatal(err)
	}
	if c.SessionID() != "SID-123" {
		t.Errorf("session id = %q, want SID-123", c.SessionID())
	}
}

func TestAppium_CapabilitiesSerialized(t *testing.T) {
	c, _, rec := fakeAppium(t)
	caps := NewAndroidCaps("Pixel", "com.example.app", ".MainActivity")
	caps.NewCommandTimeoutSec = 60
	caps.NoReset = true
	if err := c.NewSession(context.Background(), caps); err != nil {
		t.Fatal(err)
	}
	if len(*rec) == 0 {
		t.Fatal("no requests recorded")
	}
	var payload struct {
		Capabilities struct {
			AlwaysMatch map[string]any `json:"alwaysMatch"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal([]byte((*rec)[0].Body), &payload); err != nil {
		t.Fatal(err)
	}
	am := payload.Capabilities.AlwaysMatch
	if am["platformName"] != "Android" {
		t.Errorf("platformName = %v", am["platformName"])
	}
	if am["appium:appPackage"] != "com.example.app" {
		t.Errorf("appium:appPackage = %v", am["appium:appPackage"])
	}
	if am["appium:newCommandTimeout"] != float64(60) {
		t.Errorf("timeout = %v", am["appium:newCommandTimeout"])
	}
	if am["appium:noReset"] != true {
		t.Errorf("noReset = %v", am["appium:noReset"])
	}
}

func TestAppium_FindElement_Click_SendKeys(t *testing.T) {
	c, _, rec := fakeAppium(t)
	caps := NewAndroidCaps("Pixel", "com.example.app", ".MainActivity")
	if err := c.NewSession(context.Background(), caps); err != nil {
		t.Fatal(err)
	}
	el, err := c.FindElement(context.Background(), "xpath", `//android.widget.Button[@text="OK"]`)
	if err != nil {
		t.Fatal(err)
	}
	if el != "EL-1" {
		t.Errorf("element id = %q, want EL-1", el)
	}
	if err := c.Click(context.Background(), el); err != nil {
		t.Fatal(err)
	}
	if err := c.SendKeys(context.Background(), el, "hello"); err != nil {
		t.Fatal(err)
	}
	// Confirm send-keys body included text.
	var saw bool
	for _, r := range *rec {
		if strings.HasSuffix(r.Path, "/value") && strings.Contains(r.Body, "hello") {
			saw = true
			break
		}
	}
	if !saw {
		t.Errorf("send-keys request missing text: %+v", *rec)
	}
}

func TestAppium_DeleteSessionClearsID(t *testing.T) {
	c, _, _ := fakeAppium(t)
	caps := NewAndroidCaps("Pixel", "com.example.app", ".MainActivity")
	_ = c.NewSession(context.Background(), caps)
	if err := c.DeleteSession(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.SessionID() != "" {
		t.Errorf("session id should be empty, got %q", c.SessionID())
	}
	// Second delete is a no-op.
	if err := c.DeleteSession(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCapabilities_Validate(t *testing.T) {
	cases := []struct {
		name string
		caps Capabilities
		ok   bool
	}{
		{"android ok", NewAndroidCaps("Pixel", "com.x", ".M"), true},
		{"android missing package", Capabilities{Platform: PlatformAndroid, DeviceName: "Pixel", AppActivity: ".M"}, false},
		{"android missing activity", Capabilities{Platform: PlatformAndroid, DeviceName: "Pixel", AppPackage: "com.x"}, false},
		{"ios simulator ok", NewIOSCaps("iPhone", "com.x.app", "", "", ""), true},
		{"ios missing bundle", Capabilities{Platform: PlatformIOS, DeviceName: "iPhone"}, false},
		{"ios partial signing", Capabilities{Platform: PlatformIOS, DeviceName: "iPhone", BundleID: "com.x.app", XcodeOrgID: "T"}, false},
		{"empty platform", Capabilities{DeviceName: "x"}, false},
		{"empty device", Capabilities{Platform: PlatformAndroid}, false},
	}
	for _, c := range cases {
		err := c.caps.Validate()
		if c.ok && err != nil {
			t.Errorf("%s: unexpected error %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: expected error", c.name)
		}
	}
}

func TestAppium_PageSourceAndScreenshot(t *testing.T) {
	c, _, _ := fakeAppium(t)
	caps := NewAndroidCaps("Pixel", "com.example.app", ".MainActivity")
	_ = c.NewSession(context.Background(), caps)
	xml, err := c.PageSource(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(xml, "android.widget.Button") {
		t.Errorf("unexpected page source: %q", xml)
	}
	png, err := c.Screenshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(png) != "PNGBYTES" {
		t.Errorf("unexpected screenshot: %q", string(png))
	}
}

func TestAppium_ExecuteScript(t *testing.T) {
	c, _, rec := fakeAppium(t)
	caps := NewAndroidCaps("Pixel", "com.example.app", ".MainActivity")
	_ = c.NewSession(context.Background(), caps)
	val, err := c.ExecuteScript(context.Background(), "mobile: clickGesture", map[string]any{"x": 10, "y": 20})
	if err != nil {
		t.Fatal(err)
	}
	if val != "OK" {
		t.Errorf("execute script value = %v", val)
	}
	var saw bool
	for _, r := range *rec {
		if strings.HasSuffix(r.Path, "/execute/sync") && strings.Contains(r.Body, "clickGesture") {
			saw = true
		}
	}
	if !saw {
		t.Errorf("execute script body missing: %+v", *rec)
	}
}

func TestAppium_ErrorOnHTTPFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"value":{"error":"broken"}}`))
	}))
	defer srv.Close()
	c := NewAppiumClient(srv.URL).WithHTTPClient(srv.Client())
	err := c.NewSession(context.Background(), NewAndroidCaps("Pixel", "com.x", ".M"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected status code in error, got %q", err.Error())
	}
}

func TestPlatformNameMapping(t *testing.T) {
	if platformName(PlatformIOS) != "iOS" {
		t.Error("iOS mapping")
	}
	if platformName(PlatformAndroid) != "Android" {
		t.Error("android mapping")
	}
	if platformName(PlatformAndroidTV) != "Android" {
		t.Error("androidtv mapping")
	}
}
