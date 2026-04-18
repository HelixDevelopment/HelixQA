package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// OIDCVerifierFromCoreOS returns a verifier function that the
// existing OIDCProvider shim can use. It wraps
// `github.com/coreos/go-oidc/v3/oidc` so production deployments get a
// real discovery + JWK Set + signature check without re-implementing
// OIDC in this codebase.
//
// Usage:
//
//	verifier, err := orchestrator.OIDCVerifierFromCoreOS(ctx, issuer, clientID, groupsClaim)
//	provider, _ := orchestrator.NewOIDCProvider(issuer, clientID, verifier)
//
// The verifier accepts any ID token issued by the OIDC provider's
// discovery endpoint whose audience matches clientID. groupsClaim
// names the custom claim that carries group membership (defaults to
// "groups"). When the claim is missing the user gets RoleViewer.
func OIDCVerifierFromCoreOS(ctx context.Context, issuer, clientID, groupsClaim string) (func(context.Context, string) (OIDCClaims, error), error) {
	if issuer == "" || clientID == "" {
		return nil, errors.New("oidc verifier: issuer and clientID required")
	}
	if groupsClaim == "" {
		groupsClaim = "groups"
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})

	return func(ctx context.Context, rawToken string) (OIDCClaims, error) {
		if rawToken == "" {
			return OIDCClaims{}, errors.New("oidc: empty token")
		}
		tok, err := verifier.Verify(ctx, rawToken)
		if err != nil {
			return OIDCClaims{}, fmt.Errorf("oidc verify: %w", err)
		}

		var raw map[string]any
		if err := tok.Claims(&raw); err != nil {
			return OIDCClaims{}, fmt.Errorf("oidc claims: %w", err)
		}

		claims := OIDCClaims{
			Subject:   tok.Subject,
			Email:     stringClaim(raw, "email"),
			Name:      stringClaim(raw, "name"),
			Team:      stringClaim(raw, "team"),
			IssuedAt:  time.Unix(tok.IssuedAt.Unix(), 0),
			ExpiresAt: time.Unix(tok.Expiry.Unix(), 0),
			Groups:    stringSliceClaim(raw, groupsClaim),
		}
		return claims, nil
	}, nil
}

func stringClaim(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func stringSliceClaim(m map[string]any, key string) []string {
	raw, ok := m[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, e := range v {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		return []string{v}
	}
	return nil
}
