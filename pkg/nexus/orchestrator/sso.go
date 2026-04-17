package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// IdentityProvider is the narrow contract every SSO backend satisfies.
// Concrete implementations bridge OIDC (via golang.org/x/oauth2 +
// go-oidc) or SAML (via gosaml2 / crewjam/saml) to the Nexus User
// struct. Keeping the contract small here lets the orchestrator stay
// SDK-agnostic; operators ship an adapter per IdP.
type IdentityProvider interface {
	// Name returns a human-readable identifier for logs.
	Name() string
	// Verify turns an opaque credential (ID token, SAML assertion
	// string, etc.) into a populated User. Implementations are free
	// to reject expired or malformed credentials.
	Verify(ctx context.Context, credential string) (User, error)
}

// ErrIdentityInvalid is returned by Verify when the credential cannot
// be trusted.
var ErrIdentityInvalid = errors.New("sso: invalid credential")

// --- OIDC shim -------------------------------------------------------

// OIDCProvider is a minimal OIDC verifier. It expects a JWT-shaped ID
// token with a JWK Set reachable at JWKSURL. The shim does not crack
// crypto itself — operators wire it up to their preferred library
// (coreos/go-oidc, go-jose, etc.) by passing in a Verifier function.
type OIDCProvider struct {
	Issuer   string
	ClientID string
	Verifier func(ctx context.Context, rawIDToken string) (OIDCClaims, error)
}

// OIDCClaims carries the fields we need from an OIDC ID token.
type OIDCClaims struct {
	Subject  string
	Email    string
	Name     string
	Groups   []string
	Team     string
	IssuedAt time.Time
	ExpiresAt time.Time
}

// NewOIDCProvider returns a provider ready to Verify() tokens. The
// verifier parameter is required; Nexus does not ship a default.
func NewOIDCProvider(issuer, clientID string, verifier func(ctx context.Context, rawIDToken string) (OIDCClaims, error)) (*OIDCProvider, error) {
	if issuer == "" || clientID == "" {
		return nil, errors.New("oidc: issuer and clientID required")
	}
	if verifier == nil {
		return nil, errors.New("oidc: verifier required")
	}
	return &OIDCProvider{Issuer: issuer, ClientID: clientID, Verifier: verifier}, nil
}

// Name reports the IdP identifier.
func (p *OIDCProvider) Name() string { return "oidc:" + p.Issuer }

// Verify decodes and verifies an OIDC ID token and maps the claims
// onto a Nexus User with a role derived from group membership.
func (p *OIDCProvider) Verify(ctx context.Context, rawIDToken string) (User, error) {
	if rawIDToken == "" {
		return User{}, fmt.Errorf("%w: empty token", ErrIdentityInvalid)
	}
	claims, err := p.Verifier(ctx, rawIDToken)
	if err != nil {
		return User{}, fmt.Errorf("%w: %v", ErrIdentityInvalid, err)
	}
	if !claims.ExpiresAt.IsZero() && time.Now().After(claims.ExpiresAt) {
		return User{}, fmt.Errorf("%w: token expired", ErrIdentityInvalid)
	}
	return User{
		ID:    claims.Subject,
		Email: claims.Email,
		Team:  claims.Team,
		Role:  mapGroupsToRole(claims.Groups),
	}, nil
}

// --- SAML shim -------------------------------------------------------

// SAMLProvider is a minimal SAML assertion verifier. Like OIDCProvider
// it accepts an operator-supplied verifier function so the Nexus
// package does not pull in heavy dependencies.
type SAMLProvider struct {
	EntityID string
	Verifier func(ctx context.Context, rawAssertion string) (SAMLAttributes, error)
}

// SAMLAttributes is the subset of SAML assertion attributes Nexus uses.
type SAMLAttributes struct {
	NameID    string
	Email     string
	Groups    []string
	Team      string
	NotAfter  time.Time
}

// NewSAMLProvider returns a verifier bound to entityID. The verifier
// parameter is required.
func NewSAMLProvider(entityID string, verifier func(ctx context.Context, rawAssertion string) (SAMLAttributes, error)) (*SAMLProvider, error) {
	if entityID == "" {
		return nil, errors.New("saml: entityID required")
	}
	if verifier == nil {
		return nil, errors.New("saml: verifier required")
	}
	return &SAMLProvider{EntityID: entityID, Verifier: verifier}, nil
}

// Name reports the IdP identifier.
func (p *SAMLProvider) Name() string { return "saml:" + p.EntityID }

// Verify decodes and verifies a SAML assertion, mapping attributes to
// a Nexus User.
func (p *SAMLProvider) Verify(ctx context.Context, rawAssertion string) (User, error) {
	if rawAssertion == "" {
		return User{}, fmt.Errorf("%w: empty assertion", ErrIdentityInvalid)
	}
	attrs, err := p.Verifier(ctx, rawAssertion)
	if err != nil {
		return User{}, fmt.Errorf("%w: %v", ErrIdentityInvalid, err)
	}
	if !attrs.NotAfter.IsZero() && time.Now().After(attrs.NotAfter) {
		return User{}, fmt.Errorf("%w: assertion expired", ErrIdentityInvalid)
	}
	return User{
		ID:    attrs.NameID,
		Email: attrs.Email,
		Team:  attrs.Team,
		Role:  mapGroupsToRole(attrs.Groups),
	}, nil
}

// --- shared helpers --------------------------------------------------

// mapGroupsToRole maps IdP group names to Nexus Role values. Unknown
// groups produce RoleViewer so a misconfigured mapping cannot escalate
// by mistake.
func mapGroupsToRole(groups []string) Role {
	highest := RoleViewer
	for _, g := range groups {
		r := groupToRole(g)
		if roleRank[r] > roleRank[highest] {
			highest = r
		}
	}
	return highest
}

func groupToRole(g string) Role {
	switch strings.ToLower(g) {
	case "helixqa-admin", "helixqa:admin", "admin":
		return RoleAdmin
	case "helixqa-operator", "helixqa:operator", "operator":
		return RoleOperator
	case "helixqa-runner", "helixqa:runner", "runner":
		return RoleRunner
	case "helixqa-viewer", "helixqa:viewer", "viewer":
		return RoleViewer
	}
	return RoleViewer
}

// --- HTTP middleware -------------------------------------------------

// AuthMiddleware decorates an http.Handler so every request carries a
// User resolved by the first IdentityProvider that accepts the token.
// Clients send either Authorization: Bearer <token> (OIDC) or
// Authorization: SAML <assertion> (SAML). Missing / invalid tokens
// return 401.
type AuthMiddleware struct {
	providers []IdentityProvider
}

// NewAuthMiddleware wires a chain of providers. The first non-error
// Verify wins.
func NewAuthMiddleware(providers ...IdentityProvider) *AuthMiddleware {
	return &AuthMiddleware{providers: providers}
}

// Wrap returns a handler that injects the resolved User into the
// request context under the key userCtxKey.
func (a *AuthMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("Authorization")
		if raw == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		scheme, token := splitScheme(raw)
		for _, p := range a.providers {
			if !supports(p, scheme) {
				continue
			}
			u, err := p.Verify(r.Context(), token)
			if err == nil {
				ctx := context.WithValue(r.Context(), userCtxKey{}, u)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

// UserFromContext returns the authenticated User, if any.
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userCtxKey{}).(User)
	return u, ok
}

type userCtxKey struct{}

func splitScheme(raw string) (string, string) {
	parts := strings.SplitN(raw, " ", 2)
	if len(parts) != 2 {
		return "", raw
	}
	return strings.ToLower(parts[0]), parts[1]
}

func supports(p IdentityProvider, scheme string) bool {
	switch p.(type) {
	case *OIDCProvider:
		return scheme == "bearer" || scheme == "oidc"
	case *SAMLProvider:
		return scheme == "saml"
	}
	// Unknown providers: accept any.
	return true
}
