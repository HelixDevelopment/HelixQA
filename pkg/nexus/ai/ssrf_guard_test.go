// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ai

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubResolver lets tests inject canned DNS responses without
// hitting the network.
type stubResolver struct {
	ips map[string][]net.IP
	err error
}

func (s stubResolver) LookupIP(_ string, host string) ([]net.IP, error) {
	if s.err != nil {
		return nil, s.err
	}
	if ips, ok := s.ips[host]; ok {
		return ips, nil
	}
	return nil, errors.New("stub: no such host")
}

func TestValidateURL_RejectsEmpty(t *testing.T) {
	if err := ValidateURL("", SSRFGuardConfig{}); err == nil {
		t.Error("empty URL must be rejected")
	}
}

func TestValidateURL_RejectsUnknownScheme(t *testing.T) {
	err := ValidateURL("gopher://example.com/", SSRFGuardConfig{})
	if err == nil || !strings.Contains(err.Error(), "scheme") {
		t.Errorf("expected scheme rejection, got %v", err)
	}
}

func TestValidateURL_RejectsLoopbackLiteral(t *testing.T) {
	for _, target := range []string{
		"http://127.0.0.1/",
		"http://127.0.0.5:8080/",
		"http://[::1]/",
		"http://localhost.localdomain/",
	} {
		err := ValidateURL(target, SSRFGuardConfig{
			Resolver: stubResolver{ips: map[string][]net.IP{
				"localhost.localdomain": {net.ParseIP("127.0.0.1")},
			}},
		})
		if err == nil || !errors.Is(err, ErrSSRFBlocked) {
			t.Errorf("loopback %q must be rejected, got %v", target, err)
		}
	}
}

func TestValidateURL_RejectsRFC1918(t *testing.T) {
	cases := []string{
		"http://10.0.0.5/",
		"http://10.255.255.254/",
		"http://172.16.0.1/",
		"http://172.31.255.254/",
		"http://192.168.1.1/",
	}
	for _, target := range cases {
		err := ValidateURL(target, SSRFGuardConfig{})
		if err == nil || !errors.Is(err, ErrSSRFBlocked) {
			t.Errorf("private address %q should block, got %v", target, err)
		}
	}
}

func TestValidateURL_RejectsCloudMetadataEndpoint(t *testing.T) {
	// AWS + GCE metadata endpoint.
	err := ValidateURL("http://169.254.169.254/latest/meta-data/", SSRFGuardConfig{})
	if err == nil || !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("169.254.169.254 (metadata) must block, got %v", err)
	}
}

func TestValidateURL_RejectsIPv6LinkLocal(t *testing.T) {
	err := ValidateURL("http://[fe80::1]/", SSRFGuardConfig{})
	if err == nil || !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("IPv6 link-local must block, got %v", err)
	}
}

func TestValidateURL_RejectsIPv6ULA(t *testing.T) {
	err := ValidateURL("http://[fc00::1]/", SSRFGuardConfig{})
	if err == nil || !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("fc00::/7 must block, got %v", err)
	}
}

func TestValidateURL_RejectsUnspecified(t *testing.T) {
	for _, target := range []string{"http://0.0.0.0/", "http://[::]/"} {
		err := ValidateURL(target, SSRFGuardConfig{})
		if err == nil || !errors.Is(err, ErrSSRFBlocked) {
			t.Errorf("%s must block, got %v", target, err)
		}
	}
}

func TestValidateURL_PublicIPAllowed(t *testing.T) {
	err := ValidateURL("https://1.1.1.1/", SSRFGuardConfig{})
	if err != nil {
		t.Errorf("public IP must pass, got %v", err)
	}
}

func TestValidateURL_PublicHostnameAllowed(t *testing.T) {
	err := ValidateURL("https://api.example.com/", SSRFGuardConfig{
		Resolver: stubResolver{ips: map[string][]net.IP{
			"api.example.com": {net.ParseIP("203.0.113.7")},
		}},
	})
	if err != nil {
		t.Errorf("public hostname must pass, got %v", err)
	}
}

func TestValidateURL_HostnamePointingAtPrivateIsBlocked(t *testing.T) {
	// This is the real SSRF pivot — operator tricks the guard by
	// pointing a "public" hostname at an internal IP.
	err := ValidateURL("https://sneaky.example.com/", SSRFGuardConfig{
		Resolver: stubResolver{ips: map[string][]net.IP{
			"sneaky.example.com": {net.ParseIP("10.0.0.5")},
		}},
	})
	if err == nil || !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("hostname → private IP must block, got %v", err)
	}
}

func TestValidateURL_HostnameMixedResolutionBlocked(t *testing.T) {
	// Even one private IP in the resolution list is enough to
	// reject — the net/http stack could pick that IP.
	err := ValidateURL("https://mixed.example.com/", SSRFGuardConfig{
		Resolver: stubResolver{ips: map[string][]net.IP{
			"mixed.example.com": {
				net.ParseIP("203.0.113.7"),
				net.ParseIP("192.168.1.1"),
			},
		}},
	})
	if err == nil || !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("mixed public+private resolution must block, got %v", err)
	}
}

func TestValidateURL_AllowPrivateNetworksOptIn(t *testing.T) {
	cfg := SSRFGuardConfig{AllowPrivateNetworks: true}
	for _, target := range []string{
		"http://127.0.0.1/",
		"http://10.0.0.5/",
		"http://192.168.1.1/",
	} {
		if err := ValidateURL(target, cfg); err != nil {
			t.Errorf("with AllowPrivateNetworks=true, %q must pass, got %v", target, err)
		}
	}
}

func TestValidateURL_LookupFailureBlocks(t *testing.T) {
	err := ValidateURL("https://missing.example/", SSRFGuardConfig{
		Resolver: stubResolver{err: errors.New("nxdomain")},
	})
	if err == nil || !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("lookup failure must block, got %v", err)
	}
}

func TestValidateURL_CustomSchemeAllowList(t *testing.T) {
	// An operator that explicitly wants to allow a WS / file URL
	// can do so via AllowedSchemes.
	cfg := SSRFGuardConfig{AllowedSchemes: []string{"wss"}}
	if err := ValidateURL("wss://example.com/", SSRFGuardConfig{AllowedSchemes: []string{"wss"}, Resolver: stubResolver{ips: map[string][]net.IP{"example.com": {net.ParseIP("203.0.113.7")}}}}); err != nil {
		t.Errorf("wss allowed via opt-in must pass, got %v", err)
	}
	_ = cfg
}

// ---------------------------------------------------------------------------
// HTTPLLMClient integration — the guard fires before any bytes leave.
// ---------------------------------------------------------------------------

func TestHTTPLLMClient_SSRF_BlocksMetadataEndpoint(t *testing.T) {
	c := NewHTTPLLMClient("http://169.254.169.254/latest/meta-data/", "", "m")
	_, err := c.Chat(context.Background(), ChatRequest{UserPrompt: "x"})
	if err == nil || !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("metadata endpoint must be blocked, got %v", err)
	}
}

func TestHTTPLLMClient_SSRF_BlocksRFC1918(t *testing.T) {
	c := NewHTTPLLMClient("http://192.168.1.1/v1/chat", "", "m")
	_, err := c.Chat(context.Background(), ChatRequest{UserPrompt: "x"})
	if err == nil || !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("RFC1918 must be blocked, got %v", err)
	}
}

func TestHTTPLLMClient_SSRF_AllowPrivateUnblocksLocalhost(t *testing.T) {
	// Re-verifies the opt-in escape hatch works end-to-end against
	// a real httptest server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{}}`))
	}))
	defer srv.Close()

	c := NewHTTPLLMClient(srv.URL, "", "m")
	c.SSRFGuard.AllowPrivateNetworks = true
	resp, err := c.Chat(context.Background(), ChatRequest{UserPrompt: "x"})
	if err != nil {
		t.Fatalf("opt-in must let localhost pass: %v", err)
	}
	if resp.Text != "ok" {
		t.Errorf("text = %q", resp.Text)
	}
}
