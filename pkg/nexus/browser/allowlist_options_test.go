package browser

import "testing"

func TestCaseInsensitiveAllowlist_LowersHosts(t *testing.T) {
	in := []string{"Catalogizer.local", "Example.COM"}
	out := CaseInsensitiveAllowlist(in)
	if out[0] != "catalogizer.local" || out[1] != "example.com" {
		t.Errorf("case insensitive allowlist = %+v", out)
	}
}

func TestCaseInsensitiveAllowlist_EmptyStaysEmpty(t *testing.T) {
	out := CaseInsensitiveAllowlist(nil)
	if len(out) != 0 {
		t.Errorf("nil input should produce empty output, got %+v", out)
	}
}
