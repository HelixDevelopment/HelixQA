package browser

import (
	"strings"
)

// decodeHTMLEntities replaces the handful of HTML entities that
// realistically appear inside attribute values with their literal
// characters. The snapshot parser is intentionally tolerant — it does
// not pull in the full html package — so we decode the most common
// named entities plus numeric (&#NN;, &#xHH;) forms.
//
// The Nexus snapshot only needs correct decoding for names/labels the
// AI navigator will consume; we err on the side of over-tolerance
// rather than reject otherwise-good snapshots.
func decodeHTMLEntities(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	// Cheap named entities first.
	replacements := []string{
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&apos;", "'",
		"&#39;", "'",
		"&nbsp;", " ",
	}
	r := strings.NewReplacer(replacements...)
	s = r.Replace(s)

	// Numeric entities: &#NN; and &#xHH;
	var out strings.Builder
	out.Grow(len(s))
	i := 0
	for i < len(s) {
		if i+2 < len(s) && s[i] == '&' && s[i+1] == '#' {
			end := strings.IndexByte(s[i:], ';')
			if end > 0 {
				token := s[i+2 : i+end]
				if r, ok := parseNumericEntity(token); ok {
					out.WriteRune(r)
					i += end + 1
					continue
				}
			}
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

func parseNumericEntity(token string) (rune, bool) {
	if len(token) == 0 {
		return 0, false
	}
	base := 10
	if token[0] == 'x' || token[0] == 'X' {
		token = token[1:]
		base = 16
	}
	var value int32
	for _, c := range token {
		var digit int32
		switch {
		case c >= '0' && c <= '9':
			digit = int32(c - '0')
		case base == 16 && c >= 'a' && c <= 'f':
			digit = int32(c - 'a' + 10)
		case base == 16 && c >= 'A' && c <= 'F':
			digit = int32(c - 'A' + 10)
		default:
			return 0, false
		}
		value = value*int32(base) + digit
		if value > 0x10FFFF { // beyond the highest Unicode code point
			return 0, false
		}
	}
	return rune(value), true
}
