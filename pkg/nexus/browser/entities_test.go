package browser

import "testing"

func TestDecodeHTMLEntities_NamedEntities(t *testing.T) {
	cases := map[string]string{
		"Fish &amp; Chips":  "Fish & Chips",
		"&lt;tag&gt;":       "<tag>",
		"&quot;hello&quot;": `"hello"`,
		"it&apos;s":         "it's",
		"it&#39;s":          "it's",
		"a&nbsp;b":          "a b",
		"no entities":       "no entities",
	}
	for in, want := range cases {
		if got := decodeHTMLEntities(in); got != want {
			t.Errorf("decode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDecodeHTMLEntities_NumericDecimal(t *testing.T) {
	if got := decodeHTMLEntities("A&#65;B"); got != "AAB" {
		t.Errorf("decode numeric = %q", got)
	}
}

func TestDecodeHTMLEntities_NumericHex(t *testing.T) {
	if got := decodeHTMLEntities("&#x48;&#x69;"); got != "Hi" {
		t.Errorf("decode hex = %q", got)
	}
}

func TestDecodeHTMLEntities_MalformedLeavesLiteral(t *testing.T) {
	// An incomplete entity at end-of-string should pass through
	// unchanged so callers do not see corruption.
	if got := decodeHTMLEntities("incomplete &amp"); got != "incomplete &amp" {
		t.Errorf("incomplete entity mangled: %q", got)
	}
}

func TestDecodeHTMLEntities_NoAmpersandFastPath(t *testing.T) {
	in := "simple string"
	if got := decodeHTMLEntities(in); got != in {
		t.Errorf("fast path mangled input: %q", got)
	}
}

func TestParseNumericEntity_BadBase(t *testing.T) {
	if _, ok := parseNumericEntity(""); ok {
		t.Fatal("empty token should fail")
	}
	if _, ok := parseNumericEntity("xZZ"); ok {
		t.Fatal("non-hex token should fail")
	}
	// Value above the highest Unicode codepoint.
	if _, ok := parseNumericEntity("x110000"); ok {
		t.Fatal("out-of-range codepoint should fail")
	}
}
