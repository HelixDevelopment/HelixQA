// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// content_evidence.go closes the §11.4.69 / §11.4.5 hole that the
// reference GlobEvidenceResolver leaves open: it only checks a token's
// artefact EXISTS and is NON-EMPTY (fi.Size() > 0). A non-empty-but-
// WRONG file (e.g. an `arvus_codec_in_use.txt` whose body is the empty
// placeholder `N.E.` or the downmixed `stereo`, or a `video_verdict.json`
// whose `video_live` is false / whose routed display is the wrong one)
// satisfies the size-only resolver and a Challenge bluff-PASSes.
//
// ContentAssertingResolver upgrades the ledger so a Challenge PASS
// REQUIRES the artefact's CONTENT to match the bank-declared assertion —
// the analyzed content, not file-exists. It stays project-agnostic per
// CONST-051 (§11.4.28): the resolver interprets a generic, declarative
// assertion grammar, and the consuming project supplies ALL concrete
// values (which codec regex, which JSON key, which threshold) entirely
// in its bank data. HelixQA hardcodes NO ATMOSphere codec name, no
// device serial, no Arvus value, no display id.
//
// ---------------------------------------------------------------------
// Token grammar (generic — the bank author writes these strings):
//
//	<path-or-glob> | <assertion> [ | <assertion> ... ]
//
// Everything before the FIRST " | " is the artefact path/glob (rooted
// at BaseDir for relative paths, exactly like GlobEvidenceResolver — so
// existing bare-path tokens keep working unchanged). Everything after is
// a pipe-separated list of assertions, ALL of which must hold for the
// token to be satisfied. A token with no " | " is a pure file-exists
// token (backward-compatible with GlobEvidenceResolver).
//
// Supported assertions (closed set, project-agnostic primitives):
//
//	nonempty                 — file exists and Size() > 0 (the old floor)
//	match:<regex>            — file text matches <regex> (case-insensitive)
//	not_match:<regex>        — file text does NOT match <regex> (case-insensitive)
//	json:<jsonpath><op><val> — parse JSON, navigate dotted <jsonpath>,
//	                           compare against <val> with <op>:
//	                             ==  equals (bool/number/string)
//	                             !=  not equals
//	                             >=  numeric ≥
//	                             <=  numeric ≤
//	                             >   numeric >
//	                             <   numeric <
//	                           e.g.  json:video_live==true
//	                                 json:checks.delta_e2000==true
//	                                 json:min_consecutive_ssim<0.99
//	                                 json:route==secondary
//	min_int:<field>:<n>      — first integer after a `<field>` token in the
//	                           file text is >= <n> (e.g. min_int:channels:6
//	                           over a `channels: 6` hw_params snapshot).
//
// This grammar is enough to express EVERY required_evidence assertion the
// audit asked for (Arvus value != N.E./stereo, hw_params channels>=6,
// video_live==true, route==expected-display, ΔE2000 pass, freeze absent,
// captured-WAV RMS-not-silent, channels==N) WITHOUT HelixQA learning any
// ATMOSphere fact — the bank declares the regex/key/value.
//
// Wrong-but-present content now FAILs the ledger; the size-only blind
// spot (a `echo stereo > codec.txt` PASS-bluff) is closed.
// ---------------------------------------------------------------------

// ContentAssertingResolver resolves a RequiredEvidence token by (1)
// globbing its path part (rooted at BaseDir for relative tokens) and
// (2) asserting EVERY declared assertion holds against the matched
// artefact's content. A token is satisfied only when at least one glob
// match exists AND every assertion passes for it.
//
// It is a strict superset of GlobEvidenceResolver: a token with no
// assertion clause behaves identically (exists + non-empty).
type ContentAssertingResolver struct {
	// BaseDir, when non-empty, is prepended to relative path parts so a
	// bank can use short tokens resolved against a per-run evidence dir.
	BaseDir string
}

// Resolve implements EvidenceResolver with content-correctness checking.
func (c ContentAssertingResolver) Resolve(token string) ([]string, error) {
	pathPart, assertions := splitEvidenceToken(token)

	pattern := pathPart
	if c.BaseDir != "" && !filepath.IsAbs(pathPart) {
		pattern = filepath.Join(c.BaseDir, pathPart)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("evidence glob %q invalid: %w", pattern, err)
	}

	var satisfied []string
	for _, m := range matches {
		fi, statErr := os.Stat(m)
		if statErr != nil || fi.IsDir() || fi.Size() == 0 {
			// Existence + non-empty is the implicit floor for EVERY
			// token (matches GlobEvidenceResolver). A 0-byte / missing
			// artefact never satisfies, assertions or not.
			continue
		}
		body, readErr := os.ReadFile(m)
		if readErr != nil {
			continue
		}
		ok, aerr := evalAssertions(string(body), assertions)
		if aerr != nil {
			// A malformed assertion is the bank author's bug, not a
			// device defect — surface it so it cannot silently pass.
			return nil, fmt.Errorf("evidence token %q assertion error: %w", token, aerr)
		}
		if ok {
			satisfied = append(satisfied, m)
		}
	}
	return satisfied, nil
}

// splitEvidenceToken separates the leading path/glob from the trailing
// pipe-separated assertion list. The FIRST " | " (space-pipe-space) is
// the boundary so a path containing a bare '|' is still possible (rare)
// and, more importantly, the existing free-text descriptive tokens in
// the bank — e.g. "codec.txt (Arvus shows 5.1 ...)" — that have NO
// " | " are treated as pure path tokens (their descriptive parenthetical
// is part of the "path" and simply globs to nothing extra; callers that
// want enforcement add explicit ` | ` assertions). To keep the common
// descriptive-parenthetical bank style working, we also strip a trailing
// " (...)" descriptive suffix from the path part when no assertions were
// declared, so "codec.txt (desc)" globs as "codec.txt".
func splitEvidenceToken(token string) (path string, assertions []string) {
	parts := strings.Split(token, " | ")
	path = strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		// No explicit assertions — strip a trailing descriptive
		// "(...)" so legacy descriptive tokens still glob correctly.
		path = stripTrailingParen(path)
		return path, nil
	}
	for _, a := range parts[1:] {
		a = strings.TrimSpace(a)
		if a != "" {
			assertions = append(assertions, a)
		}
	}
	return path, assertions
}

var trailingParenRe = regexp.MustCompile(`\s*\([^)]*\)\s*$`)

func stripTrailingParen(s string) string {
	return strings.TrimSpace(trailingParenRe.ReplaceAllString(s, ""))
}

// evalAssertions returns true only when EVERY assertion holds.
func evalAssertions(body string, assertions []string) (bool, error) {
	for _, a := range assertions {
		ok, err := evalOneAssertion(body, a)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func evalOneAssertion(body, a string) (bool, error) {
	switch {
	case a == "nonempty":
		return strings.TrimSpace(body) != "", nil

	case strings.HasPrefix(a, "not_match:"):
		re, err := regexp.Compile("(?i)" + strings.TrimPrefix(a, "not_match:"))
		if err != nil {
			return false, fmt.Errorf("not_match regex: %w", err)
		}
		return !re.MatchString(body), nil

	case strings.HasPrefix(a, "match:"):
		re, err := regexp.Compile("(?i)" + strings.TrimPrefix(a, "match:"))
		if err != nil {
			return false, fmt.Errorf("match regex: %w", err)
		}
		return re.MatchString(body), nil

	case strings.HasPrefix(a, "min_int:"):
		return evalMinInt(body, strings.TrimPrefix(a, "min_int:"))

	case strings.HasPrefix(a, "json:"):
		return evalJSON(body, strings.TrimPrefix(a, "json:"))

	default:
		return false, fmt.Errorf("unknown assertion %q", a)
	}
}

// evalMinInt: spec "<field>:<n>" — find the first integer that follows a
// `<field>` token in the body and require it >= n. Used for hw_params
// `channels: 6` style snapshots without forcing JSON.
func evalMinInt(body, spec string) (bool, error) {
	i := strings.LastIndex(spec, ":")
	if i < 0 {
		return false, fmt.Errorf("min_int spec %q must be <field>:<n>", spec)
	}
	field := spec[:i]
	nStr := spec[i+1:]
	n, err := strconv.Atoi(strings.TrimSpace(nStr))
	if err != nil {
		return false, fmt.Errorf("min_int threshold %q: %w", nStr, err)
	}
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(field) + `[^0-9-]*(-?\d+)`)
	mm := re.FindStringSubmatch(body)
	if mm == nil {
		return false, nil // field/integer absent → assertion fails (not present)
	}
	got, err := strconv.Atoi(mm[1])
	if err != nil {
		return false, nil
	}
	return got >= n, nil
}

var jsonAssertRe = regexp.MustCompile(`^(.*?)(==|!=|>=|<=|>|<)(.*)$`)

// evalJSON: spec "<dotted.path><op><value>". Parses body as JSON, walks
// the dotted path, compares with op. Bool/number/string comparisons.
func evalJSON(body, spec string) (bool, error) {
	mm := jsonAssertRe.FindStringSubmatch(spec)
	if mm == nil {
		return false, fmt.Errorf("json assertion %q must be <path><op><value>", spec)
	}
	jpath := strings.TrimSpace(mm[1])
	op := mm[2]
	want := strings.TrimSpace(mm[3])

	var root any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return false, fmt.Errorf("artefact is not valid JSON: %w", err)
	}
	got, ok := navigateJSON(root, jpath)
	if !ok {
		// Path absent → equality/inequality both treat as "no match".
		// == fails (value not present), != succeeds (not equal to want).
		return op == "!=", nil
	}
	return compareJSON(got, op, want)
}

// navigateJSON walks a dotted path over decoded JSON (maps + slices by
// numeric index). Returns the leaf value and whether the path resolved.
func navigateJSON(root any, dotted string) (any, bool) {
	cur := root
	if dotted == "" {
		return cur, true
	}
	for _, seg := range strings.Split(dotted, ".") {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[seg]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, false
			}
			cur = node[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

func compareJSON(got any, op, want string) (bool, error) {
	// Numeric comparison when both sides parse as numbers.
	gf, gIsNum := toFloat(got)
	wf, wErr := strconv.ParseFloat(want, 64)
	if gIsNum && wErr == nil {
		switch op {
		case "==":
			return gf == wf, nil
		case "!=":
			return gf != wf, nil
		case ">=":
			return gf >= wf, nil
		case "<=":
			return gf <= wf, nil
		case ">":
			return gf > wf, nil
		case "<":
			return gf < wf, nil
		}
	}
	// Bool comparison.
	if gb, ok := got.(bool); ok && (want == "true" || want == "false") {
		wb := want == "true"
		switch op {
		case "==":
			return gb == wb, nil
		case "!=":
			return gb != wb, nil
		}
		return false, fmt.Errorf("operator %q not valid for bool", op)
	}
	// String comparison (case-insensitive) for == / !=.
	gs := fmt.Sprintf("%v", got)
	switch op {
	case "==":
		return strings.EqualFold(gs, want), nil
	case "!=":
		return !strings.EqualFold(gs, want), nil
	}
	return false, fmt.Errorf("operator %q not valid for non-numeric value %q", op, gs)
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}
