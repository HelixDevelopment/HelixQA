package browser

// CaseInsensitiveAllowlist returns a copy of hosts lowered so the
// Engine's allowlist comparison matches any casing. Operators opt in
// with:
//
//   cfg := browser.Config{
//     AllowedHosts: browser.CaseInsensitiveAllowlist([]string{"Catalogizer.local"}),
//   }
//
// By default Nexus keeps the allowlist case-sensitive so a misconfigured
// "Catalogizer.Local" does not silently pass. Use this helper to relax
// that behaviour when the IdP or CMS being tested emits mixed-case
// hostnames.
func CaseInsensitiveAllowlist(hosts []string) []string {
	out := make([]string, len(hosts))
	for i, h := range hosts {
		out[i] = toLower(h)
	}
	return out
}
