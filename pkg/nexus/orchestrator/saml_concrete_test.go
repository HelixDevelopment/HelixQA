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
