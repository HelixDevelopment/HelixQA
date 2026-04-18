// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package browser

import "testing"

// FuzzDecodeHTMLEntities — P8 fix (docs/nexus/remaining-work.md):
// any HTML entity decoder is a classic catastrophic-backtracking /
// denial-of-service target on adversarial input. The fuzzer proves
// the decoder terminates + never panics on arbitrary byte sequences
// including malformed &amp; / &#x; references and embedded nulls.
func FuzzDecodeHTMLEntities(f *testing.F) {
	seed := []string{
		"",
		"plain text",
		"&amp;",
		"&quot;",
		"&lt;script&gt;",
		"&#60;",
		"&#x3c;",
		"&notarealentity;",
		"&&&&",
		"&",
		"\x00&amp;\x00",
		"&amp;&amp;&amp;&amp;&amp;",
	}
	for _, s := range seed {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		_ = decodeHTMLEntities(in)
	})
}

// FuzzParseAttrs exercises the permissive HTML attribute parser
// against arbitrary quote / whitespace / escape combinations. P8
// guard: the parser must terminate and never loop on quote-imbalance
// patterns that caused historical regressions in similar tools.
func FuzzParseAttrs(f *testing.F) {
	seed := []string{
		``,
		` id=test role=button `,
		` id="x" role='y' `,
		` data-x=  "value with spaces"`,
		` aria-label="quote " inside"`,
		` bad= noclose `,
		` <<< >>> `,
		` id=&amp;amp; `,
	}
	for _, s := range seed {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		_ = parseAttrs(in, in)
	})
}
