package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- Helpers ---

func oidcVerifier(claims OIDCClaims, err error) func(context.Context, string) (OIDCClaims, error) {
	return func(context.Context, string) (OIDCClaims, error) {
		return claims, err
	}
}

func samlVerifier(attrs SAMLAttributes, err error) func(context.Context, string) (SAMLAttributes, error) {
	return func(context.Context, string) (SAMLAttributes, error) {
		return attrs, err
	}
}

// --- OIDC ---

func TestNewOIDCProvider_Validation(t *testing.T) {
	if _, err := NewOIDCProvider("", "c", oidcVerifier(OIDCClaims{}, nil)); err == nil {
		t.Fatal("empty issuer must error")
	}
	if _, err := NewOIDCProvider("i", "", oidcVerifier(OIDCClaims{}, nil)); err == nil {
		t.Fatal("empty clientID must error")
	}
	if _, err := NewOIDCProvider("i", "c", nil); err == nil {
		t.Fatal("nil verifier must error")
	}
}

func TestOIDCProvider_VerifyHappyPath(t *testing.T) {
	p, _ := NewOIDCProvider("https://idp.example", "helixqa", oidcVerifier(OIDCClaims{
		Subject: "u1", Email: "e@x", Team: "qa",
		Groups:    []string{"helixqa-operator"},
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil))
	u, err := p.Verify(context.Background(), "token")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != "u1" || u.Email != "e@x" || u.Role != RoleOperator {
		t.Errorf("mapping wrong: %+v", u)
	}
}

func TestOIDCProvider_VerifyRejectsEmpty(t *testing.T) {
	p, _ := NewOIDCProvider("i", "c", oidcVerifier(OIDCClaims{}, nil))
	_, err := p.Verify(context.Background(), "")
	if !errors.Is(err, ErrIdentityInvalid) {
		t.Errorf("expected ErrIdentityInvalid, got %v", err)
	}
}

func TestOIDCProvider_VerifyRejectsExpired(t *testing.T) {
	p, _ := NewOIDCProvider("i", "c", oidcVerifier(OIDCClaims{
		Subject: "u1", ExpiresAt: time.Now().Add(-time.Hour),
	}, nil))
	_, err := p.Verify(context.Background(), "token")
	if !errors.Is(err, ErrIdentityInvalid) {
		t.Errorf("expected ErrIdentityInvalid, got %v", err)
	}
}

func TestOIDCProvider_VerifyPropagatesError(t *testing.T) {
	p, _ := NewOIDCProvider("i", "c", oidcVerifier(OIDCClaims{}, errors.New("sig invalid")))
	_, err := p.Verify(context.Background(), "token")
	if !errors.Is(err, ErrIdentityInvalid) {
		t.Errorf("expected ErrIdentityInvalid, got %v", err)
	}
}

// --- SAML ---

func TestNewSAMLProvider_Validation(t *testing.T) {
	if _, err := NewSAMLProvider("", samlVerifier(SAMLAttributes{}, nil)); err == nil {
		t.Fatal("empty entityID must error")
	}
	if _, err := NewSAMLProvider("e", nil); err == nil {
		t.Fatal("nil verifier must error")
	}
}

func TestSAMLProvider_VerifyHappyPath(t *testing.T) {
	p, _ := NewSAMLProvider("helixqa-sp", samlVerifier(SAMLAttributes{
		NameID: "alice", Email: "a@x", Team: "qa",
		Groups:   []string{"helixqa-admin"},
		NotAfter: time.Now().Add(time.Hour),
	}, nil))
	u, err := p.Verify(context.Background(), "assertion")
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != RoleAdmin || u.Email != "a@x" {
		t.Errorf("mapping wrong: %+v", u)
	}
}

func TestSAMLProvider_VerifyRejectsExpired(t *testing.T) {
	p, _ := NewSAMLProvider("e", samlVerifier(SAMLAttributes{
		NameID: "x", NotAfter: time.Now().Add(-time.Hour),
	}, nil))
	_, err := p.Verify(context.Background(), "assertion")
	if !errors.Is(err, ErrIdentityInvalid) {
		t.Errorf("expected ErrIdentityInvalid, got %v", err)
	}
}

// --- Group → Role mapping ---

func TestMapGroupsToRole_PickHighest(t *testing.T) {
	cases := []struct {
		groups []string
		want   Role
	}{
		{[]string{"viewer", "runner"}, RoleRunner},
		{[]string{"helixqa-operator"}, RoleOperator},
		{[]string{"helixqa:admin"}, RoleAdmin},
		{[]string{"unknown-group"}, RoleViewer},
		{nil, RoleViewer},
	}
	for _, c := range cases {
		if got := mapGroupsToRole(c.groups); got != c.want {
			t.Errorf("mapGroupsToRole(%v) = %s, want %s", c.groups, got, c.want)
		}
	}
}

// --- Middleware ---

func TestAuthMiddleware_AcceptsValidBearer(t *testing.T) {
	p, _ := NewOIDCProvider("i", "c", oidcVerifier(OIDCClaims{
		Subject: "u", Email: "e@x", ExpiresAt: time.Now().Add(time.Hour),
	}, nil))
	mw := NewAuthMiddleware(p)

	var captured User
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := UserFromContext(r.Context())
		captured = u
		w.WriteHeader(204)
	}))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL, nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 204 || captured.ID != "u" {
		t.Errorf("status=%d user=%+v", resp.StatusCode, captured)
	}
}

func TestAuthMiddleware_Rejects401WhenNoHeader(t *testing.T) {
	mw := NewAuthMiddleware()
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestAuthMiddleware_Rejects401WhenVerifierFails(t *testing.T) {
	p, _ := NewOIDCProvider("i", "c", oidcVerifier(OIDCClaims{}, errors.New("bad sig")))
	mw := NewAuthMiddleware(p)
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL, nil)
	req.Header.Set("Authorization", "Bearer invalid")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestAuthMiddleware_ChoosesProviderByScheme(t *testing.T) {
	oidc, _ := NewOIDCProvider("i", "c", oidcVerifier(OIDCClaims{Subject: "oid", ExpiresAt: time.Now().Add(time.Hour)}, nil))
	saml, _ := NewSAMLProvider("e", samlVerifier(SAMLAttributes{NameID: "sam", NotAfter: time.Now().Add(time.Hour)}, nil))
	mw := NewAuthMiddleware(oidc, saml)

	var captured User
	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := UserFromContext(r.Context())
		captured = u
		w.WriteHeader(204)
	}))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	for _, tc := range []struct {
		auth, want string
	}{
		{"Bearer x", "oid"},
		{"SAML y", "sam"},
	} {
		req, _ := http.NewRequest("GET", srv.URL, nil)
		req.Header.Set("Authorization", tc.auth)
		_, _ = http.DefaultClient.Do(req)
		if captured.ID != tc.want {
			t.Errorf("scheme %q → user %q, want %q", tc.auth, captured.ID, tc.want)
		}
	}
}

func TestUserFromContext_NilSafe(t *testing.T) {
	if _, ok := UserFromContext(context.Background()); ok {
		t.Error("UserFromContext should return ok=false when no user set")
	}
}
