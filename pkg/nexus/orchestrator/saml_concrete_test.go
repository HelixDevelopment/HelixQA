package orchestrator

import (
	"strings"
	"testing"
	"time"

	samllib "github.com/crewjam/saml"
)

func TestFirstValue_PicksFirstNonEmpty(t *testing.T) {
	values := []samllib.AttributeValue{
		{Value: ""},
		{Value: "hello"},
		{Value: "world"},
	}
	if got := firstValue(values); got != "hello" {
		t.Errorf("firstValue = %q", got)
	}
	if got := firstValue(nil); got != "" {
		t.Errorf("nil should return empty, got %q", got)
	}
}

func TestFirstExpirySAML_NilAssertion(t *testing.T) {
	if !firstExpirySAML(nil).IsZero() {
		t.Error("nil assertion should return zero time")
	}
}

func TestFirstExpirySAML_UsesConditionsNotOnOrAfter(t *testing.T) {
	now := time.Now().Add(time.Hour)
	assertion := &samllib.Assertion{Conditions: &samllib.Conditions{NotOnOrAfter: now}}
	got := firstExpirySAML(assertion)
	if !got.Equal(now) {
		t.Errorf("expiry = %v, want %v", got, now)
	}
}

// TestSAMLVerifierFromCrewjam_RejectsEmptyInput proves the verifier
// contract — empty assertions return the SAMLProvider's standard
// ErrIdentityInvalid rather than crashing the parse.
func TestSAMLVerifierFromCrewjam_RejectsEmptyInput(t *testing.T) {
	sp := &samllib.ServiceProvider{}
	verifier := SAMLVerifierFromCrewjam(sp, "Groups", "Team", nil)
	_, err := verifier(nil, "")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("empty assertion must error with empty hint, got %v", err)
	}
}

func TestSAMLVerifierFromCrewjam_NilServiceProvider(t *testing.T) {
	verifier := SAMLVerifierFromCrewjam(nil, "", "", nil)
	_, err := verifier(nil, "someassertion")
	if err == nil || !strings.Contains(err.Error(), "nil service provider") {
		t.Errorf("nil SP must error with nil hint, got %v", err)
	}
}

// TestSAMLVerifierFromCrewjam_B6_RejectsEmptyRequestIDSlice locks in
// B6 from docs/nexus/remaining-work.md: the verifier must refuse an
// empty possibleRequestIDs slice with an actionable error instead of
// letting a strict ParseXMLResponse fail deep inside the library.
func TestSAMLVerifierFromCrewjam_B6_RejectsEmptyRequestIDSlice(t *testing.T) {
	sp := &samllib.ServiceProvider{}

	// nil slice
	verifier := SAMLVerifierFromCrewjam(sp, "Groups", "Team", nil)
	_, err := verifier(nil, "some-assertion")
	if err == nil || !strings.Contains(err.Error(), "possibleRequestIDs") {
		t.Errorf("nil slice must mention possibleRequestIDs, got %v", err)
	}

	// explicit empty slice
	verifier = SAMLVerifierFromCrewjam(sp, "Groups", "Team", []string{})
	_, err = verifier(nil, "some-assertion")
	if err == nil || !strings.Contains(err.Error(), "possibleRequestIDs") {
		t.Errorf("empty slice must mention possibleRequestIDs, got %v", err)
	}

	// slice containing an empty string (the old default)
	verifier = SAMLVerifierFromCrewjam(sp, "Groups", "Team", []string{""})
	_, err = verifier(nil, "some-assertion")
	if err == nil || !strings.Contains(err.Error(), "empty strings") {
		t.Errorf("slice with empty entry must reject, got %v", err)
	}
}
