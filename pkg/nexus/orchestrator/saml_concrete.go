package orchestrator

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"time"

	samllib "github.com/crewjam/saml"
)

// SAMLVerifierFromCrewjam wraps a *samllib.ServiceProvider into a
// Verifier function compatible with the SAMLProvider shim. The service
// provider stays with the operator (it owns the SP key pair + metadata
// rotation); this helper only provides the "turn a raw assertion
// string into SAMLAttributes" step.
//
// Usage:
//
//	sp, _ := samlsp.New(samlsp.Options{ ... })
//	verifier := orchestrator.SAMLVerifierFromCrewjam(&sp.ServiceProvider, "groups", "team", possibleRequestIDs)
//	provider, _ := orchestrator.NewSAMLProvider(sp.ServiceProvider.EntityID, verifier)
//
// possibleRequestIDs is the list of AuthnRequest ids the caller stored
// in the user's cookie / session so the SP can match InResponseTo.
// The slice MUST be non-empty; a strict-mode SAML library rejects
// InResponseTo validation against an empty-string id, which used to
// produce confusing "InResponseTo id must be non-empty" errors deep
// inside ParseXMLResponse. B6 fix (docs/nexus/remaining-work.md):
// require a non-empty slice explicitly — callers that want to skip
// InResponseTo validation pass a single `"-"` sentinel the library
// treats as "anything goes".
func SAMLVerifierFromCrewjam(sp *samllib.ServiceProvider, groupsAttr, teamAttr string, possibleRequestIDs []string) func(context.Context, string) (SAMLAttributes, error) {
	if groupsAttr == "" {
		groupsAttr = "Groups"
	}
	if teamAttr == "" {
		teamAttr = "Team"
	}
	return func(_ context.Context, raw string) (SAMLAttributes, error) {
		if sp == nil {
			return SAMLAttributes{}, errors.New("saml: nil service provider")
		}
		if len(possibleRequestIDs) == 0 {
			return SAMLAttributes{}, errors.New("saml: possibleRequestIDs must be non-empty — pass the stored AuthnRequest ids or a single \"-\" sentinel if InResponseTo validation is skipped")
		}
		for _, id := range possibleRequestIDs {
			if id == "" {
				return SAMLAttributes{}, errors.New("saml: possibleRequestIDs must not contain empty strings")
			}
		}
		if raw == "" {
			return SAMLAttributes{}, errors.New("saml: empty assertion")
		}
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			decoded = []byte(raw)
		}
		assertion, err := sp.ParseXMLResponse(decoded, possibleRequestIDs, url.URL{})
		if err != nil {
			return SAMLAttributes{}, fmt.Errorf("saml parse: %w", err)
		}
		attrs := SAMLAttributes{
			NameID:   assertion.Subject.NameID.Value,
			NotAfter: firstExpirySAML(assertion),
		}
		for _, st := range assertion.AttributeStatements {
			for _, a := range st.Attributes {
				switch a.Name {
				case "Email", "email", "mail":
					attrs.Email = firstValue(a.Values)
				case teamAttr:
					attrs.Team = firstValue(a.Values)
				case groupsAttr:
					for _, v := range a.Values {
						if v.Value != "" {
							attrs.Groups = append(attrs.Groups, v.Value)
						}
					}
				}
			}
		}
		return attrs, nil
	}
}

func firstValue(values []samllib.AttributeValue) string {
	for _, v := range values {
		if v.Value != "" {
			return v.Value
		}
	}
	return ""
}

func firstExpirySAML(a *samllib.Assertion) time.Time {
	if a == nil {
		return time.Time{}
	}
	if a.Conditions != nil && !a.Conditions.NotOnOrAfter.IsZero() {
		return a.Conditions.NotOnOrAfter
	}
	return time.Time{}
}
