package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestOIDCVerifierFromCoreOS_RejectsEmptyArguments(t *testing.T) {
	if _, err := OIDCVerifierFromCoreOS(context.Background(), "", "c", ""); err == nil {
		t.Fatal("empty issuer must error")
	}
	if _, err := OIDCVerifierFromCoreOS(context.Background(), "i", "", ""); err == nil {
		t.Fatal("empty clientID must error")
	}
}

func TestOIDCVerifierFromCoreOS_DiscoveryFailurePropagates(t *testing.T) {
	// An issuer that cannot be reached (invalid host) must surface a
	// descriptive error so operators notice a misconfigured IdP.
	_, err := OIDCVerifierFromCoreOS(context.Background(),
		"https://invalid-idp.not-a-real-tld.example-broken", "client", "")
	if err == nil {
		t.Fatal("unreachable issuer must error")
	}
	if !strings.Contains(err.Error(), "oidc discovery") {
		t.Errorf("error should identify discovery failure, got %q", err.Error())
	}
}

func TestStringClaim_MissingKeyReturnsEmpty(t *testing.T) {
	m := map[string]any{"a": "b"}
	if got := stringClaim(m, "missing"); got != "" {
		t.Errorf("missing key should return empty, got %q", got)
	}
	if got := stringClaim(m, "a"); got != "b" {
		t.Errorf("present key should return value, got %q", got)
	}
}

func TestStringSliceClaim_HandlesEveryShape(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		want []string
	}{
		{"nil map", map[string]any{}, nil},
		{"[]string", map[string]any{"g": []string{"a", "b"}}, []string{"a", "b"}},
		{"[]any of string", map[string]any{"g": []any{"a", "b"}}, []string{"a", "b"}},
		{"[]any with non-string dropped", map[string]any{"g": []any{"a", 42, "b"}}, []string{"a", "b"}},
		{"string singular", map[string]any{"g": "only"}, []string{"only"}},
		{"unsupported type ignored", map[string]any{"g": 12}, nil},
	}
	for _, c := range cases {
		got := stringSliceClaim(c.in, "g")
		if len(got) != len(c.want) {
			t.Errorf("%s: len = %d, want %d", c.name, len(got), len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: [%d] = %q, want %q", c.name, i, got[i], c.want[i])
			}
		}
	}
}

// Integration-shaped test that exercises the verifier wrapping. We
// build an OIDCProvider using a synthetic verifier so the full shim +
// concrete bridge path runs without network.
func TestOIDCVerifier_WiredThroughOIDCProvider(t *testing.T) {
	verifier := func(_ context.Context, raw string) (OIDCClaims, error) {
		if raw == "expired" {
			return OIDCClaims{Subject: "u"}, errors.New("token expired")
		}
		return OIDCClaims{Subject: "u", Email: "e@x", Groups: []string{"helixqa-operator"}}, nil
	}
	p, err := NewOIDCProvider("https://i.example", "helixqa", verifier)
	if err != nil {
		t.Fatal(err)
	}
	u, err := p.Verify(context.Background(), "good")
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != RoleOperator {
		t.Errorf("role = %s, want operator", u.Role)
	}

	_, err = p.Verify(context.Background(), "expired")
	if !errors.Is(err, ErrIdentityInvalid) {
		t.Errorf("expected ErrIdentityInvalid, got %v", err)
	}
}
